package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const protocolVersion = "2025-03-26"

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func tools() []tool {
	return []tool{
		{"formula_hint", "提供数学公式的选择提示与检查问题；不计算题目最终得数。", map[string]any{"type": "object", "properties": map[string]any{"topic": map[string]any{"type": "string", "enum": []string{"triangle_area", "rectangle_area", "linear_equation"}}}, "required": []string{"topic"}, "additionalProperties": false}},
		{"unit_convert", "提供单位换算关系及方法提示，不对题目数值直接计算答案。", map[string]any{"type": "object", "properties": map[string]any{"from": map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "kg", "g"}}, "to": map[string]any{"type": "string", "enum": []string{"m", "cm", "mm", "km", "kg", "g"}}}, "required": []string{"from", "to"}, "additionalProperties": false}},
	}
}
func toolText(name string, args map[string]any) (string, error) {
	str := func(k string) string { s, _ := args[k].(string); return s }
	switch name {
	case "formula_hint":
		if len(args) != 1 {
			return "", fmt.Errorf("仅允许topic参数")
		}
		switch str("topic") {
		case "triangle_area":
			return "公式提示：先标出一条底边与对应的垂直高。三角形可补成平行四边形，检查面积关系；再自行选择底×高÷2，不代入题目数值。检查问题：你选的高是否垂直于这条底边？", nil
		case "rectangle_area":
			return "公式提示：检查长与宽是否使用相同单位，再选择长×宽；请自行代入。检查问题：图形是否真为长方形？", nil
		case "linear_equation":
			return "方程提示：先合并同类项，再对等式两边实施相同运算，保留未知数；请自行求解并代回检验。", nil
		default:
			return "", fmt.Errorf("topic不受支持")
		}
	case "unit_convert":
		if len(args) != 2 {
			return "", fmt.Errorf("仅允许from和to参数")
		}
		units := map[string]struct {
			kind  string
			scale float64
		}{"km": {"length", 1000}, "m": {"length", 1}, "cm": {"length", .01}, "mm": {"length", .001}, "kg": {"mass", 1000}, "g": {"mass", 1}}
		a, ok := units[str("from")]
		b, ok2 := units[str("to")]
		if !ok || !ok2 || a.kind != b.kind {
			return "", fmt.Errorf("只允许同一量纲的已支持单位换算")
		}
		return fmt.Sprintf("换算提示：1 %s = %g %s。先统一单位，再自行将题目中的数值乘以这个换算系数；本工具不返回题目最终得数。", str("from"), a.scale/b.scale, str("to")), nil
	default:
		return "", fmt.Errorf("工具不存在")
	}
}
func handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", 401)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			http.Error(w, "browser origins not allowed", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		fail := func(code int, message string) {
			id := req.ID
			if len(id) == 0 {
				id = json.RawMessage("null")
			}
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		if e := decoder.Decode(&req); e != nil {
			fail(-32700, "请求必须是有效JSON对象")
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			fail(-32600, "不支持批量或尾随JSON")
			return
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			fail(-32600, "需要jsonrpc=2.0和method")
			return
		}
		if req.Method == "notifications/initialized" && len(req.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			fail(-32600, "请求需要id")
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if json.Unmarshal(req.Params, &p) != nil || p.ProtocolVersion == "" {
				fail(-32602, "缺少protocolVersion")
				return
			}
			result = map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "campus-tools", "version": "1.0.0"}}
		case "tools/list":
			result = map[string]any{"tools": tools()}
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(req.Params, &p) != nil || p.Arguments == nil {
				fail(-32602, "需要name及arguments对象")
				return
			}
			known := false
			for _, t := range tools() {
				if t.Name == p.Name {
					known = true
				}
			}
			if !known {
				fail(-32602, "工具不存在")
				return
			}
			text, e := toolText(p.Name, p.Arguments)
			if e != nil {
				result = map[string]any{"isError": true, "content": []any{map[string]string{"type": "text", "text": e.Error()}}}
			} else {
				result = map[string]any{"isError": false, "content": []any{map[string]string{"type": "text", "text": text}}}
			}
		default:
			fail(-32601, "不支持的方法")
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	})
	return mux
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "selfcheck" {
		if e := selfcheck(); e != nil {
			log.Fatal(e)
		}
		return
	}
	token := os.Getenv("MCP_AUTH_TOKEN")
	if len(strings.TrimSpace(token)) < 32 {
		log.Fatal("MCP_AUTH_TOKEN must contain at least 32 characters")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	s := &http.Server{Addr: ":" + port, Handler: handler(token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	log.Fatal(s.ListenAndServe())
}
