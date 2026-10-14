# Verification

- 2026-09-30: TypeScript/Vite production build passed (280 modules).
- Python smoke and retrieval smoke syntax passed.
- Frontend request helper checks passed: Bearer header added, login excluded, JSON header preserved, multipart boundary left to browser, credentials omitted, 401 clears stored Token.
- Added Go unit cases for missing/legacy-cookie-only credentials, valid Bearer, malformed scheme/token, and duplicate Authorization headers.
- Go toolchain and Docker are unavailable here. New Go unit tests and live Compose integration smoke have NOT run. Execute both scripts and `go test ./...` on a Docker/Go-capable machine before archiving.
