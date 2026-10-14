package main

import (
 "net/http/httptest"
 "strings"
 "testing"
)

func TestRequestToken(t *testing.T) {
 token := strings.Repeat("ab",32)
 for _, tc := range []struct{header string; valid bool}{
  {"Bearer "+token,true}, {"bearer "+token,true}, {"",false},
  {"Basic "+token,false}, {"Bearer invalid",false},
  {"Bearer "+strings.Repeat("z",64),false}, {"Bearer "+token+" extra",false},
 } {
  r := httptest.NewRequest("GET","/api/me",nil)
  if tc.header!="" {r.Header.Set("Authorization",tc.header)}
  r.Header.Set("Cookie","cc_session="+token)
  got,ok := requestToken(r)
  if ok!=tc.valid || ok&&got!=token {t.Fatalf("header %q: got %q, %v",tc.header,got,ok)}
 }
 r:=httptest.NewRequest("GET","/api/me",nil)
 r.Header.Add("Authorization","Bearer "+token)
 r.Header.Add("Authorization","Bearer "+token)
 if _,ok:=requestToken(r);ok {t.Fatal("duplicate Authorization accepted")}
}
