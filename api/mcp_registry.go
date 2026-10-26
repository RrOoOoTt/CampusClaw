package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
)

var mcpRegistryMutex sync.Mutex

func initMCP() error {
	if e := addColumn("mcp_servers", "session_id", "VARCHAR(255) NOT NULL DEFAULT ''"); e != nil {
		return e
	}
	if e := addColumn("assistant_messages", "tool_calls", "JSON NULL"); e != nil {
		return e
	}
	var size int
	if e := db.QueryRow("SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='tutor_skills' AND COLUMN_NAME='tool'").Scan(&size); e != nil {
		return e
	}
	if size < 512 {
		_, e := db.Exec("ALTER TABLE tutor_skills MODIFY COLUMN tool VARCHAR(512) NOT NULL DEFAULT 'knowledge_search'")
		return e
	}
	return nil
}
func getMCPServer(id, class int64) (mcpServer, error) {
	var s mcpServer
	e := db.QueryRow("SELECT id,name,base_url,token_env,connected,last_error,session_id FROM mcp_servers WHERE id=? AND class_id=?", id, class).Scan(&s.ID, &s.Name, &s.BaseURL, &s.TokenEnv, &s.Connected, &s.LastError, &s.SessionID)
	s.HasToken = len(os.Getenv(s.TokenEnv)) >= 32
	return s, e
}
func lookupMCP(ref mcpRef, class int64) (mcpServer, mcpTool, error) {
	var s mcpServer
	var t mcpTool
	e := db.QueryRow("SELECT id,name,base_url,token_env,connected,last_error,session_id FROM mcp_servers WHERE name=? AND class_id=?", ref.Server, class).Scan(&s.ID, &s.Name, &s.BaseURL, &s.TokenEnv, &s.Connected, &s.LastError, &s.SessionID)
	if e != nil {
		return s, t, e
	}
	if !s.Connected {
		return s, t, errors.New("服务器未连接，工具调用被拒")
	}
	var raw []byte
	e = db.QueryRow("SELECT name,description,input_schema FROM mcp_tools WHERE server_id=? AND name=?", s.ID, ref.Tool).Scan(&t.Name, &t.Description, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &t.Schema)
	}
	return s, t, e
}
func validateSkillTools(s tutorSkill, class int64) error {
	refs, e := parseToolRefs(s.Tool)
	if e != nil {
		return e
	}
	for _, ref := range refs {
		_, _, e := lookupMCP(ref, class)
		if errors.Is(e, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		if e != nil {
			return errors.New("工具不在本班已连接服务器的发现清单中")
		}
	}
	return nil
}

// @api GET /api/mcp/servers | 本班MCP服务器（不含令牌） | teacher | - | MCPServer[]
func listMCPServers(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	rows, e := db.Query("SELECT id,name,base_url,token_env,connected,last_error,session_id FROM mcp_servers WHERE class_id=? ORDER BY id", u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	out := []mcpServer{}
	for rows.Next() {
		var s mcpServer
		if e = rows.Scan(&s.ID, &s.Name, &s.BaseURL, &s.TokenEnv, &s.Connected, &s.LastError, &s.SessionID); e != nil {
			unavailable(w)
			return
		}
		s.HasToken = len(os.Getenv(s.TokenEnv)) >= 32
		out = append(out, s)
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	respond(w, out)
}

// @api GET /api/mcp/servers/{id} | 读取本班MCP服务器 | teacher | - | MCPServer
func readMCPServer(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	s, e := getMCPServer(id, u.ClassID)
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	respond(w, s)
}

// @api POST /api/mcp/servers | 登记本班MCP服务器（令牌用环境变量引用） | teacher | MCPServerInput | MCPServer
// @api PUT /api/mcp/servers/{id} | 修改MCP登记并断开旧连接 | teacher | MCPServerInput | MCPServer
func writeMCPServer(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	var in struct {
		Name     string `json:"name"`
		BaseURL  string `json:"base_url"`
		TokenEnv string `json:"token_env"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		failure(w, 400)
		return
	}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if in.TokenEnv == "" {
		in.TokenEnv = "MCP_AUTH_TOKEN"
	}
	if !mcpServerName.MatchString(in.Name) || !mcpTokenEnv.MatchString(in.TokenEnv) {
		readableFailure(w, 400, "服务器名须小写字母开头，最多24字；令牌引用须为MCP_AUTH_TOKEN或MCP_TOKEN_前缀的环境变量")
		return
	}
	if e := validateMCPURL(in.BaseURL); e != nil {
		readableFailure(w, 400, e.Error())
		return
	}
	mcpRegistryMutex.Lock()
	defer mcpRegistryMutex.Unlock()
	id := int64(0)
	var e error
	if r.Method == "POST" {
		res, err := db.Exec("INSERT INTO mcp_servers(class_id,name,base_url,token_env) VALUES(?,?,?,?)", u.ClassID, in.Name, in.BaseURL, in.TokenEnv)
		if err != nil {
			databaseWriteFailure(w, err)
			return
		}
		id, e = res.LastInsertId()
	} else {
		id, e = routeID(r)
		if e != nil {
			failure(w, 404)
			return
		}
		if _, e = getMCPServer(id, u.ClassID); errors.Is(e, sql.ErrNoRows) {
			failure(w, 404)
			return
		} else if e != nil {
			unavailable(w)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			unavailable(w)
			return
		}
		defer tx.Rollback()
		if _, e = tx.Exec("UPDATE mcp_servers SET name=?,base_url=?,token_env=?,connected=FALSE,session_id='',last_error='' WHERE id=? AND class_id=?", in.Name, in.BaseURL, in.TokenEnv, id, u.ClassID); e != nil {
			databaseWriteFailure(w, e)
			return
		}
		if _, e = tx.Exec("DELETE FROM mcp_tools WHERE server_id=?", id); e != nil {
			unavailable(w)
			return
		}
		e = tx.Commit()
	}
	if e != nil {
		unavailable(w)
		return
	}
	s, e := getMCPServer(id, u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	if r.Method == "POST" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(201)
	}
	respond(w, s)
}
func disconnectMCP(id, class int64, reason string) error {
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE mcp_servers SET connected=FALSE,session_id='',last_error=? WHERE id=? AND class_id=?", limitCharacters(reason, 255), id, class); e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM mcp_tools WHERE server_id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}
func connectMCP(ctx context.Context, s mcpServer, class int64) (mcpServer, error) {
	if e := disconnectMCP(s.ID, class, ""); e != nil {
		return s, e
	}
	tools, e := discoverMCP(ctx, &s)
	if e != nil {
		disconnectMCP(s.ID, class, e.Error())
		s.Connected = false
		s.LastError = e.Error()
		return s, e
	}
	tx, e := db.Begin()
	if e != nil {
		return s, e
	}
	defer tx.Rollback()
	for _, tool := range tools {
		raw, err := json.Marshal(tool.Schema)
		if err != nil {
			return s, err
		}
		if _, e = tx.Exec("INSERT INTO mcp_tools(server_id,name,description,input_schema) VALUES(?,?,?,?)", s.ID, tool.Name, tool.Description, string(raw)); e != nil {
			return s, e
		}
	}
	if _, e = tx.Exec("UPDATE mcp_servers SET connected=TRUE,last_error='',session_id=? WHERE id=? AND class_id=?", s.SessionID, s.ID, class); e != nil {
		return s, e
	}
	if e = tx.Commit(); e != nil {
		return s, e
	}
	s.Connected = true
	s.LastError = ""
	s.HasToken = len(os.Getenv(s.TokenEnv)) >= 32
	return s, nil
}

// @api POST /api/mcp/servers/{id}/connect | 握手并发现，失败保持断开 | teacher | - | MCPServer
func connectMCPHandler(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	mcpRegistryMutex.Lock()
	defer mcpRegistryMutex.Unlock()
	s, e := getMCPServer(id, u.ClassID)
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	s, e = connectMCP(r.Context(), s, u.ClassID)
	if e != nil {
		readableFailure(w, 400, e.Error())
		return
	}
	respond(w, s)
}

// @api POST /api/mcp/servers/{id}/disconnect | 断开并清空发现缓存 | teacher | - | Deleted
func disconnectMCPHandler(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	mcpRegistryMutex.Lock()
	defer mcpRegistryMutex.Unlock()
	if _, e = getMCPServer(id, u.ClassID); errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	} else if e != nil {
		unavailable(w)
		return
	}
	if e = disconnectMCP(id, u.ClassID, ""); e != nil {
		unavailable(w)
		return
	}
	respond(w, map[string]bool{"ok": true})
}

// @api DELETE /api/mcp/servers/{id} | 删除本班MCP登记及缓存 | teacher | - | Deleted
func deleteMCPServer(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	mcpRegistryMutex.Lock()
	defer mcpRegistryMutex.Unlock()
	res, e := db.Exec("DELETE FROM mcp_servers WHERE id=? AND class_id=?", id, u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	n, e := res.RowsAffected()
	if e != nil {
		unavailable(w)
		return
	}
	if n == 0 {
		failure(w, 404)
		return
	}
	respond(w, map[string]bool{"deleted": true})
}

// @api GET /api/mcp/servers/{id}/tools | 本班服务器发现清单 | teacher | - | MCPTool[]
func listMCPTools(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	s, e := getMCPServer(id, u.ClassID)
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	out := []mcpTool{}
	if !s.Connected {
		respond(w, out)
		return
	}
	rows, e := db.Query("SELECT name,description,input_schema FROM mcp_tools WHERE server_id=? ORDER BY name", id)
	if e != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t mcpTool
		var raw []byte
		if rows.Scan(&t.Name, &t.Description, &raw) != nil || json.Unmarshal(raw, &t.Schema) != nil {
			unavailable(w)
			return
		}
		out = append(out, t)
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	respond(w, out)
}

// @api GET /api/mcp/calls | 本班最近100次工具调用审计 | teacher | - | MCPAudit[]
func listMCPCalls(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	rows, e := db.Query("SELECT c.id,c.tool_ref,s.name,c.status,u.username,c.round_no,c.arguments_json,c.result_text,c.detail,c.created_at,c.message_id FROM mcp_calls c JOIN mcp_servers s ON s.id=c.server_id AND s.class_id=c.class_id JOIN users u ON u.id=c.user_id WHERE c.class_id=? ORDER BY c.id DESC LIMIT 100", u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var ref, server, status, name, result, detail, created string
		var round int
		var args []byte
		var message sql.NullInt64
		if rows.Scan(&id, &ref, &server, &status, &name, &round, &args, &result, &detail, &created, &message) != nil {
			unavailable(w)
			return
		}
		var arguments any
		json.Unmarshal(args, &arguments)
		out = append(out, map[string]any{"id": id, "tool_ref": ref, "server_name": server, "status": status, "username": name, "round": round, "arguments": arguments, "result": result, "detail": detail, "created_at": created, "message_id": message.Int64})
	}
	if rows.Err() != nil {
		unavailable(w)
		return
	}
	respond(w, out)
}

// @api POST /api/mcp/servers/{id}/probe | 教师试调用本班工具并写审计 | teacher | MCPProbeInput | ToolUse
func probeMCPTool(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	s, e := getMCPServer(id, u.ClassID)
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&in) != nil || !mcpToolName.MatchString(in.Name) || len(in.Arguments) == 0 {
		readableFailure(w, 400, "需要有效name与arguments对象")
		return
	}
	call := executeMCP(r.Context(), u, 0, 1, toolCandidate{Ref: mcpRef{in.Name, s.Name}}, string(in.Arguments))
	respond(w, call)
}
