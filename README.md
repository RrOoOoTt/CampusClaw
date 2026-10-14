# CampusClaw · 第 3–5 课

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

只发布 web 的 8080 端口。原文件保存在 api 专用的 uploads 卷；没有 `/uploads` 静态目录。数据库只在内部 Compose 网络。生产环境需在外层终止 TLS，配置 HTTPS 后再对外开放。`.env` 不入库；勿将演示账户用于真实校园数据。

## 验证

```bash
# 设置与 .env 中相同的四个账号口令供验收脚本使用
export SEED_TEACHER_A_PASSWORD='…' SEED_STUDENT_A_PASSWORD='…'
export SEED_TEACHER_B_PASSWORD='…' SEED_STUDENT_B_PASSWORD='…'
python3 scripts/smoke.py
```

脚本覆盖匿名访问、四账号登录、教师上传、同班读取、跨班 404、学生禁止上传、文件鉴权、登出旧 Token、非法扩展名/编码/大小以及 `/health`。可在数据库容器中验证哈希和两表关系：

```bash
docker compose exec db sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -u"$MYSQL_USER" "$MYSQL_DATABASE" -e "SELECT username,password_hash,role,class_id FROM users; SELECT COUNT(*) FROM materials; SELECT COUNT(*) FROM knowledge_entries"'
```

数据库中断演练：`docker compose stop db` 后，`curl -i localhost:8080/health` 应返回 200；使用有效 Bearer Token 请求 `/api/materials` 应返回 503。恢复：`docker compose start db`。本项目的 Go API 在数据库断开后会重连；首次启动须等待数据库就绪。

## 接口

| 方法与路径 | 权限 | 行为 |
|---|---|---|
| POST `/api/login` | 公开 | JSON `{username,password}`，Bearer Token |
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


## 作业补充：Authorization Bearer 登录认证

登录接口现在返回 JSON `{"access_token":"64位随机字符串","token_type":"Bearer","expires_in":43200}`。所有受保护接口仅接受 `Authorization: Bearer <access_token>`；旧 `cc_session` Cookie 不再作为认证依据。Token 仍是服务端随机会话，不是 JWT；数据库仅存 HMAC 摘要，12 小时过期，登出删除会话并立即失效。班级和角色仍由数据库决定。

前端将 Token 保存到当前标签页的 sessionStorage，刷新页面可继续使用；材料列表、上传、下载、检索、问答和重建索引统一添加 Authorization 请求头。关闭标签页后需重新登录。升级后旧 Cookie 登录态需重新登录。真实部署必须使用 HTTPS；前端 JavaScript 可以读取此 Token，因此继续禁止材料 HTML 执行并避免引入不可信脚本。

Chrome 验收：F12 → Network，登录后查看 `/api/login` 的 Response，再查看 `/api/me` 的 Request Headers，应出现 `Authorization: Bearer ...`。退出后再次使用原 Token 访问 `/api/me` 应返回 401；只有旧 Cookie、无请求头或错误 Token 也均返回 401。

PowerShell 接口验证（输入你自己的账号口令）：

```powershell
$body = @{username='teacher_a';password='你的教师A口令'} | ConvertTo-Json
$session = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/login' -ContentType 'application/json' -Body $body
$headers = @{Authorization="Bearer $($session.access_token)"}
Invoke-RestMethod -Uri 'http://localhost:8080/api/me' -Headers $headers
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/logout' -Headers $headers
# 此请求应返回 401，因为 Token 已注销
Invoke-RestMethod -Uri 'http://localhost:8080/api/me' -Headers $headers
```

补充 OpenSpec 变更见 `openspec/changes/use-bearer-auth/`。两个 smoke 脚本已改为 Bearer 请求头，仍使用原有四个 SEED 环境变量。


## 第 5 课：AI 解题助手

依据上传的《本课概述 · 第 5 课 · 提示词、AI 技能与流式会话》实现：学生提问先认证，再加载教师角色提示词与解题引导技能，混合检索本班材料，最后流式回答并保存个人会话。主界面现在是“AI 解题助手”；左侧“教研材料”仍可浏览，教师仍可上传和重建索引。

### 更新与使用（Windows）

把更新包 CampusClaw 文件夹里面的全部内容复制到原项目目录（不要形成 CampusClaw\CampusClaw），保留 `.env` 和现有数据卷。运行：

```powershell
docker compose up -d --build
docker compose ps
curl.exe -i http://localhost:8080/health
```

数据库启动时自动增加 3 张助手相关表。不要运行 `down -v`。Docker 构建会先运行 Go 单元测试；若构建失败，请检查完整输出中最早的编译/测试错误。前端更新后 Chrome 用 Ctrl+Shift+R 刷新并重新登录。

1. 教师登录 → AI 解题助手 → 展开“本班助手配置”，保存角色提示词，并选择是否启用解题引导；从下一轮回答生效。教师在“教研材料”上传与题目相关的 `.txt/.md`。
2. 学生登录 → 新会话 → 提问题目，可继续追问。开启引导时给分析思路与提示；关闭时允许依据材料直接回答。
3. 回答逐段显示，本轮材料依据含标题、切片、处理后字符位置。展开引用可看摘录或打开原材料。无本班命中时明确说明资料中未找到，没有虚构出处。
4. 刷新历史/重新登录可读取自己的会话；同班其他用户和教师也不能读取你的个人会话。停止生成后可刷新查看中断状态与已保存的部分内容。

### 真实模型与演示网关

默认仍为确定性演示 fixture，用于复现权限、技能切换、引用与 SSE。它会显示角色设定，给出模板式的引导或材料摘录，**不能解决真实题目，也不是模型质量证明**。

接入真实 AI 最简单的方式是在现有 `.env` 增加以下三项，然后重建/启动 API。这只替换第 5 课助手的对话模型，可以保持原有嵌入模型和 Qdrant 索引：

```dotenv
CHAT_GATEWAY_BASE_URL=https://你的OpenAI兼容网关地址/v1
CHAT_GATEWAY_API_KEY=你的真实密钥
CHAT_MODEL=网关提供的支持流式输出的模型名
```

网关必须支持 `POST /chat/completions` 的 `stream=true`，输出 `choices[].delta.content` 的 SSE，并用 `[DONE]` 结束。未配置 CHAT_GATEWAY_* 时沿用第 4 课的 GATEWAY_BASE_URL/GATEWAY_API_KEY。真实模型的解题正确性、引导是否避免最终答案、提示词是否生效都需人工检查。没有自动降级到演示回答来掩盖真实网关错误。

### 接口与截图验收

| 方法/路径 | 行为 |
|---|---|
| GET `/api/assistant/settings` | 本班提示词与技能开关 |
| PUT `/api/assistant/settings` | 仅教师保存 `{prompt,guidance_enabled}`，班级只取登录身份 |
| GET/POST `/api/assistant/conversations` | 本人的会话列表/创建 |
| GET `/api/assistant/conversations/{id}` | 本人本班历史与引用、配置快照 |
| POST `/api/assistant/conversations/{id}/messages` | `{question}`，Bearer 认证，SSE 回答 |

SSE 依次返回 `meta`（配置快照与证据）、多个 `delta`、`done`（已保存的完整内容）；生成错误用 `error`，不会伪称完成。Chrome F12 → Network 查看 `/messages` 的 Authorization、`Content-Type: text/event-stream` 与事件数据，可截图。教师配置、同题技能开/关的两次回答、无命中、历史刷新与引用原文可作为界面验收。

设置四个 SEED 密码环境变量后运行：

```powershell
python .\scripts\smoke.py
python .\scripts\smoke_retrieval.py
python .\scripts\smoke_assistant.py
```

最后一个脚本覆盖教师配置权限、班级隔离、配置快照从下一轮生效、个人会话隔离、引用和历史保存；默认还检查演示模型的引导/直接回答差异及 SSE 不被整条缓冲。真实模型验收前可设 `$env:ASSISTANT_DEMO_GATEWAY='0'` 跳过仅针对 fixture 文案/时间的断言，仍需人工检查回答质量。脚本会保留测试会话和材料，并在结束后恢复教师原配置。

活动 OpenSpec 变更：`openspec/changes/add-streaming-learning-assistant/`。本轮不实现作业提交、批改、错题讲解、异步队列、关怀对话或任意工具调用。

本地单实例演示限制：每会话最多 100 轮，模型上下文最近最多 12 条已完成消息，输出最多 6000 字。服务重启可释放 busy 并标记中断；突然断电时未落库的内存增量可能丢失。多 API 副本需要带租约的分布式锁，不能直接复用当前启动解锁方式。

前端 SSE 解析检查可在 `web` 目录运行 `npm run test:stream`（先 `npm ci`，Node 22）；测试逐字节拆分中文 UTF-8、CRLF 与事件边界。

快速演示材料：教师可上传 `examples/lesson5-math.md`，学生提问“三角形面积怎么求？底是8厘米，高是5厘米”，再追问“先给我第一步提示”。切换教师技能开关后，对同一题再次提问，比较两轮输出并展开引用。

## 资料和会话删除更新

保留当前 `.env`，解压并覆盖代码后执行 `docker compose up -d --build`。不要使用 `docker compose down -v`。新清理任务表在启动时自动建立，已有资料、会话和口令保持。

教师在“教研材料”打开资料详情，点击“删除资料”并确认；仅允许删除本班资料。资料正文和切片立即从数据库删除，文件和Qdrant向量同步清理；依赖不可用时自动重试。旧回答的引用快照保留，已删除资料的原文链接返回404。

会话列表每行提供“删除”，确认后清除该会话及全部消息，仅可删除自己的会话。生成中的会话不能删除；停止生成后刷新历史，等待状态释放再操作。

新增API：`DELETE /api/materials/{id}`，返回`deleted`和`cleanup_pending`；`DELETE /api/assistant/conversations/{id}`，返回`deleted`。均要求Bearer认证，401未登录、403非教师删除资料、404不存在或无访问权限、409会话正在生成。

DeepSeek官方接口默认关闭思考模式，输出上限2048，避免思考占满额度而没有回答正文。此参数只发送到api.deepseek.com，保留现有CHAT_*配置。

验证：前端`npm run build`和`npm run test:stream`；设置SEED_TEACHER_A_PASSWORD、SEED_TEACHER_B_PASSWORD、SEED_STUDENT_A_PASSWORD后运行`python scripts/smoke_deletion.py`（只删除脚本创建的测试资料和会话）。Go测试会在Docker构建中执行。交付时尚未实测Go编译及运行中的数据库/Qdrant删除链路。
