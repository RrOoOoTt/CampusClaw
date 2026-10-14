package main

import (
 "encoding/hex"
 "net/http"
 "strings"
)

// Only the Authorization header authenticates a protected API request.
func requestToken(r *http.Request) (string, bool) {
 values := r.Header.Values("Authorization")
 if len(values) != 1 { return "", false }
 parts := strings.Fields(values[0])
 if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) != 64 { return "", false }
 if _, err := hex.DecodeString(parts[1]); err != nil { return "", false }
 return parts[1], true
}
