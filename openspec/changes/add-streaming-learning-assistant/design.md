# Design

## Workflow
Bearer auth 先校验服务端会话并读出用户、角色和班级。助手路由在 user_id 与 class_id 条件下读取会话，原子 busy 锁防止同一会话并发轮次。每轮读取 assistant_settings 的 prompt 与 guidance_enabled，随后构造解题技能指令并查询本班混合索引的前 4 条证据。模型 messages 包含服务端 system（教师角色设定 + 技能规则）、最近最多 12 条完成历史、用户问题及编号证据。客户端自带 history、role、class_id 不进入模型。

短追问结合最近用户问题进行检索，长问题使用自身；这是可复现的简单指代补全，非模型查询重写。无命中返回固定说明与空 citations，不调用模型。命中时 stream=true 连接网关，逐个转发 delta。指令要求开启引导不给最终答案；默认模拟网关显式区分引导/直接模式，真实模型的遵守程度需人工验收。

## Storage
assistant_settings: class_id 主键、system_prompt、guidance_enabled、updated_at。
assistant_conversations: user_id、class_id、title、busy、timestamps。
assistant_messages: role、content、citations JSON、prompt_snapshot、guidance_enabled、state、timestamps。

每轮先事务保存用户消息与 streaming 助手占位。完成后保存最终正文与 complete 状态再发 done；取消/错误保存收到的部分正文并标 interrupted/error。断电或进程崩溃时未写入的内存增量可能丢失，重启将 streaming 标为 interrupted 并释放 busy。此部署为单 API 实例；多副本须改为带租约的分布式锁。

## HTTP and SSE
- GET/PUT /api/assistant/settings: 登录可读；PUT 仅本班教师。
- GET/POST /api/assistant/conversations: 本人的会话列表/创建。
- GET /api/assistant/conversations/{id}: 本人本班消息历史；跨人/跨班/不存在均 404。
- POST /api/assistant/conversations/{id}/messages: JSON question，Authorization Bearer，SSE response。

SSE 事件：meta（message IDs、配置快照与 citations）、delta（text）、done（持久化后的最终 content/citations/state）、error（生成/保存失败，不伪装 done）。HTTP 错误在开始 SSE 前用 400/401/403/404/409/503 报告。会话最多 200 条消息；提示词最多 2000 字、问题最多 500 字；模型流 120 秒超时、正文最多 6000 字。

前端用 fetch + ReadableStream，统一注入 Bearer；EventSource 不用于这条带头的 POST。解析跨网络包、跨 UTF-8 字符的 SSE 帧，AbortController 停止生成。Nginx 关闭 buffering、转发 Authorization、延长读取超时。没有 Token 放在 URL 的做法。

## Provenance and security
证据来自原有 class_id 过滤与 SQL 再校验。存入消息的 citations 包括材料、切片、字符位置与摘录；点击原材料仍受 Bearer 和本班校验。字符位置以第 4 课处理后文本为坐标。模型引用编号超界会删除，若没有有效编号只附“本轮参考材料”，不伪称每句话已验证。最终 done 替换屏幕中的流式草稿。

Markdown 禁止 HTML；提示词可见于本班，不能写入秘密。教师只能修改本班角色设定；技能指令独立且包含用户提示不可覆盖的规则。真实模型仍可能生成不准确答案或违反引导规程；授权边界由程序和 SQL 执行，不能依赖模型。

## References
SSE framing: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events
OpenAI-compatible streaming: https://developers.openai.com/api/docs/guides/streaming-responses
