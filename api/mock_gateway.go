package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Development-only deterministic OpenAI-compatible fixture, not a language model.
func runMockGateway() {
	dim := 64
	if v, err := strconv.Atoi(os.Getenv("EMBEDDING_DIM")); err == nil && v > 0 && v <= 4096 {
		dim = v
	}
	log.Fatal(http.ListenAndServe(":8090", mockGatewayHandler(dim)))
}

func mockGatewayHandler(dim int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			http.Error(w, "bad input", http.StatusBadRequest)
			return
		}
		vector := make([]float64, dim)
		runes := []rune(strings.ToLower(in.Input))
		for i := 0; i+1 < len(runes); i++ {
			digest := sha256.Sum256([]byte(string(runes[i : i+2])))
			position := (int(digest[0])*256 + int(digest[1])) % dim
			vector[position]++
		}
		norm := 0.0
		for _, value := range vector {
			norm += value * value
		}
		if norm > 0 {
			norm = math.Sqrt(norm)
			for i := range vector {
				vector[i] /= norm
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{map[string]any{"embedding": vector}},
		})
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Messages []modelMessage `json:"messages"`
			Stream   bool           `json:"stream"`
			Tools    []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&in); err != nil || len(in.Messages) == 0 {
			http.Error(w, "bad input", http.StatusBadRequest)
			return
		}
		prompt := in.Messages[len(in.Messages)-1].Content
		for _, m := range in.Messages {
			if m.Role == "user" && strings.HasPrefix(m.Content, "问题：") {
				prompt = m.Content
			}
		}
		if !in.Stream && len(in.Tools) > 0 {
			called := map[string]bool{}
			for _, m := range in.Messages {
				for _, c := range m.ToolCalls {
					called[c.Function.Name] = true
				}
			}
			name := ""
			args := map[string]string{}
			for _, t := range in.Tools {
				if strings.Contains(t.Function.Name, "unit_convert") && !called[t.Function.Name] && (strings.Contains(prompt, "换算") || strings.Contains(prompt, "厘米") || strings.Contains(prompt, "米")) {
					name = t.Function.Name
					args = map[string]string{"from": "m", "to": "cm"}
					if strings.Contains(prompt, "千克") && strings.Contains(prompt, "厘米") {
						args = map[string]string{"from": "kg", "to": "cm"}
					}
					break
				}
			}
			if name == "" {
				for _, t := range in.Tools {
					if strings.Contains(t.Function.Name, "formula_hint") && !called[t.Function.Name] && (strings.Contains(prompt, "三角形") || strings.Contains(prompt, "公式") || strings.Contains(prompt, "面积")) {
						name = t.Function.Name
						args = map[string]string{"topic": "triangle_area"}
						break
					}
				}
			}
			m := modelMessage{Role: "assistant", Content: "工具信息已足够，接下来按已有资料和工具结果作答。"}
			finish := "stop"
			if name != "" {
				raw, _ := json.Marshal(args)
				c := llmToolCall{ID: "demo_" + strconv.Itoa(len(called)+1), Type: "function"}
				c.Function.Name = name
				c.Function.Arguments = string(raw)
				m.Content = ""
				m.ToolCalls = []llmToolCall{c}
				finish = "tool_calls"
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": m, "finish_reason": finish}}})
			return
		}
		answer := "资料片段"
		if offset := strings.Index(prompt, "正文：\n"); offset >= 0 {
			part := prompt[offset+len("正文：\n"):]
			if end := strings.Index(part, "\n\n"); end >= 0 {
				part = part[:end]
			}
			characters := []rune(strings.TrimSpace(part))
			if len(characters) > 100 {
				characters = characters[:100]
			}
			if len(characters) > 0 {
				answer = string(characters)
			}
		}
		responseText := "演示摘录：" + answer + " [1]"
		if in.Stream {
			role := defaultAssistantPrompt
			system := ""
			if len(in.Messages) > 0 {
				system = in.Messages[0].Content
			}
			if begin := strings.Index(system, "教师角色设定：\n"); begin >= 0 {
				part := system[begin+len("教师角色设定：\n"):]
				if stop := strings.Index(part, "\n角色设定结束"); stop >= 0 {
					role = limitCharacters(part[:stop], 80)
				}
			}
			location := "本班材料 [1]"
			if begin := strings.Index(prompt, "[1] 材料："); begin >= 0 {
				part := prompt[begin:]
				if stop := strings.Index(part, "；正文："); stop >= 0 {
					location = part[:stop]
				}
			}
			if strings.Contains(system, "【解题引导技能：开启】") {
				responseText = "演示助手 · 角色设定：" + role + "\n\n解题引导已开启。先列出题目的已知条件，再对照 " + location + " 中的方法选择解题步骤。你可以先写出第一步推导，我会继续给提示；这一轮不直接给出最终答案。"
			} else {
				responseText = "演示助手 · 角色设定：" + role + "\n\n解题引导已关闭，直接回答。依据 " + location + "，相关内容为：" + answer + "。"
			}
			if !strings.Contains(prompt, "正文：\n") {
				if strings.Contains(system, "【解题引导技能：开启】") {
					responseText = "演示助手 · 角色设定：" + role + "\n本轮没有本班材料依据。先列出已知条件，说明你已经尝试的步骤，我会给下一步提示，不直接给最终答案。"
				} else {
					responseText = "演示助手 · 角色设定：" + role + "\n本轮没有本班材料依据。可以先明确问题的目标、已知条件与需要解释的概念，再分步骤分析；这段演示模板不是来自知识库的引用。"
				}
			}
			summaries := []string{}
			for _, m := range in.Messages {
				if m.Role == "tool" {
					start := strings.Index(m.Content, "{")
					if start >= 0 {
						var use toolUse
						if json.Unmarshal([]byte(m.Content[start:]), &use) == nil && use.Status == "ok" {
							summaries = append(summaries, use.Result)
						}
					}
				}
			}
			if len(summaries) > 0 {
				responseText += "\n\n已使用工具返回的提示：\n" + limitCharacters(strings.Join(summaries, "\n"), 1000)
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("X-Accel-Buffering", "no")
			characters := []rune(responseText)
			for start := 0; start < len(characters); start += 10 {
				end := start + 10
				if end > len(characters) {
					end = len(characters)
				}
				payload := map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": string(characters[start:end])}}}}
				data, _ := json.Marshal(payload)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
				if http.NewResponseController(w).Flush() != nil {
					return
				}
				select {
				case <-r.Context().Done():
					return
				case <-time.After(30 * time.Millisecond):
				}
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
			http.NewResponseController(w).Flush()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]string{"content": responseText},
			}},
		})
	})
	return mux
}
