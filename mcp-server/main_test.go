package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRPCAuthAndTools(t *testing.T) {
	h := handler(strings.Repeat("t", 32))
	req := func(raw, token string) (int, map[string]any) {
		r := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(raw))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	for _, token := range []string{"", "wrong"} {
		status, _ := req(list, token)
		if status != 401 {
			t.Fatal(status)
		}
	}
	token := strings.Repeat("t", 32)
	status, result := req(list, token)
	if status != 200 || len(result["result"].(map[string]any)["tools"].([]any)) != 2 {
		t.Fatal(result)
	}
	for _, raw := range []string{`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"formula_hint","arguments":{"topic":"triangle_area"}}}`, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"unit_convert","arguments":{"from":"m","to":"cm"}}}`} {
		_, result = req(raw, token)
		if result["result"] == nil || result["error"] != nil {
			t.Fatal(result)
		}
	}
	_, result = req(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"unit_convert","arguments":{"from":"kg","to":"cm"}}}`, token)
	if result["result"].(map[string]any)["isError"] != true {
		t.Fatal("dimension error not surfaced")
	}
	for _, raw := range []string{`{`, `{"jsonrpc":"2.0","id":1,"method":"unknown"}`, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing","arguments":{}}}`} {
		_, result = req(raw, token)
		if result["error"] == nil {
			t.Fatal("invalid RPC accepted")
		}
	}
}
func TestOnlyHintsNoFinalValues(t *testing.T) {
	text, e := toolText("unit_convert", map[string]any{"from": "m", "to": "cm"})
	if e != nil || !strings.Contains(text, "1 m = 100 cm") || !strings.Contains(text, "不返回题目最终得数") {
		t.Fatal(text, e)
	}
	if _, e = toolText("formula_hint", map[string]any{"topic": "triangle_area", "execute": "sh"}); e == nil {
		t.Fatal("extra args accepted")
	}
}
