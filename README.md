# CampusClaw · 第 3–4 课

教师和学生按班级隔离的教研材料、知识库检索与引用问答系统。技术栈：React 18 + TypeScript + Vite + React Markdown/GFM，Go 标准库 `net/http` + MySQL 驱动 + bcrypt，MySQL 8.0 + ngram FULLTEXT，Qdrant，Nginx，Docker Compose。

## 启动

需要 Docker Engine 与 Compose v2；首次构建需访问 Docker Hub、Go modules 与 npm registry。

```bash
cp .env.example .env
# 将所有 REPLACE_WITH_* 改为不同的随机值；SESSION_SECRET 至少 32 字符
# 例如：openssl rand -hex 32（仅用于生成随机值，勿提交 .env）
docker compose config --quiet
docker compose up --build -d
curl http://localhost:8080/health
```

打开 http://localhost:8080 。初始账号：`teacher_a`、`student_a` 属于 A 班，`teacher_b`、`student_b` 属于 B 班；口令是 `.env` 对应的四个 `SEED_*_PASSWORD`。首次启动把它们以 bcrypt 哈希创建。之后调整 `.env` 的预置口令不会覆盖现有账号；要重置演示数据，请明确知晓这会删除所有材料，再运行 `docker compose down -v` 并重启。

只发布 web 的 8080 端口。原文件保存在 api 专用的 uploads 卷；没有 `/uploads` 静态目录。数据库只在内部 Compose 网络。生产环境需在外层终止 TLS，并将 `COOKIE_SECURE=true`，配置 HTTPS 后再对外开放。`.env` 不入库；勿将演示账户用于真实校园数据。

## 验证

```bash
# 设置与 .env 中相同的四个账号口令供验收脚本使用
export SEED_TEACHER_A_PASSWORD='…' SEED_STUDENT_A_PASSWORD='…'
export SEED_TEACHER_B_PASSWORD='…' SEED_STUDENT_B_PASSWORD='…'
python3 scripts/smoke.py
```

脚本覆盖匿名访问、四账号登录、教师上传、同班读取、跨班 404、学生禁止上传、文件鉴权、登出旧 Cookie、非法扩展名/编码/大小以及 `/health`。可在数据库容器中验证哈希和两表关系：

```bash
docker compose exec db sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -u"$MYSQL_USER" "$MYSQL_DATABASE" -e "SELECT username,password_hash,role,class_id FROM users; SELECT COUNT(*) FROM materials; SELECT COUNT(*) FROM knowledge_entries"'
```

数据库中断演练：`docker compose stop db` 后，`curl -i localhost:8080/health` 应返回 200；使用已登录 Cookie 请求 `/api/materials` 应返回 503。恢复：`docker compose start db`。本项目的 Go API 在数据库断开后会重连；首次启动须等待数据库就绪。

## 接口

| 方法与路径 | 权限 | 行为 |
|---|---|---|
| POST `/api/login` | 公开 | JSON `{username,password}`，会话 Cookie |
| POST `/api/logout` | 登录 | 删除会话 |
| GET `/api/me` | 登录 | 角色与班级 |
| GET `/api/materials?q=` | 登录 | 仅本班标题搜索 |
| GET `/api/materials/{id}` | 本班 | 详情与正文 |
| GET `/api/materials/{id}/file` | 本班 | 原文件 |
| POST `/api/materials` | 教师 | multipart `file`，只收 txt/md |
| GET `/health` | 公开 | 进程存活 |

单文件上限 2 MiB；非法扩展名 415，超限 413，非法编码/空文件 422。不存在和跨班详情或文件均为 404。登录失败和限流均返回一致的 401 文案。登录限流为进程内按账号与来源地址记录的简化实现，单实例演示足够；上线前应补持久、共享的限流机制。

## 设计与局限

- 以服务端会话支持即时登出和每请求更新角色/班级。JWT 携带旧角色直到过期，会给本迭代的即时失效验收增加额外状态管理。
- `/health` 不查询数据库，避免短暂数据库故障触发 API 反复重启；业务查询失败报告 503。
- 文本原文件必须经 Go 的班级校验，Nginx 直出会绕过鉴权。数据库端口不映射，减少直接暴露面。
- 磁盘与 MySQL 不能共享原子事务。实现先写私有文件，再在一个 SQL 事务中写两表；SQL 失败会清理文件。极端断电可留下孤儿文件，生产部署应增加定期对账与清理。
- 第 3 课版本只保存与按标题列出本班正文；第 4 课增加检索和问答，仍不包含管理员或校园 SSO。

完整可验收要求见 [spec.md](openspec/specs/auth-upload/spec.md)。

## 第 4 课：本班检索与引用问答

本项目新增 OpenSpec 活动变更 `openspec/changes/add-knowledge-retrieval/`（proposal、design、delta spec、tasks）。**保留第 3 课的 MySQL 与 uploads 卷**，直接在现有目录更新代码并运行 `docker compose up -d --build`；数据库启动时自动创建 `knowledge_chunks`，旧材料若尚无切片会按自动策略补建。不要执行 `down -v`。第四课仍只发布 web 的 8080 端口；Qdrant 和模拟模型网关只在 Compose 网络内。

- 上传 `.txt/.md` 后切分，正文仍保留在 MySQL；`knowledge_chunks` 保存切片及处理后文本的字符区间，MySQL ngram FULLTEXT 负责关键词检索，Qdrant `campusclaw_chunks` 保存余弦向量与 ID/班级元数据（不存正文）。
- `GET /api/search?q=...&mode=keyword|vector|hybrid` 返回本班切片、材料标题、摘录与来源链接。关键词只走 MySQL；向量查询 Qdrant 并回 MySQL 再核班级；混合使用 RRF k=60。空查询为 400，无命中是 200 加空列表。Qdrant 故障不影响关键词，向量/混合返回 503。
- `POST /api/ask` JSON `{ "question": "..." }`，先混合检索本班前 4 条，再请模型以 `[1]` 等编号撰写简短回答。无切片时直接返回 `资料中未找到相关内容`，citations 为空，不调用对话网关。
- 教师可调用 `POST /api/materials/{id}/reindex`，JSON `{"strategy":"hierarchy"}` 或 `{"strategy":"auto"}`。`custom` 还需 `max_chars` (100–2000)、`overlap_ratio` (0–0.5)、`separator` (`newline`/`blankline`/`period`)，可设 `remove_urls`、`remove_emails`、`collapse_whitespace` 布尔值。上传表单也接受相同字段。原始正文永不随预处理而改变。
- 上传时嵌入失败，材料仍保存，切片为 `failed`；修复网关后由教师重建。若更换真实嵌入模型或维度，先备份并清理 Qdrant 的 `campusclaw_chunks` 集合，然后对本班材料重建索引。旧向量维度不能与新维度混用。

### 本地模型网关与真实网关

默认 `gateway` 是**确定性模拟网关**：以字符二元组生成 64 维向量，从第一条证据摘录生成示例回答。它只用于离线复现和隔离验收，不能代表真实语义理解或大模型生成质量。要接入课程的 OpenAI 兼容网关，在 `.env` 添加 `GATEWAY_BASE_URL`（含 `/v1`）、`GATEWAY_API_KEY`、`EMBEDDING_MODEL`、`CHAT_MODEL`、`EMBEDDING_DIM`（必须与所选嵌入模型真实输出一致）。只允许后端读取密钥；若 HTTPS 网关使用自签证书，应将可信 CA 正确配置进镜像。不要把真实密钥写进仓库或浏览器。使用真实模型时仍要自行检查生成答案是否准确对应所引切片。

### 验收

先运行第 3 课的 `scripts/smoke.py`；再把四个 SEED 密码作为本机环境变量提供给 `scripts/smoke_retrieval.py`，运行 `python scripts/smoke_retrieval.py`。脚本上传两班互不相同的测试材料，检查三种模式的隔离、引用、无命中、学生禁止重建、教师重建和未登录 401。它会留存测试材料，可用演示班级专门验收。`go test ./...`（在 `api` 目录）覆盖纯切分逻辑。

Windows PowerShell 示例（四项填入你现有 `.env` 的对应口令；切勿把 `.env` 发布出去）：

```powershell
docker compose up -d --build
$env:SEED_TEACHER_A_PASSWORD='你的教师A口令'
$env:SEED_STUDENT_A_PASSWORD='你的学生A口令'
$env:SEED_TEACHER_B_PASSWORD='你的教师B口令'
$env:SEED_STUDENT_B_PASSWORD='你的学生B口令'
python .\scripts\smoke.py
python .\scripts\smoke_retrieval.py
docker compose ps
```

注意：MySQL 与 Qdrant 没有跨库事务。写向量失败时 SQL 的 `ready` 标志会阻断不完整切片；极端断电可能留下孤儿向量，教师重建可修复。生产部署还需要后台任务、对账清理、正式模型质量评测和访问审计。
