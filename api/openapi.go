package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
)

//go:embed *.go openapi-schemas.json swagger/*
var apiDocs embed.FS

func openAPISpec() map[string]any {
	schemasBytes, _ := apiDocs.ReadFile("openapi-schemas.json")
	var schemas map[string]any
	json.Unmarshal(schemasBytes, &schemas)
	paths := map[string]any{}
	files, _ := fs.Glob(apiDocs, "*.go")
	re := regexp.MustCompile(`(?m)^// @api (GET|POST|PUT|DELETE) (\S+) \| ([^\n|]+) \| (public|auth|teacher) \| (\S+) \| (\S+)`)
	schemaRef := func(n string) map[string]any {
		if strings.HasSuffix(n, "[]") {
			return map[string]any{"type": "array", "items": map[string]string{"$ref": "#/components/schemas/" + strings.TrimSuffix(n, "[]")}}
		}
		return map[string]any{"$ref": "#/components/schemas/" + n}
	}
	for _, file := range files {
		src, _ := apiDocs.ReadFile(file)
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			method, path, summary, access, input, output := strings.ToLower(m[1]), m[2], strings.TrimSpace(m[3]), m[4], m[5], m[6]
			op := map[string]any{"summary": summary, "operationId": method + strings.NewReplacer("/", "_", "{", "", "}", "").Replace(path), "tags": []string{strings.Split(strings.TrimPrefix(path, "/api/"), "/")[0]}}
			if access != "public" {
				op["security"] = []any{map[string]any{"BearerAuth": []string{}}}
				op["description"] = "需Bearer认证。"
				if access == "teacher" {
					op["description"] = "需Bearer认证且为本班教师。"
				}
			} else {
				op["security"] = []any{}
			}
			params := []any{}
			if strings.Contains(path, "{id}") {
				params = append(params, map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer", "format": "int64"}})
			}
			if input == "SearchQuery" {
				for _, name := range []string{"q", "mode", "agent_id"} {
					schema := map[string]any{"type": "string"}
					if name == "agent_id" {
						schema = map[string]any{"type": "integer", "format": "int64"}
					}
					if name == "mode" {
						schema["enum"] = []string{"keyword", "vector", "hybrid"}
					}
					params = append(params, map[string]any{"name": name, "in": "query", "required": name == "q" && path == "/api/search", "schema": schema})
				}
			} else if input != "-" {
				mime := "application/json"
				if input == "MaterialUpload" {
					mime = "multipart/form-data"
				}
				op["requestBody"] = map[string]any{"required": input != "ChunkOptions", "content": map[string]any{mime: map[string]any{"schema": schemaRef(input)}}}
			}
			if len(params) > 0 {
				op["parameters"] = params
			}
			mime := "application/json"
			responseSchema := schemaRef(output)
			description := "成功"
			if output == "SSE" {
				mime = "text/event-stream"
				responseSchema = map[string]any{"type": "string", "example": "event: meta\ndata: {\"citations\":[]}\n\nevent: delta\ndata: {\"text\":\"提示正文\"}\n\nevent: done\ndata: {\"state\":\"complete\"}\n\n"}
				description = "SSE：meta先给引用、选中技能与tool_calls；tool_call事件在正文前报告工具状态；delta新增文本；done完整正文与tool_calls；error生成失败。HTTP200不保证生成完成。"
			}
			if output == "Binary" {
				mime = "application/octet-stream"
				responseSchema = map[string]any{"type": "string", "format": "binary"}
			}
			code := "200"
			if method == "post" && (path == "/api/mcp/servers" || path == "/api/agents" || path == "/api/skills" || path == "/api/materials" || path == "/api/assistant/conversations") {
				code = "201"
			}
			responses := map[string]any{code: map[string]any{"description": description, "content": map[string]any{mime: map[string]any{"schema": responseSchema}}}}
			for c, d := range map[string]string{"400": "输入无效或班内名称重复", "401": "未登录或令牌失效", "403": "角色或同源检查不通过", "404": "不存在、跨班或非本人；响应相同", "409": "会话生成中或达到消息上限", "413": "请求体或文件过大", "415": "不支持的文件类型", "422": "文件内容无效", "503": "数据库、检索或模型依赖不可用"} {
				responses[c] = map[string]any{"description": d, "content": map[string]any{"application/json": map[string]any{"schema": schemaRef("Error")}}}
			}
			op["responses"] = responses
			if paths[path] == nil {
				paths[path] = map[string]any{}
			}
			paths[path].(map[string]any)[method] = op
		}
	}
	return map[string]any{"openapi": "3.0.3", "info": map[string]string{"title": "CampusClaw API", "version": "week05", "description": "由Go处理器的@api注解在启动时生成；使用随机Bearer会话令牌，非JWT。"}, "servers": []any{map[string]string{"url": "/"}}, "paths": paths, "components": map[string]any{"schemas": schemas, "securitySchemes": map[string]any{"BearerAuth": map[string]string{"type": "http", "scheme": "bearer", "bearerFormat": "Opaque session token"}}}}
}
func mountOpenAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) { respond(w, openAPISpec()) })
	assets, _ := fs.Sub(apiDocs, "swagger")
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusTemporaryRedirect)
	})
}
