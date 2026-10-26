package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type tutorAgent struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Prompt      string  `json:"prompt"`
	Enabled     bool    `json:"enabled"`
	SkillIDs    []int64 `json:"skill_ids"`
	MaterialIDs []int64 `json:"material_ids"`
}
type tutorSkill struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Conditions string `json:"conditions"`
	Subject    string `json:"subject"`
	Tool       string `json:"tool"`
	Body       string `json:"body"`
	Enabled    bool   `json:"enabled"`
	Source     string `json:"source"`
}

const defaultSkillFile = "名称：数学解题引导\n适用条件：数学，三角形，面积，方程，几何，计算\n启用：是\n工具：knowledge_search\n学科：数学\n\n先核对已知条件，给一条下一步提示并提出检查问题。引用命中的材料，不写最终答案、选项、得数或完整解题过程。"

// The teaching format is a small field header plus a blank line and body; it is not general SKILL.md frontmatter.
func parseSkill(source string) (tutorSkill, error) {
	var s tutorSkill
	if !utf8.ValidString(source) || len([]rune(source)) > 8000 {
		return s, errors.New("技能文件须为UTF-8且不超过8000字")
	}
	source = strings.ReplaceAll(strings.TrimPrefix(source, "\ufeff"), "\r\n", "\n")
	parts := strings.SplitN(source, "\n\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return s, errors.New("字段后须空一行，再填写技能正文")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(parts[0], "\n") {
		at := strings.IndexAny(line, ":：")
		if at < 0 {
			return s, errors.New("字段格式应为 名称：内容")
		}
		key := strings.TrimSpace(line[:at])
		_, size := utf8.DecodeRuneInString(line[at:])
		value := strings.TrimSpace(line[at+size:])
		switch key {
		case "名称", "适用条件", "启用", "工具", "学科":
		default:
			return s, fmt.Errorf("未知字段：%s", key)
		}
		if _, ok := fields[key]; ok {
			return s, fmt.Errorf("重复字段：%s", key)
		}
		fields[key] = value
	}
	for _, key := range []string{"名称", "适用条件", "启用"} {
		if fields[key] == "" {
			return s, fmt.Errorf("缺少字段：%s", key)
		}
	}
	if len([]rune(fields["名称"])) > 64 || len([]rune(fields["适用条件"])) > 1000 {
		return s, errors.New("名称最多64字，适用条件最多1000字")
	}
	if fields["启用"] != "是" && fields["启用"] != "否" {
		return s, errors.New("启用只能填是或否")
	}
	tool := fields["工具"]
	if tool == "" {
		tool = "knowledge_search"
	}
	if len(tool) > 512 {
		return s, errors.New("工具引用超过512字节")
	}
	if _, e := parseToolRefs(tool); e != nil {
		return s, e
	}
	subject := fields["学科"]
	if subject != "" && !validSubject(subject) {
		return s, errors.New("学科仅支持数学、语文、英语、通用")
	}
	s = tutorSkill{Name: fields["名称"], Conditions: fields["适用条件"], Subject: subject, Tool: tool, Body: strings.TrimSpace(parts[1]), Enabled: fields["启用"] == "是", Source: source}
	return s, nil
}
func validSubject(s string) bool {
	return s == "通用" || s == "数学" || s == "语文" || s == "英语"
}
func inferSubject(q string) string {
	for _, group := range []struct {
		subject string
		words   []string
	}{{"数学", []string{"数学", "三角形", "面积", "方程", "几何", "计算", "单位", "换算", "厘米"}}, {"语文", []string{"语文", "古诗", "诗词", "作文", "文言"}}, {"英语", []string{"英语", "英文", "grammar", "english", "单词"}}} {
		for _, word := range group.words {
			if strings.Contains(strings.ToLower(q), word) {
				return group.subject
			}
		}
	}
	return ""
}
func chooseSkill(question string, skills []tutorSkill) *tutorSkill {
	bestScore := 0
	var best *tutorSkill
	q := strings.ToLower(question)
	for _, s := range skills {
		if !s.Enabled {
			continue
		}
		score := 0
		seen := map[string]bool{}
		words := strings.FieldsFunc(strings.ToLower(s.Conditions), func(r rune) bool { return strings.ContainsRune(",，;；、\n \t", r) })
		for _, word := range words {
			if word != "" && !seen[word] && strings.Contains(q, word) {
				score++
				seen[word] = true
			}
		}
		if score > bestScore || (score > 0 && score == bestScore && (best == nil || s.ID < best.ID)) {
			copy := s
			best = &copy
			bestScore = score
		}
	}
	return best
}
func addColumn(table, column, definition string) error {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?", table, column).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		_, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
		return err
	}
	return nil
}
func initTutors() error {
	if err := addColumn("assistant_conversations", "agent_id", "BIGINT NULL"); err != nil {
		return err
	}
	if err := addColumn("materials", "subject", "VARCHAR(32) NOT NULL DEFAULT '通用'"); err != nil {
		return err
	}
	// Seed only classes without agents; migrate legacy private histories to a default agent once.
	rows, err := db.Query("SELECT id FROM classes")
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		var agentID int64
		err = tx.QueryRow("SELECT id FROM tutor_agents WHERE class_id=? ORDER BY id LIMIT 1", id).Scan(&agentID)
		if errors.Is(err, sql.ErrNoRows) {
			settings, e := loadAssistantSettings(id)
			if e != nil {
				tx.Rollback()
				return e
			}
			res, e := tx.Exec("INSERT INTO tutor_agents(class_id,name,system_prompt,enabled) VALUES(?,'本班学习助手',?,TRUE)", id, settings.Prompt)
			if e != nil {
				tx.Rollback()
				return e
			}
			agentID, e = res.LastInsertId()
			if e != nil {
				tx.Rollback()
				return e
			}
			skill, _ := parseSkill(defaultSkillFile)
			if !settings.GuidanceEnabled {
				skill.Source = strings.Replace(skill.Source, "启用：是", "启用：否", 1)
			}
			res, e = tx.Exec("INSERT INTO tutor_skills(class_id,name,`conditions`,subject,tool,body,enabled,source_text) VALUES(?,?,?,?,?,?,?,?)", id, skill.Name, skill.Conditions, skill.Subject, skill.Tool, skill.Body, settings.GuidanceEnabled, skill.Source)
			if e != nil {
				tx.Rollback()
				return e
			}
			sid, e := res.LastInsertId()
			if e != nil {
				tx.Rollback()
				return e
			}
			if _, e = tx.Exec("INSERT INTO tutor_agent_skills VALUES(?,?)", agentID, sid); e != nil {
				tx.Rollback()
				return e
			}
			if _, e = tx.Exec("INSERT INTO tutor_agent_materials SELECT ?,id FROM materials WHERE class_id=?", agentID, id); e != nil {
				tx.Rollback()
				return e
			}
		} else if err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec("UPDATE assistant_conversations SET agent_id=? WHERE class_id=? AND agent_id IS NULL", agentID, id); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
func routeID(r *http.Request) (int64, error) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		return 0, sql.ErrNoRows
	}
	return id, nil
}
func readableFailure(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func databaseWriteFailure(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "Duplicate entry") {
		readableFailure(w, 400, "本班已有同名记录")
		return
	}
	unavailable(w)
}
func getAgent(id, classID int64, requireEnabled bool) (tutorAgent, error) {
	a := tutorAgent{SkillIDs: []int64{}, MaterialIDs: []int64{}}
	query := "SELECT id,name,system_prompt,enabled FROM tutor_agents WHERE id=? AND class_id=?"
	if requireEnabled {
		query += " AND enabled=TRUE"
	}
	err := db.QueryRow(query, id, classID).Scan(&a.ID, &a.Name, &a.Prompt, &a.Enabled)
	if err != nil {
		return a, err
	}
	for _, b := range []struct {
		query string
		out   *[]int64
	}{{"SELECT s.id FROM tutor_agent_skills b JOIN tutor_skills s ON s.id=b.skill_id WHERE b.agent_id=? AND s.class_id=? ORDER BY s.id", &a.SkillIDs}, {"SELECT m.id FROM tutor_agent_materials b JOIN materials m ON m.id=b.material_id WHERE b.agent_id=? AND m.class_id=? ORDER BY m.id", &a.MaterialIDs}} {
		rows, e := db.Query(b.query, id, classID)
		if e != nil {
			return a, e
		}
		for rows.Next() {
			var v int64
			if e = rows.Scan(&v); e != nil {
				rows.Close()
				return a, e
			}
			*b.out = append(*b.out, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return a, e
		}
	}
	return a, nil
}

// @api GET /api/agents | 本班智能体列表 | auth | - | Agent[]
func listAgents(w http.ResponseWriter, r *http.Request, u user) {
	q := "SELECT id FROM tutor_agents WHERE class_id=?"
	if u.Role != "teacher" {
		q += " AND enabled=TRUE"
	}
	q += " ORDER BY id"
	rows, e := db.Query(q, u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			unavailable(w)
			return
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		unavailable(w)
		return
	}
	out := []tutorAgent{}
	for _, id := range ids {
		a, e := getAgent(id, u.ClassID, u.Role != "teacher")
		if e != nil {
			unavailable(w)
			return
		}
		out = append(out, a)
	}
	respond(w, out)
}

// @api GET /api/agents/{id} | 读取本班智能体 | auth | - | Agent
func readAgent(w http.ResponseWriter, r *http.Request, u user) {
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	a, e := getAgent(id, u.ClassID, u.Role != "teacher")
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	respond(w, a)
}

// @api POST /api/agents | 创建智能体及绑定 | teacher | AgentInput | Agent
// @api PUT /api/agents/{id} | 修改启停智能体及绑定 | teacher | AgentInput | Agent
func writeAgent(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	var in struct {
		Name        string  `json:"name"`
		Prompt      string  `json:"prompt"`
		Enabled     *bool   `json:"enabled"`
		SkillIDs    []int64 `json:"skill_ids"`
		MaterialIDs []int64 `json:"material_ids"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Enabled == nil || !utf8.ValidString(in.Name+in.Prompt) || len([]rune(strings.TrimSpace(in.Name))) < 1 || len([]rune(in.Name)) > 64 || len([]rune(in.Prompt)) > 2000 || len(in.SkillIDs) > 100 || len(in.MaterialIDs) > 100 {
		readableFailure(w, 400, "名称1–64字、提示词最多2000字；enabled必填；每类绑定最多100条")
		return
	}
	id := int64(0)
	if r.Method == "PUT" {
		var e error
		id, e = routeID(r)
		if e != nil {
			failure(w, 404)
			return
		}
	}
	tx, e := db.Begin()
	if e != nil {
		unavailable(w)
		return
	}
	defer tx.Rollback()
	if id > 0 {
		var found int
		e = tx.QueryRow("SELECT 1 FROM tutor_agents WHERE id=? AND class_id=? FOR UPDATE", id, u.ClassID).Scan(&found)
		if errors.Is(e, sql.ErrNoRows) {
			failure(w, 404)
			return
		}
		if e != nil {
			unavailable(w)
			return
		}
	}
	for _, binding := range []struct {
		table string
		ids   []int64
	}{{"tutor_skills", in.SkillIDs}, {"materials", in.MaterialIDs}} {
		seen := map[int64]bool{}
		for _, bid := range binding.ids {
			if seen[bid] {
				readableFailure(w, 400, "绑定编号重复")
				return
			}
			seen[bid] = true
			var found int
			e = tx.QueryRow("SELECT 1 FROM "+binding.table+" WHERE id=? AND class_id=? FOR UPDATE", bid, u.ClassID).Scan(&found)
			if errors.Is(e, sql.ErrNoRows) {
				failure(w, 404)
				return
			}
			if e != nil {
				unavailable(w)
				return
			}
		}
	}
	if id == 0 {
		res, err := tx.Exec("INSERT INTO tutor_agents(class_id,name,system_prompt,enabled) VALUES(?,?,?,?)", u.ClassID, strings.TrimSpace(in.Name), in.Prompt, *in.Enabled)
		if err != nil {
			databaseWriteFailure(w, err)
			return
		}
		id, e = res.LastInsertId()
	} else {
		_, e = tx.Exec("UPDATE tutor_agents SET name=?,system_prompt=?,enabled=? WHERE id=?", strings.TrimSpace(in.Name), in.Prompt, *in.Enabled, id)
	}
	if e != nil {
		databaseWriteFailure(w, e)
		return
	}
	for _, binding := range []struct {
		table, column string
		ids           []int64
	}{{"tutor_agent_skills", "skill_id", in.SkillIDs}, {"tutor_agent_materials", "material_id", in.MaterialIDs}} {
		if _, e = tx.Exec("DELETE FROM "+binding.table+" WHERE agent_id=?", id); e != nil {
			unavailable(w)
			return
		}
		for _, bid := range binding.ids {
			if _, e = tx.Exec("INSERT INTO "+binding.table+"(agent_id,"+binding.column+") VALUES(?,?)", id, bid); e != nil {
				unavailable(w)
				return
			}
		}
	}
	if e = tx.Commit(); e != nil {
		unavailable(w)
		return
	}
	a, e := getAgent(id, u.ClassID, false)
	if e != nil {
		unavailable(w)
		return
	}
	if r.Method == "POST" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(201)
	}
	respond(w, a)
}
func getSkill(id, classID int64) (tutorSkill, error) {
	var s tutorSkill
	e := db.QueryRow("SELECT id,name,`conditions`,subject,tool,body,enabled,source_text FROM tutor_skills WHERE id=? AND class_id=?", id, classID).Scan(&s.ID, &s.Name, &s.Conditions, &s.Subject, &s.Tool, &s.Body, &s.Enabled, &s.Source)
	return s, e
}

// @api GET /api/skills | 本班技能列表 | auth | - | Skill[]
func listSkills(w http.ResponseWriter, r *http.Request, u user) {
	rows, e := db.Query("SELECT id FROM tutor_skills WHERE class_id=? ORDER BY id", u.ClassID)
	if e != nil {
		unavailable(w)
		return
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			unavailable(w)
			return
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		unavailable(w)
		return
	}
	out := []tutorSkill{}
	for _, id := range ids {
		s, e := getSkill(id, u.ClassID)
		if e != nil {
			unavailable(w)
			return
		}
		out = append(out, s)
	}
	respond(w, out)
}

// @api GET /api/skills/{id} | 读取本班技能 | auth | - | Skill
func readSkill(w http.ResponseWriter, r *http.Request, u user) {
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	s, e := getSkill(id, u.ClassID)
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

// @api POST /api/skills | 解析并创建技能文件 | teacher | SkillInput | Skill
// @api PUT /api/skills/{id} | 解析并修改技能文件 | teacher | SkillInput | Skill
func writeSkill(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	var in struct {
		Source string `json:"source"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<10)
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		readableFailure(w, 400, "需要source文本字段")
		return
	}
	s, e := parseSkill(in.Source)
	if e != nil {
		readableFailure(w, 400, e.Error())
		return
	}
	if e = validateSkillTools(s, u.ClassID); errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	} else if e != nil {
		readableFailure(w, 400, e.Error())
		return
	}
	id := int64(0)
	if r.Method == "PUT" {
		id, e = routeID(r)
		if e != nil {
			failure(w, 404)
			return
		}
		if _, e = getSkill(id, u.ClassID); errors.Is(e, sql.ErrNoRows) {
			failure(w, 404)
			return
		} else if e != nil {
			unavailable(w)
			return
		}
	}
	if id == 0 {
		res, err := db.Exec("INSERT INTO tutor_skills(class_id,name,`conditions`,subject,tool,body,enabled,source_text) VALUES(?,?,?,?,?,?,?,?)", u.ClassID, s.Name, s.Conditions, s.Subject, s.Tool, s.Body, s.Enabled, s.Source)
		if err != nil {
			databaseWriteFailure(w, err)
			return
		}
		id, e = res.LastInsertId()
	} else {
		var res sql.Result
		res, e = db.Exec("UPDATE tutor_skills SET name=?,`conditions`=?,subject=?,tool=?,body=?,enabled=?,source_text=? WHERE id=? AND class_id=?", s.Name, s.Conditions, s.Subject, s.Tool, s.Body, s.Enabled, s.Source, id, u.ClassID)
		if e == nil {
			var n int64
			n, e = res.RowsAffected()
			_ = n
		}
	}
	if e != nil {
		databaseWriteFailure(w, e)
		return
	}
	s.ID = id
	if r.Method == "POST" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(201)
	}
	respond(w, s)
}

// @api DELETE /api/skills/{id} | 删除本班技能及绑定 | teacher | - | Deleted
func removeSkill(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	res, e := db.Exec("DELETE FROM tutor_skills WHERE id=? AND class_id=?", id, u.ClassID)
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
func boundSkills(agentID, classID int64) ([]tutorSkill, error) {
	rows, e := db.Query("SELECT s.id,s.name,s.`conditions`,s.subject,s.tool,s.body,s.enabled,s.source_text FROM tutor_agent_skills b JOIN tutor_skills s ON s.id=b.skill_id WHERE b.agent_id=? AND s.class_id=? AND s.enabled=TRUE ORDER BY s.id", agentID, classID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []tutorSkill{}
	for rows.Next() {
		var s tutorSkill
		if e = rows.Scan(&s.ID, &s.Name, &s.Conditions, &s.Subject, &s.Tool, &s.Body, &s.Enabled, &s.Source); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// @api PUT /api/materials/{id}/subject | 修改本班资料学科 | teacher | SubjectInput | Deleted
func setMaterialSubject(w http.ResponseWriter, r *http.Request, u user) {
	if u.Role != "teacher" {
		failure(w, 403)
		return
	}
	id, e := routeID(r)
	if e != nil {
		failure(w, 404)
		return
	}
	var in struct {
		Subject string `json:"subject"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if json.NewDecoder(r.Body).Decode(&in) != nil || !validSubject(in.Subject) {
		readableFailure(w, 400, "学科仅支持数学、语文、英语、通用")
		return
	}
	var exists int
	e = db.QueryRow("SELECT 1 FROM materials WHERE id=? AND class_id=?", id, u.ClassID).Scan(&exists)
	if errors.Is(e, sql.ErrNoRows) {
		failure(w, 404)
		return
	}
	if e != nil {
		unavailable(w)
		return
	}
	if _, e = db.Exec("UPDATE materials SET subject=? WHERE id=? AND class_id=?", in.Subject, id, u.ClassID); e != nil {
		unavailable(w)
		return
	}
	respond(w, map[string]bool{"updated": true})
}
