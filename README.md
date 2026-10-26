# CampusClaw（第3–6课增量实现）

React18 + TypeScript + Vite、Go net/http、MySQL8.0.40、Qdrant1.12.6、Nginx、Docker Compose。登录采用随机Bearer令牌和数据库会话（非JWT）。

## 更新部署

解压后将压缩包内 `CampusClaw` 的文件覆盖到原项目根目录，不要多嵌套一层目录。保留自己的 `.env`，不要覆盖种子口令、SESSION_SECRET或模型密钥。

```powershell
docker compose up -d --build
docker compose ps -a
```

浏览器访问 `http://localhost:8080`，按Ctrl+Shift+R刷新。停止使用 `docker compose down`，不要加 `-v`，以保留数据库、上传和向量卷。

首次运行：复制 `.env.example` 为 `.env` 并设置所有口令与随机字符串，再执行上述启动命令。已有数据库的种子账号口令不会被环境变量覆盖。

只发布web的8080端口，API/数据库/Qdrant在容器网络内。公网部署应另配HTTPS。

## 第六课新增：MCP与工具

在原有 `.env` 追加以下配置，保留原数据库口令、SESSION_SECRET和模型配置：

```dotenv
MCP_AUTH_TOKEN=<32字符以上随机字符串>
MCP_ALLOWED_HOSTS=mcp-demo
MCP_MAX_ROUNDS=3
MCP_MAX_CALLS=4
```

PowerShell执行 `[guid]::NewGuid().ToString("N")` 可生成32字符随机值，将结果填入MCP_AUTH_TOKEN，尖括号不保留。Compose将同一值提供给API与MCP服务；不要填用户登录Token或DeepSeek密钥。

```powershell
docker compose up -d --build
docker compose ps -a
docker compose exec -T mcp-demo /mcp-server selfcheck
```

1. 教师登录 → AI解题助手 → MCP管理，点击“连接演示MCP并安装两个内置应用”。等待工具清单出现formula_hint、unit_convert。可重复安装，已有同名应用配置不覆盖。
2. 两个内置应用为“公式提示应用”“单位换算应用”；安装时绑定已有数学资料。后续上传的资料需在教师管理中显式绑定。
3. 学生选择“公式提示应用”新建会话，输入“请先调用单位换算工具得到米到厘米的关系，再调用公式提示工具，给三角形面积的分析提示，不要计算最终得数。”
4. 页面先显示工具执行状态，再显示回答；教师可查本班调用审计。展开“模型请求与返回”可查看自己的会话参数与解析后响应。模型是否选择工具仍由模型决定，需要支持OpenAI兼容tools/tool_calls协议。
5. 默认模拟gateway也能演示多轮工具流程，但不代表真实模型效果。已有DeepSeek配置保留即可；如果该模型不支持tools，工具决策降级，仍尝试普通问答。

独立服务源码在 `mcp-server/`，容器内POST /mcp、GET /health，默认不发布到宿主机。教师手工登记：名称campus-tools、Base URL `http://mcp-demo:8090`、凭据环境变量名MCP_AUTH_TOKEN（填名称，非密钥）。其他服务器必须同时加入API的MCP_ALLOWED_HOSTS，并将MCP_TOKEN_前缀的凭据变量显式传入API容器。此实现固定课程协议2025-03-26，支持JSON响应的HTTP子集，不宣称兼容所有MCP传输/授权方式。

完整实现、五项阶段验收问题和同伴验证步骤见 [第六课验收](docs/week06-verification.md)、[阶段验收说明](docs/stage-acceptance-mcp.md)。

## 第五课使用流程

1. 教师登录，进入“教研材料”。上传 `.txt`/`.md`（UTF-8、非空、最多2MiB），选择学科，例如“数学”。旧材料可在详情里修改学科，不需要重建向量。
2. 进入“AI解题助手”，展开“教师管理”。维护至少一个“解题引导”技能；可粘贴或读入 `examples/lesson5-guidance.skill.md`。
3. 创建智能体，填写班内唯一名称、角色提示词，勾选本班技能与材料并保存。角色提示词留空时回退默认角色。
4. 学生从本班已启用的智能体中选择一个，发送2–500字问题。会话锁定该智能体，换智能体需新开会话。
5. 本轮只路由已绑定且已启用的技能，至多一个；按条件关键词计分，同分取较小技能ID。选中名称包含“解题引导”的技能时只给提示，不给最终答案。
6. 检索只限该智能体绑定的本班材料，识别数学/语文/英语学科后严格匹配学科。引用最多4条，随meta先返回，正文通过delta逐段显示。
7. 停用或解绑技能、修改角色与材料绑定，下轮立即生效。停用智能体禁止新建会话，但已有会话仍可继续使用它。
8. 没有命中或没绑定材料时仍按角色和技能回答，说明没有本班资料依据，引用为空、不写[n]，不再整段固定拒答。

旧班级配置接口 `/api/assistant/settings` 保留兼容，但新回答不再读取它。旧会话首次更新会迁移到本班默认智能体；迁移只给尚无智能体的班级创建默认智能体、数学引导技能，并绑定当时已有材料。后续新增材料必须显式绑定。

**旧资料默认学科是“通用”。** 问题识别为数学时只检索数学材料；请在详情把数学资料分类为“数学”，再在智能体中勾选绑定，避免误以为问答失效。

## 技能文件格式

```text
名称：数学解题引导
适用条件：数学，三角形，面积，方程
启用：是
工具：knowledge_search
学科：数学

先核对已知条件，给下一步提示与检查问题，不给最终答案、选项、得数或完整解题过程。引用本轮资料。
```

名称、适用条件、启用必填；工具可省略，默认knowledge_search；第六课支持逗号分隔的 `工具名@服务器名`，例如 `knowledge_search,formula_hint@campus-tools`，最多8个MCP引用。保存前必须连接同班服务器并成功发现对应工具；学科可省略，支持数学/语文/英语/通用。空行下面必须有正文。名称最多64字，条件最多1000字，全文最多8000字；拒绝重复字段、未知字段、非法工具。适用条件使用逗号或空白分隔词组，词组按包含关系计分。这是课内字段格式，不是通用SKILL.md YAML。

## 模型配置

默认gateway仅用模拟模板和64维字符二元组哈希Embedding，能验证流程，不能当作真实大模型效果。

使用已有DeepSeek配置即可：

```dotenv
CHAT_GATEWAY_BASE_URL=https://api.deepseek.com
CHAT_GATEWAY_API_KEY=<完整密钥，仅放本机.env>
CHAT_MODEL=deepseek-flash
```

官方DeepSeek请求设thinking disabled、max_tokens2048。Embedding仍走GATEWAY_*，没有改向量维度，不必重建现有索引。启用真实Embedding时需匹配EMBEDDING_DIM并重建全部向量索引。

页面中的流式助手使用CHAT_*，第四课旧 `/api/ask` 不带智能体的兼容路径仍使用GATEWAY_*；保留第4课行为，不应把两条生成路径混为一谈。

## Swagger与接口

- `http://localhost:8080/swagger/`：Swagger UI，资源随项目打包，无需外部CDN。
- `http://localhost:8080/openapi.json`：OpenAPI3.0.3。
- 文档由Go处理器的 `// @api` 注解在启动时生成，schema在 `api/openapi-schemas.json`；变更后重新构建API即可更新文档。不是一份与代码无关的手写路径清单。
- Swagger先调用POST /api/login，复制access_token，再点Authorize粘贴Token本身（无需再次输入Bearer），可试调主要接口。
- 新接口：GET/POST `/api/agents`、GET/PUT `/api/agents/{id}`，GET/POST `/api/skills`、GET/PUT/DELETE `/api/skills/{id}`，PUT `/api/materials/{id}/subject`。
- 创建会话：POST `/api/assistant/conversations`，JSON `{"agent_id":1}`；发送消息：POST `/api/assistant/conversations/{id}/messages`，JSON `{"question":"如何分析三角形面积？"}`，SSE meta/tool_call/delta/done/error。
- GET `/api/search?q=...&agent_id=...` 可限定启用智能体绑定范围（该路径使用混合检索）；不传agent_id保留第四课全班检索和mode参数。

学生不能写智能体或技能，返回403。跨班、非本人、不存在资源一致返回404；学生访问停用智能体同样404，教师可读取本班停用智能体以维护。所有业务接口使用Bearer。用户Token与外部模型密钥是两种不同凭据，后者不会返回浏览器。

新增MCP接口：`/api/mcp/servers` 及其connect/disconnect/tools/probe、`/api/mcp/calls`、`/api/mcp/demo/setup`，均限教师；会话traces限本人。Swagger中的probe可直接试调工具，调用结果同样入审计。详细路径以Swagger为准。

## 删除

教师在资料详情删除本班资料，正文和切片级联删除；文件与向量清理失败会留下持久化任务，后台重试。会话列表可删除本人的会话及全部消息，生成中返回409。历史引用快照保留，已删原材料返回404。已有生成可能使用删除前读取的资料快照。

## 接口验收与HTML报告

在项目根目录执行（依赖已运行的Compose、Docker CLI及Python3，无额外Python包）：

```powershell
python .\apitest\run_all.py
```

逐接口脚本按URL组织，例如 `apitest/cases/api/login/POST.py`。可单独执行：

```powershell
python .\apitest\cases\api\login\POST.py
```

自动读取本机.env中的teacher_a/student_a/teacher_b密码。通过 `docker compose exec db` 查询MySQL，核对会话、绑定、技能源码和消息正文，不只检查状态码。测试建立唯一命名的临时资料/技能/智能体/会话，结束清理；模型问答测试会消耗少量API额度。失败后报告说明清理问题，测试不修改既有教师配置。

输出 `apitest/reports/report.html`，已加入.gitignore，**不要提交报告或密钥**。报告脱敏用户口令和access_token。课堂要求的同伴验证与截图需由同伴登录后完成，分别记录到 `docs/week05-verification.md` 和 `docs/week06-verification.md`，不能用自动化测试冒充同伴复核。

## 本次验证与边界

已通过Go单元测试、go vet、Go静态编译、前端TypeScript/生产构建、SSE解析测试、Python语法检查。真实Compose/MySQL/Qdrant/DeepSeek端到端验收与同伴交叉验证需要在本机执行；没有将未执行项标为通过。

历史上下文最近6条完成消息，每条截断500字；最多200条消息/会话。输出正文6000字、模型超时120秒，Nginx150秒。引用编号会在增量和结束时过滤，不能据此保证每句话确实有证据支持；引导不直接给答案依赖模型遵循提示词，仍需真人验收。学科识别为规则词表，尚不覆盖任意学科。
