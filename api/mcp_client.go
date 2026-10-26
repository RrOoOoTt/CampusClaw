package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const mcpProtocol = "2025-03-26"

var mcpToolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
var mcpServerName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
var mcpTokenEnv = regexp.MustCompile(`^MCP_(AUTH_TOKEN|TOKEN_[A-Z0-9_]{1,48})$`)
var mcpID atomic.Int64
var mcpHTTP = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type mcpServer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Connected bool   `json:"connected"`
	LastError string `json:"last_error"`
	HasToken  bool   `json:"has_token"`
	TokenEnv  string `json:"-"`
	SessionID string `json:"-"`
}
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
}
type mcpRef struct{ Tool, Server string }

func parseToolRefs(s string) ([]mcpRef, error) {
	refs := []mcpRef{}
	seen := map[string]bool{}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == ';' || r == '；' })
	if len(parts) == 0 || len(parts) > 8 {
		return nil, errors.New("工具需为knowledge_search或tool@server，最多8项")
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "knowledge_search" {
			continue
		}
		bits := strings.Split(part, "@")
		if len(bits) != 2 || !mcpToolName.MatchString(bits[0]) || !mcpServerName.MatchString(bits[1]) {
			return nil, errors.New("工具格式须为knowledge_search或tool@server；工具名小写字母开头且无连字符")
		}
		if seen[part] {
			return nil, errors.New("工具引用重复")
		}
		seen[part] = true
		refs = append(refs, mcpRef{bits[0], bits[1]})
	}
	return refs, nil
}
func (ref mcpRef) String() string { return ref.Tool + "@" + ref.Server }
func (ref mcpRef) FunctionName() string {
	return "mcp_" + strings.ReplaceAll(ref.Server, "-", "_") + "__" + ref.Tool
}
func validateMCPURL(base string) error {
	u, e := url.Parse(base)
	if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("Base URL须为http(s)主机地址，不含凭据、路径或查询参数")
	}
	allowed := os.Getenv("MCP_ALLOWED_HOSTS")
	if allowed == "" {
		allowed = "mcp-demo"
	}
	for _, host := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimSpace(host), u.Hostname()) {
			return nil
		}
	}
	return errors.New("主机未在服务端MCP_ALLOWED_HOSTS中允许")
}
func mcpRPC(ctx context.Context, s *mcpServer, method string, params any, out any) error {
	if e := validateMCPURL(s.BaseURL); e != nil {
		return e
	}
	token := os.Getenv(s.TokenEnv)
	if len(strings.TrimSpace(token)) < 32 {
		return errors.New("服务端未配置有效MCP令牌环境变量（至少32字符）")
	}
	id := mcpID.Add(1)
	body, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(s.BaseURL, "/")+"/mcp", bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("MCP-Protocol-Version", mcpProtocol)
	if s.SessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.SessionID)
	}
	res, e := mcpHTTP.Do(req)
	if e != nil {
		return errors.New("MCP服务连接失败或超时，请检查地址与网络")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return errors.New("MCP认证失败：两侧令牌不一致")
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("MCP返回HTTP %d", res.StatusCode)
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		return errors.New("本课程客户端要求MCP返回application/json")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 128<<10))
	if e != nil {
		return errors.New("MCP响应读取失败")
	}
	if len(data) >= 128<<10 {
		return errors.New("MCP响应过大")
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &response) != nil || response.JSONRPC != "2.0" || response.ID != id {
		return errors.New("MCP响应协议或id无效")
	}
	if response.Error != nil {
		return fmt.Errorf("MCP方法失败，JSON-RPC code=%d", response.Error.Code)
	}
	if session := res.Header.Get("Mcp-Session-Id"); session != "" {
		if len(session) > 255 {
			return errors.New("MCP会话标识过长")
		}
		s.SessionID = session
	}
	if out != nil && json.Unmarshal(response.Result, out) != nil {
		return errors.New("MCP结果格式无效")
	}
	return nil
}
func mcpInitialized(ctx context.Context, s mcpServer) error {
	body := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(s.BaseURL, "/")+"/mcp", bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+os.Getenv(s.TokenEnv))
	req.Header.Set("MCP-Protocol-Version", mcpProtocol)
	if s.SessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.SessionID)
	}
	res, e := mcpHTTP.Do(req)
	if e != nil {
		return errors.New("MCP初始化通知失败")
	}
	defer res.Body.Close()
	if res.StatusCode != 202 && res.StatusCode != 200 {
		return fmt.Errorf("MCP初始化通知HTTP %d", res.StatusCode)
	}
	return nil
}
func discoverMCP(ctx context.Context, s *mcpServer) ([]mcpTool, error) {
	s.SessionID = ""
	var init struct {
		Protocol     string                     `json:"protocolVersion"`
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if e := mcpRPC(ctx, s, "initialize", map[string]any{"protocolVersion": mcpProtocol, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "CampusClaw", "version": "week06"}}, &init); e != nil {
		return nil, e
	}
	if init.Protocol != mcpProtocol || init.Capabilities["tools"] == nil {
		return nil, errors.New("MCP版本或tools能力不受支持")
	}
	if e := mcpInitialized(ctx, *s); e != nil {
		return nil, e
	}
	tools := []mcpTool{}
	names := map[string]bool{}
	cursor := ""
	for page := 0; page < 8; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var list struct {
			Tools []mcpTool `json:"tools"`
			Next  string    `json:"nextCursor"`
		}
		if e := mcpRPC(ctx, s, "tools/list", params, &list); e != nil {
			return nil, e
		}
		for _, tool := range list.Tools {
			if !mcpToolName.MatchString(tool.Name) || names[tool.Name] || len([]rune(tool.Description)) > 1000 || strings.TrimSpace(tool.Description) == "" {
				return nil, errors.New("发现工具名称、说明或重复项无效")
			}
			rawTool, _ := json.Marshal(tool)
			if secret := os.Getenv(s.TokenEnv); secret != "" && strings.Contains(string(rawTool), secret) {
				return nil, errors.New("工具说明包含敏感信息，已拒绝")
			}
			if tool.Schema["type"] != "object" {
				return nil, errors.New("工具输入结构必须是object")
			}
			if e := validateToolSchema(tool.Schema, 0); e != nil {
				return nil, e
			}
			names[tool.Name] = true
			tools = append(tools, tool)
		}
		if len(tools) > 32 {
			return nil, errors.New("发现工具超过32项")
		}
		if list.Next == "" {
			return tools, nil
		}
		if list.Next == cursor {
			return nil, errors.New("工具分页游标重复")
		}
		cursor = list.Next
	}
	return nil, errors.New("工具分页次数超限")
}
func validateToolSchema(s map[string]any, depth int) error {
	if depth > 4 || s == nil {
		return errors.New("工具结构为空或嵌套过深")
	}
	if _, ok := s["$ref"]; ok {
		return errors.New("本课程不支持工具schema的$ref")
	}
	// Reject constraints we cannot enforce instead of silently ignoring them.
	for key := range s {
		switch key {
		case "type", "properties", "required", "additionalProperties", "enum", "items", "title", "description", "$schema":
		default:
			return fmt.Errorf("工具schema字段%s暂不支持", key)
		}
	}
	if raw, exists := s["enum"]; exists {
		values, ok := raw.([]any)
		if !ok || len(values) == 0 || len(values) > 100 {
			return errors.New("enum须为1至100项数组")
		}
	}
	if raw, exists := s["additionalProperties"]; exists {
		if value, ok := raw.(bool); !ok || value {
			return errors.New("仅支持additionalProperties=false")
		}
	}
	kind, _ := s["type"].(string)
	switch kind {
	case "object":
		props, ok := s["properties"].(map[string]any)
		if !ok {
			return errors.New("工具object需要properties")
		}
		if len(props) > 20 {
			return errors.New("工具参数过多")
		}
		for _, raw := range props {
			child, ok := raw.(map[string]any)
			if !ok {
				return errors.New("工具参数结构无效")
			}
			if e := validateToolSchema(child, depth+1); e != nil {
				return e
			}
		}
		if req, ok := s["required"]; ok {
			list, ok := req.([]any)
			if !ok {
				return errors.New("required格式无效")
			}
			for _, raw := range list {
				k, ok := raw.(string)
				if !ok || props[k] == nil {
					return errors.New("required包含未声明参数")
				}
			}
		}
	case "array":
		child, ok := s["items"].(map[string]any)
		if !ok {
			return errors.New("数组需要items")
		}
		return validateToolSchema(child, depth+1)
	case "string", "number", "integer", "boolean":
	default:
		return errors.New("工具参数type不受支持")
	}
	return nil
}
func validateToolValue(value any, s map[string]any, depth int) error {
	if depth > 4 {
		return errors.New("参数嵌套超限")
	}
	kind, _ := s["type"].(string)
	switch kind {
	case "object":
		v, ok := value.(map[string]any)
		if !ok {
			return errors.New("参数必须是对象")
		}
		props, _ := s["properties"].(map[string]any)
		required, _ := s["required"].([]any)
		for _, r := range required {
			if _, ok := v[r.(string)]; !ok {
				return fmt.Errorf("缺少参数%s", r)
			}
		}
		for k, val := range v {
			child, ok := props[k].(map[string]any)
			if !ok {
				return fmt.Errorf("未声明参数%s", k)
			}
			if e := validateToolValue(val, child, depth+1); e != nil {
				return e
			}
		}
	case "string":
		v, ok := value.(string)
		if !ok || len([]rune(v)) > 512 {
			return errors.New("字符串参数无效或过长")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("参数应为boolean")
		}
	case "number", "integer":
		v, ok := value.(float64)
		if !ok || v > 1e12 || v < -1e12 || kind == "integer" && v != float64(int64(v)) {
			return errors.New("数字参数无效")
		}
	case "array":
		v, ok := value.([]any)
		if !ok || len(v) > 20 {
			return errors.New("数组参数无效")
		}
		child, _ := s["items"].(map[string]any)
		for _, item := range v {
			if e := validateToolValue(item, child, depth+1); e != nil {
				return e
			}
		}
	default:
		return errors.New("参数类型不受支持")
	}
	if enum, ok := s["enum"].([]any); ok {
		found := false
		for _, v := range enum {
			a, _ := json.Marshal(v)
			b, _ := json.Marshal(value)
			if bytes.Equal(a, b) {
				found = true
			}
		}
		if !found {
			return errors.New("参数不在枚举范围内")
		}
	}
	return nil
}
func checkToolResult(text, token string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("工具返回空内容")
	}
	if len([]rune(text)) > 3000 {
		return "", errors.New("工具结果超过3000字")
	}
	lower := strings.ToLower(text)
	for _, bad := range []string{"ignore previous", "ignore all instructions", "system prompt", "忽略之前", "忽略所有", "泄露", "authorization:", "bearer sk-"} {
		if strings.Contains(lower, bad) {
			return "", errors.New("工具结果命中指令或敏感信息检查，已整条丢弃")
		}
	}
	if token != "" && strings.Contains(text, token) {
		return "", errors.New("工具结果包含令牌，已丢弃")
	}
	return text, nil
}
