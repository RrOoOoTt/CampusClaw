package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// @api POST /api/mcp/demo/setup | 连接同仓MCP演示服务并建立两个内置AI应用 | teacher | - | DemoSetup
func setupMCPDemo(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	mcpRegistryMutex.Lock()
	defer mcpRegistryMutex.Unlock()
	_, e := db.Exec("INSERT IGNORE INTO mcp_servers(class_id,name,base_url,token_env) VALUES(?,'campus-tools','http://mcp-demo:8090','MCP_AUTH_TOKEN')", u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	var id int64
	if e = db.QueryRow("SELECT id FROM mcp_servers WHERE class_id=? AND name='campus-tools'", u.ClassID).Scan(&id); e != nil {
		unavailable(w)
		return
	}
	server, e := getMCPServer(id, u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	server, e = connectMCP(r.Context(), server, u.ClassID)
	if e != nil {
		readableFailure(w, 400, e.Error())
		return
	}
	presets := []struct{ name, prompt, source string }{
		{"公式提示应用", "你是数学公式提示助手。先向工具请求公式或换算关系，再给检查问题，不写题目最终得数。", "名称：MCP解题引导\n适用条件：公式，三角形，面积，方程\n启用：是\n工具：formula_hint@campus-tools,unit_convert@campus-tools\n学科：数学\n\n需要公式或单位关系时先调用对应工具。综合提示指导学生选择公式，不直接给题目答案。"},
		{"单位换算应用", "你是单位换算学习助手。调用换算工具，讲清量纲和换算关系，让学生自己完成数值运算。", "名称：单位换算解题引导\n适用条件：单位，换算，厘米，千米，千克\n启用：是\n工具：unit_convert@campus-tools\n学科：数学\n\n调用单位换算工具取得关系，说明为何相乘或相除，不计算题目最终得数。"},
	}
	ids := []int64{}
	for _, p := range presets {
		skill, e := parseSkill(p.source)
		if e != nil {
			unavailable(w)
			return
		}
		if e = validateSkillTools(skill, u.ClassID); e != nil {
			readableFailure(w, 400, "演示工具清单不完整，未建立应用")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			unavailable(w)
			return
		}
		if _, e = tx.Exec("INSERT IGNORE INTO tutor_skills(class_id,name,`conditions`,subject,tool,body,enabled,source_text) VALUES(?,?,?,?,?,?,?,?)", u.ClassID, skill.Name, skill.Conditions, skill.Subject, skill.Tool, skill.Body, skill.Enabled, skill.Source); e != nil {
			tx.Rollback()
			unavailable(w)
			return
		}
		var sid int64
		if e = tx.QueryRow("SELECT id FROM tutor_skills WHERE class_id=? AND name=?", u.ClassID, skill.Name).Scan(&sid); e != nil {
			tx.Rollback()
			unavailable(w)
			return
		}
		var aid int64
		e = tx.QueryRow("SELECT id FROM tutor_agents WHERE class_id=? AND name=?", u.ClassID, p.name).Scan(&aid)
		if errors.Is(e, sql.ErrNoRows) {
			res, err := tx.Exec("INSERT INTO tutor_agents(class_id,name,system_prompt,enabled) VALUES(?,?,?,TRUE)", u.ClassID, p.name, p.prompt)
			if err != nil {
				tx.Rollback()
				unavailable(w)
				return
			}
			aid, e = res.LastInsertId()
			if e != nil {
				tx.Rollback()
				unavailable(w)
				return
			}
			if _, e = tx.Exec("INSERT INTO tutor_agent_skills(agent_id,skill_id) VALUES(?,?)", aid, sid); e != nil {
				tx.Rollback()
				unavailable(w)
				return
			}
			if _, e = tx.Exec("INSERT INTO tutor_agent_materials SELECT ?,id FROM materials WHERE class_id=? AND subject='数学'", aid, u.ClassID); e != nil {
				tx.Rollback()
				unavailable(w)
				return
			}
		} else if e != nil {
			tx.Rollback()
			unavailable(w)
			return
		}
		if e = tx.Commit(); e != nil {
			unavailable(w)
			return
		}
		ids = append(ids, aid)
	}
	respond(w, map[string]any{"server": server, "agent_ids": ids, "message": "两个内置应用已就绪；已有同名配置保持，后续新增资料请手动绑定"})
}
