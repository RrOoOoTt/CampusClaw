package main

import (
 "database/sql"
 "errors"
 "log"
 "net/http"
 "os"
 "path/filepath"
 "strconv"
 "time"
)

// @api DELETE /api/materials/{id} | 删除本班资料与索引 | teacher | - | DeleteResult
func deleteMaterial(w http.ResponseWriter,r *http.Request,u user){
 if u.Role!="teacher"{failure(w,403);return}
 id,err:=strconv.ParseInt(r.PathValue("id"),10,64);if err!=nil||id<1{failure(w,404);return}
 // Serialize with indexing/backfill so a deleted material cannot acquire new vector points.
 materialMutation.Lock();defer materialMutation.Unlock()
 tx,err:=db.Begin();if err!=nil{unavailable(w);return};defer tx.Rollback()
 var name string
 err=tx.QueryRow("SELECT storage_name FROM materials WHERE id=? AND class_id=? FOR UPDATE",id,u.ClassID).Scan(&name)
 if errors.Is(err,sql.ErrNoRows){failure(w,404);return};if err!=nil{unavailable(w);return}
 // The durable cleanup job commits with deletion. External failures cannot resurrect a document.
 if _,err=tx.Exec("INSERT INTO material_cleanup_jobs(material_id,class_id,storage_name) VALUES(?,?,?)",id,u.ClassID,name);err!=nil{unavailable(w);return}
 if _,err=tx.Exec("DELETE FROM materials WHERE id=? AND class_id=?",id,u.ClassID);err!=nil{unavailable(w);return}
 if err=tx.Commit();err!=nil{unavailable(w);return}
 pending:=cleanupMaterial(id,u.ClassID,name)!=nil
 respond(w,map[string]any{"deleted":true,"cleanup_pending":pending})
}

func cleanupMaterial(id,classID int64,name string)error{
 // SQL retrieval no longer exposes the material even if Qdrant is temporarily offline.
 _,err:=qRequest("POST","/collections/"+collection+"/points/delete?wait=true",map[string]any{"filter":map[string]any{"must":[]any{
  map[string]any{"key":"material_id","match":map[string]any{"value":id}},
  map[string]any{"key":"class_id","match":map[string]any{"value":classID}},
 }}},nil)
 if err!=nil{log.Printf("material cleanup %d: %v",id,err);return err}
 if filepath.Base(name)!=name||name=="."||name==""{return errors.New("invalid stored filename")}
 if err=os.Remove(filepath.Join(uploadDir,name));err!=nil&&!os.IsNotExist(err){log.Printf("material file cleanup %d: %v",id,err);return err}
 _,err=db.Exec("DELETE FROM material_cleanup_jobs WHERE material_id=?",id)
 return err
}
func runMaterialCleanup(){
 for {
  rows,err:=db.Query("SELECT material_id,class_id,storage_name FROM material_cleanup_jobs ORDER BY material_id LIMIT 100")
  if err==nil{
   type job struct{id,class int64;name string};jobs:=[]job{}
   for rows.Next(){var j job;if rows.Scan(&j.id,&j.class,&j.name)==nil{jobs=append(jobs,j)}};rows.Close()
   for _,j:=range jobs{cleanupMaterial(j.id,j.class,j.name)}
  }else{log.Printf("material cleanup queue: %v",err)}
  time.Sleep(30*time.Second)
 }
}
// @api DELETE /api/assistant/conversations/{id} | 删除本人会话及消息 | auth | - | Deleted
func deleteConversation(w http.ResponseWriter,r *http.Request,u user){
 id,err:=strconv.ParseInt(r.PathValue("id"),10,64);if err!=nil||id<1{failure(w,404);return}
 tx,err:=db.Begin();if err!=nil{unavailable(w);return};defer tx.Rollback()
 var busy bool
 err=tx.QueryRow("SELECT busy FROM assistant_conversations WHERE id=? AND user_id=? AND class_id=? FOR UPDATE",id,u.ID,u.ClassID).Scan(&busy)
 if errors.Is(err,sql.ErrNoRows){failure(w,404);return};if err!=nil{unavailable(w);return}
 if busy{failure(w,409);return}
 if _,err=tx.Exec("DELETE FROM assistant_conversations WHERE id=? AND user_id=? AND class_id=?",id,u.ID,u.ClassID);err!=nil{unavailable(w);return}
 if err=tx.Commit();err!=nil{unavailable(w);return}
 respond(w,map[string]bool{"deleted":true})
}
