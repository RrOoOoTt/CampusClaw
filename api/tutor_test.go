package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestSkillFileValidation(t *testing.T) {
	s, e := parseSkill(defaultSkillFile)
	if e != nil || s.Tool != "knowledge_search" || !s.Enabled {
		t.Fatal(s, e)
	}
	for _, v := range []string{strings.Replace(defaultSkillFile, "名称：数学解题引导\n", "", 1), "名称：重复\n" + defaultSkillFile, strings.Replace(defaultSkillFile, "knowledge_search", "shell_exec", 1), strings.Replace(defaultSkillFile, "启用：是", "启用：true", 1), strings.Replace(defaultSkillFile, "\n\n", "\n", 1), defaultSkillFile + strings.Repeat("字", 8000)} {
		if _, e := parseSkill(v); e == nil {
			t.Fatal("invalid file accepted")
		}
	}
	if _, e := parseSkill(strings.ReplaceAll(defaultSkillFile, "\n", "\r\n")); e != nil {
		t.Fatal(e)
	}
}
func TestSkillRoutingSingleWinner(t *testing.T) {
	skills := []tutorSkill{{ID: 7, Conditions: "面积，面积", Enabled: true}, {ID: 2, Conditions: "面积", Enabled: true}, {ID: 1, Conditions: "面积，三角形", Enabled: false}}
	if s := chooseSkill("三角形面积", skills); s == nil || s.ID != 2 {
		t.Fatal("tie order, repeated keyword or disabled routing")
	}
	if chooseSkill("古诗", skills) != nil {
		t.Fatal("zero score routed")
	}
	skills = append(skills, tutorSkill{ID: 9, Conditions: "面积，三角形", Enabled: true})
	if s := chooseSkill("三角形面积", skills); s == nil || s.ID != 9 {
		t.Fatal("highest score")
	}
}
func TestTutorHistoryEvidenceAndFallback(t *testing.T) {
	h := []modelMessage{}
	for i := 0; i < 9; i++ {
		h = append(h, modelMessage{Role: "user", Content: strings.Repeat("字", 600)})
	}
	skill, _ := parseSkill(defaultSkillFile)
	m := tutorModelMessages("", &skill, h, "面积怎么求", []hit{{Title: "面积", Body: "底乘高除以二", Start: 1, End: 8}})
	if len(m) != 8 || len([]rune(m[1].Content)) != 500 {
		t.Fatal("history window not 6 x 500")
	}
	if !strings.Contains(m[0].Content, defaultAssistantPrompt) || !strings.Contains(m[0].Content, guidanceInstruction) {
		t.Fatal("fallback or skill")
	}
	if strings.Contains(m[0].Content, "底乘高除以二") || !strings.Contains(m[len(m)-1].Content, "底乘高除以二") {
		t.Fatal("evidence must be user data")
	}
	if m := tutorModelMessages("角色", nil, nil, "问题", nil); !strings.Contains(m[0].Content, "不得整段只说资料未找到") {
		t.Fatal("no-hit behavior")
	}
}
func TestCitationFilterAcrossDeltas(t *testing.T) {
	f := newCitationFilter(0)
	got := f.Push("正文[", false) + f.Push("1]无编号[99]结束", false) + f.Push("", true)
	if got != "正文无编号结束" {
		t.Fatal(got)
	}
	f = newCitationFilter(2)
	got = f.Push("根据[", false) + f.Push("2]，排除[3]。", false) + f.Push("", true)
	if got != "根据[2]，排除。" {
		t.Fatal(got)
	}
}
func TestOpenAPIAnnotationsAndSwagger(t *testing.T) {
	spec := openAPISpec()
	paths := spec["paths"].(map[string]any)
	for _, p := range []string{"/api/login", "/api/agents", "/api/skills", "/api/assistant/conversations/{id}/messages", "/api/materials/{id}"} {
		if paths[p] == nil {
			t.Fatal("undocumented", p)
		}
	}
	b, e := json.Marshal(spec)
	if e != nil || !strings.Contains(string(b), "BearerAuth") {
		t.Fatal(e)
	}
	for _, n := range []string{"index.html", "swagger-ui-bundle.js", "swagger-ui.css"} {
		if _, e := apiDocs.ReadFile("swagger/" + n); e != nil {
			t.Fatal(e)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	writeAgent(w, r, user{Role: "student"})
	if w.Code != 403 {
		t.Fatal("student agent write")
	}
	w = httptest.NewRecorder()
	writeSkill(w, r, user{Role: "student"})
	if w.Code != 403 {
		t.Fatal("student skill write")
	}
}

func TestOpenAPIMatchesBusinessRoutes(t *testing.T) {
	spec := openAPISpec()
	paths := spec["paths"].(map[string]any)
	src, e := apiDocs.ReadFile("main.go")
	if e != nil {
		t.Fatal(e)
	}
	re := regexp.MustCompile(`mux.HandleFunc\("(GET|POST|PUT|DELETE) (/api/[^" ]+)"`)
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		path, ok := paths[m[2]].(map[string]any)
		if !ok || path[strings.ToLower(m[1])] == nil {
			t.Fatal("route lacks annotation", m[1], m[2])
		}
	}
	raw, _ := json.Marshal(spec)
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	refs := regexp.MustCompile(`#/components/schemas/([A-Za-z0-9]+)`)
	for _, m := range refs.FindAllStringSubmatch(string(raw), -1) {
		if schemas[m[1]] == nil {
			t.Fatal("unresolved schema", m[1])
		}
	}
	mux := http.NewServeMux()
	mountOpenAPI(mux)
	for _, url := range []string{"/openapi.json", "/swagger/", "/swagger/swagger-ui.css"} {
		r := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(url, w.Code)
		}
	}
}
