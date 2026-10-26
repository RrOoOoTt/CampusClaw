# 阶段性验收：五项问题与可重复验证

对应包含第六课MCP的代码。下表是代码结论及验证方法；真实模型、Compose与人工结论需填入week06-verification.md，不将受控测试等同线上运行。

| 验收问题 | 当前实现结论 | 验证位置 |
| --- | --- | --- |
| 每次LLM参数、返回是什么；提示词是否传递？ | 传model、messages、stream、max_tokens=2048；官方DeepSeek另带thinking.disabled。messages首条system包含智能体角色提示词和本轮命中Skill正文、系统边界，随后是最多6条历史（每条最多500字）及当前问题/检索片段。有候选工具时先发非流式决策请求，再单独发最终流式回答请求 | 页面“模型请求与返回”；GET本人会话traces，查看request/response，不包含Authorization头 |
| 绑定知识库会传全部内容吗？ | 不会。先限定班级、智能体显式绑定材料与识别学科；关键词+向量融合检索最多4个切片，只有命中片段的标题、编号、字符位置和正文进入当前user消息。无命中则没有资料正文。历史回答可能包含上轮内容，不能据此说整库重新注入 | tutor_pipeline.go retrieveScoped/tutorModelMessages；meta.citations与trace.request.messages对照 |
| 绑定Skill是否传全部内容？ | 不是所有已绑定Skill。规则按适用条件关键词计分，至多选择一个启用Skill；该Skill完整Body进入system（源码字段中的名称、条件等用于本地解析/路由，不原样整份文件发送）。其工具引用用于生成候选tools；未选Skill正文不发送 | tutor.go路由、tutor_pipeline.go；对照trace和教师Skill正文 |
| Skill具体执行逻辑？ | 上传文本→解析固定字段并验证工具引用→绑定智能体→本轮路由→将教学规程作为提示词。Skill文件不作为脚本执行，不运行上传代码。若声明MCP，则由LLM提出调用，宿主校验范围/schema/限额→先写审计→远端调用→检查结果→回传数据；普通Skill只有提示词行为 | parseSkill/chooseSkill、prepareMCP/executeMCP/runToolLoop；审计status与消息tool_calls |
| 集成MCP后的请求参数？ | LLM决策额外带tools:[{type:function,function:{name,description,parameters}}]、tool_choice:auto、parallel_tool_calls:false，stream:false。名称由tool@server映射为mcp_server__tool；模型返回assistant.tool_calls。宿主向MCP发JSON-RPC tools/call，params为name+arguments，附独立服务Bearer；返回过滤后成为role:tool（含tool_call_id）及user数据摘要。最终回答stream:true，不再传tools | trace中的tool_decision与final_stream、MCP审计、独立服务selfcheck |

注意：工具决策没有选择工具时，也会再请求最终流式答案，以保证只展示最终回答；这会增加一次模型请求。默认最多3轮决策、总计4次执行；每个调用记录round。知识库检索仍是宿主先执行的本地流程，knowledge_search标记不会自动变成远程MCP工具。

## 验证步骤

1. 教师A安装演示应用，上传examples/lesson5-math.md，分类数学并绑定。选择公式提示应用/对应Skill。
2. 学生A新会话问：“请先调用单位换算工具得到米到厘米的关系，再调用公式提示工具，给三角形面积的分析提示，不要计算最终得数。”
3. 观察工具状态及引用，再展开“模型请求与返回”。记录每个message_id的tool_decision轮次及final_stream。每轮tools应只来自选中Skill的工具引用；后续messages中应出现上一轮tool_calls与相应role:tool。
4. 比较角色模板、所选Skill的正文、知识库整份原文与request.messages：模板和所选Body出现；整库未注入，只有检索片段。换一个不命中Skill的问题作对照。
5. 教师A查MCP审计，对照tool_ref、round、arguments、result和status；教师B/学生权限检查见week06-verification.md。
6. 保存脱敏截图及文字结论，标注真实模型名称和时间。25项接口报告只按实际PASS/FAIL填写。

## 记录范围与限制

traces保存发出的JSON请求体和解析后的响应。tool_decision响应保留解析后的choices/message/tool_calls/finish_reason；final_stream保留聚合content、完成状态等。它不是完整原始HTTP报文，不保存认证头、供应商全部usage/reasoning字段或逐帧原始SSE。若作业要求HTTP逐字节完整响应，可另用本地脱敏抓包核对，不能把聚合记录称作原始全报文。

trace含用户问题、教师提示词与检索内容，限会话本人读取，教师无权默认读取学生trace；删除会话级联删除trace。MCP审计限本班教师读取，可能含学生工具参数；不要公开提交私人信息。当前无自动保留期限，可按课程数据管理要求另加清理策略。

证据：week06-evidence/tool-loop-requests.json是受控测试模型的请求记录，验证多轮构造；standalone-mcp-selfcheck.json是独立MCP真实进程HTTP自检。两者不替代真实DeepSeek+Compose联合验收。
