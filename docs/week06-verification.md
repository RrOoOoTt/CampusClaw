# 第六课：MCP与工具接入验收

更新日期：2026-10-21。基于第六课《本课重点》《任务清单》《实践步骤与验收》增量实现；继续使用React18/TypeScript/Vite、Go net/http、MySQL、Qdrant、Nginx、Compose。第五课历史验收结论保留，不等同第六课通过。

## 实现与证据对应

| 要求 | 实现位置/行为 | 验证入口 |
| --- | --- | --- |
| 同仓独立MCP服务，至少两个工具 | mcp-server独立Go模块；formula_hint、unit_convert只给方法/关系提示 | selfcheck；mcp-server/main_test.go |
| 鉴权、握手、发现、调用 | POST /mcp JSON-RPC2.0；Bearer；initialize→notifications/initialized→tools/list→tools/call；版本2025-03-26 | 独立进程自检记录、API客户端测试 |
| 教师登记服务器和缓存工具 | mcp_registry.go；班级范围、凭据只存环境变量名，私有session持久化；连接原子替换发现缓存 | servers GET/POST、id GET/PUT、connect/disconnect脚本 |
| Skill声明工具 | tool@server，保存时验证同班、已连接、已发现；knowledge_search保留 | tutor.go、mcp_client.go；教师管理上传示例Skill |
| LLM候选工具及多轮循环 | 仅命中Skill的引用进入tools；模型选择，宿主校验执行；默认最多3轮/4次 | mcp_loop.go；受控模型多轮测试；calls GET脚本 |
| SSE与持久化 | 先meta与tool_call，再delta；tool_calls写入消息；会话读回保留状态 | assistant.go、Assistant.tsx；calls GET脚本 |
| 审计与权限 | mcp_calls记录用户、工具、参数、轮次、结果/原因；写审计失败禁止执行 | probe脚本验证ok/error/denied与SQL；calls GET验证班级 |
| 可查看模型参数与响应 | llm_traces；本人会话traces接口和页面展开区；不记录认证头 | traces GET脚本；阶段验收说明 |
| 文档和回归 | 注解生成Swagger；增加10个逐接口脚本，总计25个 | /swagger/；apitest/run_all.py |
| 多个AI应用 | 教师安装公式提示、单位换算应用；重复安装不覆盖已有同名配置 | MCP管理安装按钮、demo/setup接口 |

## 当前实际验证状态

| 验证 | 结果 | 范围与限制 |
| --- | --- | --- |
| API Go测试、go vet、静态编译 | 通过 | 包含原有与新增MCP测试；HTTP模拟上游验证握手、候选工具、多轮、次数上限、未知工具拒绝、重定向拒绝、敏感结果拒绝 |
| 独立MCP模块测试、go vet、静态编译 | 通过 | 独立模块，无MySQL依赖 |
| 独立MCP可执行文件真实启动并HTTP自检 | 通过 | docs/week06-evidence/standalone-mcp-selfcheck.json；实际服务进程，未用假的MCP实现，验证健康、401、握手、工具发现、两个工具调用 |
| 前端TypeScript/生产构建、SSE解析测试 | 通过 | 不等于浏览器人工验收 |
| Python语法及7项测试框架回归 | 通过 | 不需要Docker；不是25项业务接口的执行结果 |
| 多轮模型请求JSON证据 | 已生成 | docs/week06-evidence/tool-loop-requests.json，来自受控测试模型和测试执行器；不能据此宣称DeepSeek或数据库链路已通过 |
| Compose/MySQL/Qdrant整体验证、新增10项业务接口、真实模型工具调用 | 待本机执行 | 当前执行环境没有Docker/MySQL，不能标为PASS |
| 人工界面与同伴四项交叉验证 | 待记录 | 由本人/同伴实际操作并附截图与时间，不预填通过 |

## 本机部署与自动验收

覆盖源码，保留原.env与数据卷，添加MCP_AUTH_TOKEN（PowerShell用 `[guid]::NewGuid().ToString("N")` 生成32字符值）。不要重新复制.env.example覆盖已有凭据。首次启动自动创建MCP/trace表、迁移技能tool字段和消息tool_calls，不删除已有资料或会话。

```powershell
docker compose up -d --build
docker compose ps -a
docker compose exec -T mcp-demo /mcp-server selfcheck
python .\apitest\run_all.py
```

如果失败，保留脱敏日志：`docker compose logs --tail=100 api mcp-demo web`。查看apitest/reports/report.html，按真实结果填写。问答脚本会消耗少量模型额度；模型必须支持tools，否则应记录FAIL并检查模型/网关能力。只用普通问答成功不能证明工具链通过。不要在提交材料中放.env、访问Token、模型密钥或未脱敏的私人对话。

## 人工与同伴交叉验证清单

先教师A安装两个演示应用，上传并绑定数学资料，学生A新建对应会话。另备教师B/学生B为另一班，不能共享A的登录Token。Swagger Authorize填用户access_token本身。

| 场景 | 操作 | 预期结果 | 实测记录 |
| --- | --- | --- | --- |
| 正常多轮 | 学生A要求先米→厘米，再公式提示；展开状态及模型参数 | tools仅含该Skill引用；工具状态出现在正文前；请求包含role:tool和tool_call_id，回答不给题目最终得数 | 待填 |
| 跨班范围（同伴1） | 教师B用A的server_id调GET/PUT/connect/tools；学生B读A会话 | 与不存在ID一致404；B不能看A工具/会话/审计 | 待填 |
| 缺少凭据（同伴2） | 无登录Token请求/api/mcp/servers；运行独立服务selfcheck检查MCP无/错密钥 | API和MCP分别401；用户Token不能充当MCP凭据 | 待填 |
| 断开拒绝（同伴3） | 教师A断开服务器，再让学生A在原会话提出命中相同Skill的问题 | 缓存清空、状态disconnected；meta出现denied；本班审计可归属服务器的拒绝；普通问答继续尝试 | 待填 |
| 审计范围（同伴4） | 教师A查/api/mcp/calls；教师B查同接口；学生A也查 | A见本班调用与轮次；B无A记录；学生403 | 待填 |
| 工具错误与参数拒绝 | Swagger probe用unit_convert分别传m/cm、kg/cm、额外extra参数 | status依次ok/error/denied；有对应SQL审计；probe HTTP200是操作结果封装，不能仅按HTTP判断工具成功 | 待填 |
| 重复连接/重启 | 连两次，记录工具数；重启API后读配置/工具并问答 | 不重复累计；登记、缓存、会话状态持久化；远端失效仍需重新连接 | 待填 |
| 删除或重命名 | 保留Skill引用，删除/改名服务器再问同题 | 引用拒绝，不执行；已删除服务器无法归属，不造审计行；前端显示denied | 待填 |

每项记录：日期、本人/同伴姓名、实际状态码或tool_calls状态、截图路径、问题与修复后结果。截图建议包含Swagger路径与状态、对话工具状态、教师审计，不展示任何完整Token。

## 协议与安全边界

本课锁定2025-03-26协议，提供JSON响应HTTP子集；不支持stdio、SSE响应传输、OAuth授权服务、资源/提示词能力或任意第三方MCP兼容性。工具schema支持object/properties/required、array/items、string/number/integer/boolean及enum；未知约束直接拒绝，不忽略pattern/minimum等未实现规则。禁止附加参数，嵌套最多4层、字符串512字符、数组20项、数值绝对值不超过1e12。

仅允许部署者列出的MCP_ALLOWED_HOSTS，禁止重定向；本课单API实例用互斥锁串行管理操作与实际工具调用。多API副本需要分布式锁/版本控制；不应直接扩容宣称等价。外部工具返回只作为不可信数据，长度/敏感词/密钥回显检查并不构成完整提示注入防护，教学引导仍须人工检查。第三方模型/网络不可用时也可能普通问答失败，不能保证所有故障都生成答案。
