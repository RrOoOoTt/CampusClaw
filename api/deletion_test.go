package main

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"
)

func TestDeleteMaterialRejectsStudentBeforeDatabase(t *testing.T){
 r:=httptest.NewRequest("DELETE","/api/materials/1",nil);r.SetPathValue("id","1")
 w:=httptest.NewRecorder();deleteMaterial(w,r,user{Role:"student",ClassID:1})
 if w.Code!=403{t.Fatalf("got %d",w.Code)}
}
func TestDeleteRejectsInvalidID(t *testing.T){
 for _,handler:=range []handler{deleteMaterial,deleteConversation}{
  r:=httptest.NewRequest("DELETE","/",nil);r.SetPathValue("id","invalid")
  w:=httptest.NewRecorder();handler(w,r,user{Role:"teacher",ClassID:1})
  if w.Code!=404{t.Fatalf("got %d",w.Code)}
 }
}
func TestCleanupFailurePreservesFileAndScopesVectors(t *testing.T){
 oldURL,oldDir:=qdrantURL,uploadDir;defer func(){qdrantURL,uploadDir=oldURL,oldDir}()
 uploadDir=t.TempDir();name:="fixture.md"
 if err:=os.WriteFile(filepath.Join(uploadDir,name),[]byte("fixture"),0600);err!=nil{t.Fatal(err)}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Method!="POST"||r.URL.Path!="/collections/"+collection+"/points/delete"||r.URL.Query().Get("wait")!="true"{t.Error("wrong Qdrant request")}
  var body struct{Filter struct{Must []struct{Key string `json:"key"`;Match struct{Value int64 `json:"value"`} `json:"match"`} `json:"must"`} `json:"filter"`}
  if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Error(err)}
  values:=map[string]int64{};for _,term:=range body.Filter.Must{values[term.Key]=term.Match.Value}
  if len(values)!=2||values["material_id"]!=42||values["class_id"]!=7{t.Error("unscoped vector deletion")}
  w.WriteHeader(503)
 }));defer server.Close();qdrantURL=server.URL
 if cleanupMaterial(42,7,name)==nil{t.Fatal("dependency failure must retain cleanup job")}
 if _,err:=os.Stat(filepath.Join(uploadDir,name));err!=nil{t.Fatal("retry must retain original file",err)}
}
