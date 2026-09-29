package main

import (
    "crypto/sha256"
    "encoding/json"
    "log"
    "math"
    "net/http"
    "os"
    "strconv"
    "strings"
)

// Development-only deterministic OpenAI-compatible fixture, not a language model.
func runMockGateway() {
    dim := 64
    if v, err := strconv.Atoi(os.Getenv("EMBEDDING_DIM")); err == nil && v > 0 && v <= 4096 {
        dim = v
    }
    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    mux.HandleFunc("POST /v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
        var in struct { Input string `json:"input"` }
        if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
            http.Error(w, "bad input", http.StatusBadRequest)
            return
        }
        vector := make([]float64, dim)
        runes := []rune(strings.ToLower(in.Input))
        for i := 0; i+1 < len(runes); i++ {
            digest := sha256.Sum256([]byte(string(runes[i:i+2])))
            position := (int(digest[0])*256 + int(digest[1])) % dim
            vector[position]++
        }
        norm := 0.0
        for _, value := range vector { norm += value * value }
        if norm > 0 {
            norm = math.Sqrt(norm)
            for i := range vector { vector[i] /= norm }
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]any{
            "data": []any{map[string]any{"embedding": vector}},
        })
    })
    mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
        var in struct {
            Messages []struct { Content string `json:"content"` } `json:"messages"`
        }
        if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in); err != nil || len(in.Messages) == 0 {
            http.Error(w, "bad input", http.StatusBadRequest)
            return
        }
        prompt := in.Messages[len(in.Messages)-1].Content
        answer := "资料片段"
        if offset := strings.Index(prompt, "正文：\n"); offset >= 0 {
            part := prompt[offset+len("正文：\n"):]
            if end := strings.Index(part, "\n\n"); end >= 0 { part = part[:end] }
            characters := []rune(strings.TrimSpace(part))
            if len(characters) > 100 { characters = characters[:100] }
            if len(characters) > 0 { answer = string(characters) }
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]any{
            "choices": []any{map[string]any{
                "message": map[string]string{"content": "演示摘录：" + answer + " [1]"},
            }},
        })
    })
    log.Fatal(http.ListenAndServe(":8090", mux))
}
