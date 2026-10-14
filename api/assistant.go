package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"
    "unicode/utf8"
)

type assistantSettings struct {
    Prompt string `json:"prompt"`
    GuidanceEnabled bool `json:"guidance_enabled"`
}
type assistantConversation struct {
    ID int64 `json:"id"`
    Title string `json:"title"`
    UpdatedAt time.Time `json:"updated_at"`
    Busy bool `json:"busy"`
}
type assistantMessage struct {
    ID int64 `json:"id"`
    Role string `json:"role"`
    Content string `json:"content"`
    Citations []hit `json:"citations"`
    PromptSnapshot string `json:"prompt_snapshot"`
    GuidanceEnabled bool `json:"guidance_enabled"`
    State string `json:"state"`
    CreatedAt time.Time `json:"created_at"`
}

func initAssistant() {
    // This Compose project runs one API instance. Release conversations interrupted by a process restart.
    if _,err:=db.Exec("UPDATE assistant_conversations SET busy=FALSE");err!=nil{log.Fatal(err)}
    if _,err:=db.Exec("UPDATE assistant_messages SET state='interrupted' WHERE state='streaming'");err!=nil{log.Fatal(err)}
}
func loadAssistantSettings(classID int64)(assistantSettings,error) {
    s:=assistantSettings{Prompt:defaultAssistantPrompt,GuidanceEnabled:true}
    err:=db.QueryRow("SELECT system_prompt,guidance_enabled FROM assistant_settings WHERE class_id=?",classID).Scan(&s.Prompt,&s.GuidanceEnabled)
    if errors.Is(err,sql.ErrNoRows){return s,nil}
    return s,err
}
func assistantGetSettings(w http.ResponseWriter,r *http.Request,u user) {
    s,err:=loadAssistantSettings(u.ClassID)
    if err!=nil{unavailable(w);return}
    respond(w,s)
}
func assistantPutSettings(w http.ResponseWriter,r *http.Request,u user) {
    if u.Role!="teacher" {failure(w,403);return}
    var in struct {Prompt string `json:"prompt"`;GuidanceEnabled *bool `json:"guidance_enabled"`}
    r.Body=http.MaxBytesReader(w,r.Body,16<<10)
    if json.NewDecoder(r.Body).Decode(&in)!=nil || in.GuidanceEnabled==nil || strings.TrimSpace(in.Prompt)=="" || !utf8.ValidString(in.Prompt) || len([]rune(in.Prompt))>2000 {failure(w,400);return}
    s:=assistantSettings{Prompt:strings.TrimSpace(in.Prompt),GuidanceEnabled:*in.GuidanceEnabled}
    _,err:=db.Exec("INSERT INTO assistant_settings(class_id,system_prompt,guidance_enabled) VALUES(?,?,?) ON DUPLICATE KEY UPDATE system_prompt=VALUES(system_prompt),guidance_enabled=VALUES(guidance_enabled)",u.ClassID,s.Prompt,s.GuidanceEnabled)
    if err!=nil{unavailable(w);return}
    respond(w,s)
}
func assistantListConversations(w http.ResponseWriter,r *http.Request,u user) {
    rows,err:=db.Query("SELECT id,title,updated_at,busy FROM assistant_conversations WHERE user_id=? AND class_id=? ORDER BY updated_at DESC,id DESC LIMIT 100",u.ID,u.ClassID)
    if err!=nil{unavailable(w);return}
    defer rows.Close()
    out:=[]assistantConversation{}
    for rows.Next(){var c assistantConversation;if rows.Scan(&c.ID,&c.Title,&c.UpdatedAt,&c.Busy)!=nil{unavailable(w);return};out=append(out,c)}
    if rows.Err()!=nil{unavailable(w);return}
    respond(w,out)
}
func assistantCreateConversation(w http.ResponseWriter,r *http.Request,u user) {
    result,err:=db.Exec("INSERT INTO assistant_conversations(user_id,class_id) VALUES(?,?)",u.ID,u.ClassID)
    if err!=nil{unavailable(w);return}
    id,err:=result.LastInsertId()
    if err!=nil{unavailable(w);return}
    w.Header().Set("Content-Type","application/json; charset=utf-8")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]int64{"id":id})
}
func ownedAssistantConversation(r *http.Request,u user)(assistantConversation,error) {
    id,err:=strconv.ParseInt(r.PathValue("id"),10,64)
    if err!=nil||id<1{return assistantConversation{},sql.ErrNoRows}
    var c assistantConversation
    err=db.QueryRow("SELECT id,title,updated_at,busy FROM assistant_conversations WHERE id=? AND user_id=? AND class_id=?",id,u.ID,u.ClassID).Scan(&c.ID,&c.Title,&c.UpdatedAt,&c.Busy)
    return c,err
}
func assistantGetConversation(w http.ResponseWriter,r *http.Request,u user) {
    c,err:=ownedAssistantConversation(r,u)
    if errors.Is(err,sql.ErrNoRows){failure(w,404);return};if err!=nil{unavailable(w);return}
    rows,err:=db.Query("SELECT id,role,content,citations,prompt_snapshot,guidance_enabled,state,created_at FROM assistant_messages WHERE conversation_id=? ORDER BY id LIMIT 200",c.ID)
    if err!=nil{unavailable(w);return};defer rows.Close()
    messages:=[]assistantMessage{}
    for rows.Next(){var m assistantMessage;var raw []byte;if rows.Scan(&m.ID,&m.Role,&m.Content,&raw,&m.PromptSnapshot,&m.GuidanceEnabled,&m.State,&m.CreatedAt)!=nil{unavailable(w);return};if json.Unmarshal(raw,&m.Citations)!=nil{unavailable(w);return};messages=append(messages,m)}
    if rows.Err()!=nil{unavailable(w);return}
    respond(w,map[string]any{"conversation":c,"messages":messages})
}
func loadAssistantHistory(id int64)([]modelMessage,error) {
    rows,err:=db.Query("SELECT role,content FROM assistant_messages WHERE conversation_id=? AND state='complete' ORDER BY id DESC LIMIT 12",id)
    if err!=nil{return nil,err};defer rows.Close()
    history:=[]modelMessage{}
    for rows.Next(){var m modelMessage;if err=rows.Scan(&m.Role,&m.Content);err!=nil{return nil,err};history=append(history,m)}
    if rows.Err()!=nil{return nil,rows.Err()}
    for left,right:=0,len(history)-1;left<right;left,right=left+1,right-1 {history[left],history[right]=history[right],history[left]}
    return history,nil
}
func persistAssistantReply(id int64,content,state string)error {
    ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
    _,err:=db.ExecContext(ctx,"UPDATE assistant_messages SET content=?,state=? WHERE id=?",content,state,id)
    return err
}
func assistantSendMessage(w http.ResponseWriter,r *http.Request,u user) {
    c,err:=ownedAssistantConversation(r,u)
    if errors.Is(err,sql.ErrNoRows){failure(w,404);return};if err!=nil{unavailable(w);return}
    var in struct {Question string `json:"question"`}
    r.Body=http.MaxBytesReader(w,r.Body,8192)
    if json.NewDecoder(r.Body).Decode(&in)!=nil||strings.TrimSpace(in.Question)==""||!utf8.ValidString(in.Question)||len([]rune(in.Question))>500{failure(w,400);return}
    question:=strings.TrimSpace(in.Question)
    lock,err:=db.Exec("UPDATE assistant_conversations SET busy=TRUE WHERE id=? AND user_id=? AND class_id=? AND busy=FALSE",c.ID,u.ID,u.ClassID)
    if err!=nil{unavailable(w);return}
    locked,err:=lock.RowsAffected();if err!=nil{unavailable(w);return};if locked!=1{failure(w,409);return}
    defer func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();if _,e:=db.ExecContext(ctx,"UPDATE assistant_conversations SET busy=FALSE,updated_at=CURRENT_TIMESTAMP WHERE id=?",c.ID);e!=nil{log.Printf("release assistant conversation: %v",e)}}()
    settings,err:=loadAssistantSettings(u.ClassID);if err!=nil{unavailable(w);return}
    history,err:=loadAssistantHistory(c.ID);if err!=nil{unavailable(w);return}
    var total int
    if err=db.QueryRow("SELECT COUNT(*) FROM assistant_messages WHERE conversation_id=?",c.ID).Scan(&total);err!=nil{unavailable(w);return}
    if total>=200{failure(w,409);return}
    hits,err:=retrieve(assistantRetrievalQuery(question,history),"hybrid",u.ClassID,4)
    if err!=nil{log.Printf("assistant retrieval: %v",err);unavailable(w);return}
    citations,err:=json.Marshal(hits);if err!=nil{unavailable(w);return}
    tx,err:=db.Begin();if err!=nil{unavailable(w);return};defer tx.Rollback()
    result,err:=tx.Exec("INSERT INTO assistant_messages(conversation_id,role,content,citations,prompt_snapshot,guidance_enabled,state) VALUES(?,'user',?,'[]',?,?,'complete')",c.ID,question,settings.Prompt,settings.GuidanceEnabled)
    if err!=nil{unavailable(w);return};userMessageID,err:=result.LastInsertId();if err!=nil{unavailable(w);return}
    result,err=tx.Exec("INSERT INTO assistant_messages(conversation_id,role,content,citations,prompt_snapshot,guidance_enabled,state) VALUES(?,'assistant','',?,?,?,'streaming')",c.ID,string(citations),settings.Prompt,settings.GuidanceEnabled)
    if err!=nil{unavailable(w);return};replyID,err:=result.LastInsertId();if err!=nil{unavailable(w);return}
    if total==0{if _,err=tx.Exec("UPDATE assistant_conversations SET title=? WHERE id=?",limitCharacters(question,60),c.ID);err!=nil{unavailable(w);return}}
    if err=tx.Commit();err!=nil{unavailable(w);return}
    w.Header().Set("Content-Type","text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control","no-store, no-transform")
    w.Header().Set("X-Accel-Buffering","no")
    w.WriteHeader(http.StatusOK)
    answer:="";state:="interrupted";persisted:=false
    defer func(){if !persisted{if e:=persistAssistantReply(replyID,answer,state);e!=nil{log.Printf("persist partial assistant reply: %v",e)}}}()
    meta:=map[string]any{"conversation_id":c.ID,"user_message_id":userMessageID,"assistant_message_id":replyID,"citations":hits,"guidance_enabled":settings.GuidanceEnabled,"prompt_snapshot":settings.Prompt}
    if sendAssistantEvent(w,"meta",meta)!=nil{return}
    emit:=func(delta string)error{
        if len([]rune(answer))+len([]rune(delta))>6000{return errors.New("assistant reply exceeds limit")}
        answer+=delta
        return sendAssistantEvent(w,"delta",map[string]string{"text":delta})
    }
    if len(hits)==0{err=emit(noEvidence+"。请补充题目关键词或请本班教师上传相关材料。")}else{
        ctx,cancel:=context.WithTimeout(r.Context(),120*time.Second)
        err=streamAssistantModel(ctx,assistantModelMessages(settings.Prompt,settings.GuidanceEnabled,history,question,hits),emit)
        cancel()
    }
    if err==nil && strings.TrimSpace(answer)==""{err=errors.New("empty assistant reply")}
    if err!=nil{
        if r.Context().Err()==nil{state="error";sendAssistantEvent(w,"error",map[string]string{"message":"回答生成中断，请稍后重试；已保存收到的部分内容。"})}
        log.Printf("assistant stream: %v",err);return
    }
    if len(hits)>0{answer=cleanAssistantCitations(answer,hits)}
    state="complete"
    if err=persistAssistantReply(replyID,answer,state);err!=nil{state="error";sendAssistantEvent(w,"error",map[string]string{"message":"回答保存失败，请稍后刷新确认。"});return}
    persisted=true
    sendAssistantEvent(w,"done",map[string]any{"message_id":replyID,"content":answer,"citations":hits,"state":state})
}
