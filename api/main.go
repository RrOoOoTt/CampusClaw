package main

import (
 "context"
 "crypto/rand"
 "crypto/hmac"
 "crypto/sha256"
 "github.com/go-sql-driver/mysql"
 "database/sql"
 "embed"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "log"
 "net/http"
 "os"
 "path/filepath"
 "strconv"
 "strings"
 "sync"
 "time"
 "unicode/utf8"


 "golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schema embed.FS
var db *sql.DB
var sessionSecret []byte
var uploadDir string
var cookieSecure bool
const maxUpload = 2 << 20
const failBody = `{"error":"请求失败"}`
type user struct { ID int64 `json:"-"`; Username string `json:"username"`; Role string `json:"role"`; ClassID int64 `json:"class_id"` }
type material struct { ID int64 `json:"id"`; Title string `json:"title"`; Size int64 `json:"size"`; Created time.Time `json:"created_at"`; Body string `json:"body,omitempty"` }
type attempt struct { count int; until time.Time }
var attempts = struct {sync.Mutex; items map[string]attempt}{items:map[string]attempt{}}

func required(key string) string { v:=os.Getenv(key); if v=="" {log.Fatalf("missing %s",key)}; return v }
func main() {
 if len(os.Args)>1 && os.Args[1]=="mock-gateway" {runMockGateway();return}
 host:=required("DB_HOST"); name:=required("DB_NAME"); pass:=required("DB_PASSWORD")
 secret:=required("SESSION_SECRET"); if len(secret)<32 {log.Fatal("SESSION_SECRET must have at least 32 characters")}
 sessionSecret=[]byte(secret);uploadDir=required("UPLOAD_DIR"); cookieSecure=os.Getenv("COOKIE_SECURE")=="true"
 cfg:=mysql.NewConfig();cfg.User=required("DB_USER");cfg.Passwd=pass;cfg.Net="tcp";cfg.Addr=host+":3306";cfg.DBName=name;cfg.ParseTime=true;cfg.MultiStatements=true;cfg.Params=map[string]string{"charset":"utf8mb4"};dsn:=cfg.FormatDSN()
 var err error; db,err=sql.Open("mysql",dsn);if err!=nil {log.Fatal(err)}
 for i:=0;i<30;i++ {ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);err=db.PingContext(ctx);cancel();if err==nil {break};time.Sleep(time.Second)}
 if err!=nil {log.Fatal(err)}
 sqlText,_:=schema.ReadFile("schema.sql");if _,err=db.Exec(string(sqlText));err!=nil {log.Fatal(err)}
 if err=os.MkdirAll(uploadDir,0700);err!=nil {log.Fatal(err)}
 seed()
 initRetrieval()
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /health",func(w http.ResponseWriter,r *http.Request){w.Write([]byte("ok"))})
 mux.HandleFunc("POST /api/login",login)
 mux.HandleFunc("POST /api/logout",auth(logout))
 mux.HandleFunc("GET /api/me",auth(me))
 mux.HandleFunc("GET /api/materials",auth(list))
 mux.HandleFunc("POST /api/materials",auth(create))
 mux.HandleFunc("GET /api/materials/{id}",auth(detail))
 mux.HandleFunc("GET /api/materials/{id}/file",auth(file))
 mux.HandleFunc("POST /api/materials/{id}/reindex",auth(reindex))
 mux.HandleFunc("GET /api/search",auth(search))
 mux.HandleFunc("POST /api/ask",auth(ask))
 log.Fatal(http.ListenAndServe(":8081",security(mux)))
}
func seed(){
 accounts:=[]struct{ name,role,class,password string }{
 {"teacher_a","teacher","A",required("SEED_TEACHER_A_PASSWORD")},{"student_a","student","A",required("SEED_STUDENT_A_PASSWORD")},{"teacher_b","teacher","B",required("SEED_TEACHER_B_PASSWORD")},{"student_b","student","B",required("SEED_STUDENT_B_PASSWORD")},
 }
 for _,a:=range accounts {var classID int64;_,err:=db.Exec("INSERT IGNORE INTO classes(name) VALUES(?)",a.class);if err!=nil {log.Fatal(err)};if err=db.QueryRow("SELECT id FROM classes WHERE name=?",a.class).Scan(&classID);err!=nil {log.Fatal(err)}
 var exists int;err=db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?",a.name).Scan(&exists);if err!=nil {log.Fatal(err)};if exists>0 {continue}
 hash,err:=bcrypt.GenerateFromPassword([]byte(a.password),bcrypt.DefaultCost);if err!=nil {log.Fatal(err)}
 if _,err=db.Exec("INSERT INTO users(username,password_hash,role,class_id) VALUES(?,?,?,?)",a.name,string(hash),a.role,classID);err!=nil {log.Fatal(err)}
 }
}
func security(next http.Handler) http.Handler {return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Cache-Control","no-store"); if r.Method=="POST" && strings.HasPrefix(r.URL.Path,"/api/") {origin:=r.Header.Get("Origin");if origin!="" {host:=r.Host;if r.Header.Get("X-Forwarded-Host")!="" {host=r.Header.Get("X-Forwarded-Host")};if origin!="http://"+host && origin!="https://"+host {failure(w,http.StatusForbidden);return}};if r.Header.Get("Sec-Fetch-Site")=="cross-site" {failure(w,http.StatusForbidden);return}};next.ServeHTTP(w,r)})}
func failure(w http.ResponseWriter,status int){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);io.WriteString(w,failBody)}
func unavailable(w http.ResponseWriter){failure(w,http.StatusServiceUnavailable)}
func respond(w http.ResponseWriter,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");json.NewEncoder(w).Encode(v)}
func randomToken() string {b:=make([]byte,32);if _,err:=rand.Read(b);err!=nil {log.Fatal(err)};return hex.EncodeToString(b)}
func tokenHash(s string) [32]byte {mac:=hmac.New(sha256.New,sessionSecret);mac.Write([]byte(s));var result [32]byte;copy(result[:],mac.Sum(nil));return result}
func setCookie(w http.ResponseWriter,value string,age int){http.SetCookie(w,&http.Cookie{Name:"cc_session",Value:value,Path:"/api",HttpOnly:true,Secure:cookieSecure,SameSite:http.SameSiteLaxMode,MaxAge:age})}
func login(w http.ResponseWriter,r *http.Request){var input struct{Username string `json:"username"`;Password string `json:"password"`};r.Body=http.MaxBytesReader(w,r.Body,4096);if json.NewDecoder(r.Body).Decode(&input)!=nil {failure(w,400);return};input.Username=strings.TrimSpace(input.Username)
 ip:=r.Header.Get("X-Real-IP");if ip=="" {ip=r.RemoteAddr};key:=ip+":"+input.Username;attempts.Lock();a:=attempts.items[key];limited:=a.count>=5 && time.Now().Before(a.until);attempts.Unlock()
 var id int64;var hash string;err:=db.QueryRow("SELECT id,password_hash FROM users WHERE username=?",input.Username).Scan(&id,&hash)
 if err!=nil && !errors.Is(err,sql.ErrNoRows){unavailable(w);return}
 if hash=="" {hash="$2a$10$C6UzMDM.H6dfI/f/IKcEe.8ZQ5J6pYwjZY/ZgGeIExHJhiBynB3Lu"}
 valid:=bcrypt.CompareHashAndPassword([]byte(hash),[]byte(input.Password))==nil
 if !valid||limited||id==0 {attempts.Lock();v:=attempts.items[key];if time.Now().After(v.until) {v.count=0};v.count++;v.until=time.Now().Add(5*time.Minute);attempts.items[key]=v;attempts.Unlock();failure(w,401);return}
 attempts.Lock();delete(attempts.items,key);attempts.Unlock()
 token:=randomToken();digest:=tokenHash(token);_,err=db.Exec("INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)",digest[:],id,time.Now().Add(12*time.Hour));if err!=nil {unavailable(w);return};setCookie(w,token,12*3600);respond(w,map[string]bool{"ok":true})
}
type handler func(http.ResponseWriter,*http.Request,user)
func auth(next handler) http.HandlerFunc {return func(w http.ResponseWriter,r *http.Request){c,err:=r.Cookie("cc_session");if err!=nil||len(c.Value)!=64 {failure(w,401);return};digest:=tokenHash(c.Value);var u user;var expires time.Time
 err=db.QueryRow("SELECT u.id,u.username,u.role,u.class_id,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?",digest[:]).Scan(&u.ID,&u.Username,&u.Role,&u.ClassID,&expires)
 if errors.Is(err,sql.ErrNoRows)||err==nil && time.Now().After(expires) {failure(w,401);return};if err!=nil {unavailable(w);return};if u.Role!="teacher"&&u.Role!="student" {failure(w,401);return};next(w,r,u)}}
func logout(w http.ResponseWriter,r *http.Request,u user){c,_:=r.Cookie("cc_session");d:=tokenHash(c.Value);if _,err:=db.Exec("DELETE FROM sessions WHERE token_hash=?",d[:]);err!=nil {unavailable(w);return};setCookie(w,"",-1);respond(w,map[string]bool{"ok":true})}
func me(w http.ResponseWriter,r *http.Request,u user){respond(w,u)}
func list(w http.ResponseWriter,r *http.Request,u user){q:=strings.TrimSpace(r.URL.Query().Get("q"));if len(q)>200 {failure(w,400);return};rows,err:=db.Query("SELECT id,title,size_bytes,created_at FROM materials WHERE class_id=? AND title LIKE ? ORDER BY created_at DESC,id DESC LIMIT 100",u.ClassID,"%"+escapeLike(q)+"%");if err!=nil {unavailable(w);return};defer rows.Close();items:=[]material{};for rows.Next(){var m material;if rows.Scan(&m.ID,&m.Title,&m.Size,&m.Created)!=nil {unavailable(w);return};items=append(items,m)};if rows.Err()!=nil {unavailable(w);return};respond(w,items)}
func escapeLike(s string) string {s=strings.ReplaceAll(s,`\`,`\\`);s=strings.ReplaceAll(s,`%`,`\%`);return strings.ReplaceAll(s,`_`,`\_`)}
func getMaterial(w http.ResponseWriter,r *http.Request,u user) (material,string,bool) {id,err:=strconv.ParseInt(r.PathValue("id"),10,64);if err!=nil||id<=0 {failure(w,404);return material{},"",false};var m material;var path string
 err=db.QueryRow("SELECT m.id,m.title,m.size_bytes,m.created_at,k.body,m.storage_name FROM materials m JOIN knowledge_entries k ON k.material_id=m.id WHERE m.id=? AND m.class_id=?",id,u.ClassID).Scan(&m.ID,&m.Title,&m.Size,&m.Created,&m.Body,&path)
 if errors.Is(err,sql.ErrNoRows){failure(w,404);return m,"",false};if err!=nil {unavailable(w);return m,"",false};return m,path,true}
func detail(w http.ResponseWriter,r *http.Request,u user){m,_,ok:=getMaterial(w,r,u);if ok {respond(w,m)}}
func file(w http.ResponseWriter,r *http.Request,u user){m,name,ok:=getMaterial(w,r,u);if !ok{return};f,err:=os.Open(filepath.Join(uploadDir,name));if err!=nil {log.Printf("file unavailable: %v",err);unavailable(w);return};defer f.Close();w.Header().Set("Content-Disposition",fmt.Sprintf("attachment; filename=%q",safeName(m.Title)));w.Header().Set("Content-Type","application/octet-stream");http.ServeContent(w,r,m.Title,m.Created,f)}
func safeName(s string) string {return strings.Map(func(r rune)rune{if r=='"'||r=='\\'||r=='/'||r=='\r'||r=='\n'||r<32{return '_'};return r},s)}
func create(w http.ResponseWriter,r *http.Request,u user){if u.Role!="teacher" {failure(w,403);return};r.Body=http.MaxBytesReader(w,r.Body,maxUpload+65536);if err:=r.ParseMultipartForm(maxUpload+65536);err!=nil {failure(w,413);return};if r.MultipartForm!=nil {defer r.MultipartForm.RemoveAll()};headers:=r.MultipartForm.File["file"];if len(headers)!=1 {failure(w,400);return};header:=headers[0];ext:=strings.ToLower(filepath.Ext(header.Filename));if ext!=".txt"&&ext!=".md" {failure(w,415);return};if header.Size>maxUpload {failure(w,413);return};title:=filepath.Base(header.Filename);if len(title)>255||title=="."||strings.ContainsAny(title,"\r\n") {failure(w,400);return}
 src,err:=header.Open();if err!=nil {failure(w,422);return};defer src.Close();content,err:=io.ReadAll(io.LimitReader(src,maxUpload+1));if err!=nil||len(content)>maxUpload {failure(w,413);return};if len(content)==0||!utf8.Valid(content)||strings.ContainsRune(string(content),0) {failure(w,422);return}
 opt:=chunkOptions{Strategy:r.FormValue("strategy")};if opt.Strategy=="custom" {opt.MaxChars,_=strconv.Atoi(r.FormValue("max_chars"));opt.OverlapRatio,_=strconv.ParseFloat(r.FormValue("overlap_ratio"),64);opt.Separator=r.FormValue("separator");opt.RemoveURLs=r.FormValue("remove_urls")=="true";opt.RemoveEmails=r.FormValue("remove_emails")=="true";opt.CollapseWhitespace=r.FormValue("collapse_whitespace")=="true"};if _,e:=validateOptions(opt);e!=nil {failure(w,400);return}
 name:=randomToken()+ext;path:=filepath.Join(uploadDir,name);out,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if err!=nil {unavailable(w);return};_,err=out.Write(content);closeErr:=out.Close();if err!=nil||closeErr!=nil {os.Remove(path);unavailable(w);return};committed:=false;defer func(){if !committed {os.Remove(path)}}()
 tx,err:=db.Begin();if err!=nil {unavailable(w);return};defer tx.Rollback();res,err:=tx.Exec("INSERT INTO materials(class_id,uploader_id,title,storage_name,size_bytes) VALUES(?,?,?,?,?)",u.ClassID,u.ID,title,name,len(content));if err!=nil {unavailable(w);return};id,err:=res.LastInsertId();if err!=nil {unavailable(w);return};if _,err=tx.Exec("INSERT INTO knowledge_entries(material_id,body) VALUES(?,?)",id,string(content));err!=nil {unavailable(w);return};if err=tx.Commit();err!=nil {unavailable(w);return};committed=true;status:="ready";if _,e:=indexMaterial(id,u.ClassID,string(content),opt);e!=nil {log.Printf("material %d indexing failed: %v",id,e);status="failed"};w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(201);json.NewEncoder(w).Encode(map[string]any{"id":id,"index_status":status})
}
