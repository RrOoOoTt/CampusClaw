package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func selfcheck() error {
	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://127.0.0.1:" + os.Getenv("PORT")
	if os.Getenv("PORT") == "" {
		base = "http://127.0.0.1:8090"
	}
	res, e := client.Get(base + "/health")
	if e != nil {
		return e
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("health HTTP %d", res.StatusCode)
	}
	call := func(method string, params any, token string) (int, map[string]any, error) {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", base+"/mcp", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r, e := client.Do(req)
		if e != nil {
			return 0, nil, e
		}
		defer r.Body.Close()
		data, _ := io.ReadAll(r.Body)
		var out map[string]any
		json.Unmarshal(data, &out)
		return r.StatusCode, out, nil
	}
	for _, token := range []string{"", "wrong-token"} {
		status, _, e := call("tools/list", map[string]any{}, token)
		if e != nil || status != 401 {
			return fmt.Errorf("unauthorized check failed")
		}
	}
	token := os.Getenv("MCP_AUTH_TOKEN")
	results := []any{}
	for _, tc := range []struct {
		method string
		params any
	}{{"initialize", map[string]any{"protocolVersion": protocolVersion}}, {"tools/list", map[string]any{}}, {"tools/call", map[string]any{"name": "formula_hint", "arguments": map[string]string{"topic": "triangle_area"}}}, {"tools/call", map[string]any{"name": "unit_convert", "arguments": map[string]string{"from": "m", "to": "cm"}}}} {
		status, result, e := call(tc.method, tc.params, token)
		if e != nil || status != 200 || result["error"] != nil || result["result"] == nil {
			return fmt.Errorf("%s check failed", tc.method)
		}
		results = append(results, map[string]any{"method": tc.method, "status": status, "result": result["result"]})
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"health": "ok", "unauthorized": "401", "authenticated_checks": results})
}
