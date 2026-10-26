package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssistantPromptAndSkill(t *testing.T) {
	evidence := []hit{{MaterialID: 7, Title: "几何方法", ChunkIndex: 2, Start: 20, End: 60, Body: "先列出已知条件"}}
	history := []modelMessage{{Role: "system", Content: "客户端越权设定"}, {Role: "user", Content: "上一题"}}
	guided := assistantModelMessages("你是几何老师", true, history, "如何分析？", evidence)
	if len(guided) != 3 || guided[1].Role != "user" {
		t.Fatal("untrusted role entered model history")
	}
	if !strings.Contains(guided[0].Content, "你是几何老师") || !strings.Contains(guided[0].Content, guidanceInstruction) {
		t.Fatal("prompt or skill missing")
	}
	if !strings.Contains(guided[2].Content, "几何方法") || !strings.Contains(guided[2].Content, "字符位置：20–60") {
		t.Fatal("source provenance missing")
	}
	direct := assistantModelMessages("你是几何老师", false, nil, "如何分析？", evidence)
	if !strings.Contains(direct[0].Content, directInstruction) || strings.Contains(direct[0].Content, guidanceInstruction) {
		t.Fatal("skill off ineffective")
	}
}
func TestAssistantFollowupAndCitations(t *testing.T) {
	query := assistantRetrievalQuery("再给个提示", []modelMessage{{Role: "user", Content: "三角形面积"}})
	if !strings.Contains(query, "三角形面积") {
		t.Fatal("follow-up lost topic")
	}
	evidence := []hit{{Title: "几何", Start: 1, End: 8}}
	answer := cleanAssistantCitations("参考 [99]", evidence)
	if strings.Contains(answer, "[99]") || !strings.Contains(answer, "[1]") {
		t.Fatal("invalid citation not removed")
	}
}
func TestUpstreamSSE(t *testing.T) {
	wire := ": heartbeat\r\n\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\r\n\r\ndata: {\"choices\":\n" + "data: [{\"delta\":{\"content\":\"同学\"}}]}\n\ndata: [DONE]\n\n"
	got := ""
	if err := consumeChatStream(strings.NewReader(wire), func(s string) error { got += s; return nil }); err != nil || got != "你好同学" {
		t.Fatalf("parse: %q %v", got, err)
	}
	if err := consumeChatStream(strings.NewReader("data: {\"choices\":[]}\n\n"), func(string) error { return nil }); err == nil {
		t.Fatal("incomplete upstream accepted")
	}
	stop := errors.New("client disconnected")
	if err := consumeChatStream(strings.NewReader(wire), func(string) error { return stop }); !errors.Is(err, stop) {
		t.Fatal("callback cancellation ignored")
	}
}
func TestMockAssistantStreaming(t *testing.T) {
	t.Setenv("CHAT_GATEWAY_BASE_URL", "")
	t.Setenv("CHAT_GATEWAY_API_KEY", "")
	server := httptest.NewServer(mockGatewayHandler(64))
	defer server.Close()
	oldURL, oldKey, oldModel := gatewayURL, gatewayKey, chatModel
	gatewayURL, gatewayKey, chatModel = server.URL+"/v1", "demo", "demo-chat"
	defer func() { gatewayURL, gatewayKey, chatModel = oldURL, oldKey, oldModel }()
	evidence := []hit{{Title: "本班练习", Body: "三角形面积由底和高计算", Start: 0, End: 15}}
	outputs := []string{}
	for _, enabled := range []bool{true, false} {
		output := ""
		count := 0
		messages := assistantModelMessages("耐心的几何助手", enabled, nil, "面积怎么求", evidence)
		if err := streamAssistantModel(context.Background(), messages, func(s string) error { output += s; count++; return nil }); err != nil {
			t.Fatal(err)
		}
		if count < 2 || !strings.Contains(output, "耐心的几何助手") || !strings.Contains(output, "本班练习") || !strings.Contains(output, "[1]") {
			t.Fatalf("missing streamed evidence/config: %q", output)
		}
		outputs = append(outputs, output)
	}
	if outputs[0] == outputs[1] || !strings.Contains(outputs[0], "不直接给出最终答案") || !strings.Contains(outputs[1], "直接回答") {
		t.Fatal("mock skill contrast missing")
	}
}
