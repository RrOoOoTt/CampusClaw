# 变更：第六课MCP与工具接入

现有多个智能体、Skill和班级知识库只能通过提示词影响回答，无法完成外部工具发现、授权执行和可追踪多轮调用。本次增加同仓独立MCP服务、教师按班登记/连接/发现、Skill工具引用、LLM调用循环、审计和模型参数展示。

保持现有Bearer登录、资料与会话删除、知识库班级边界及Compose数据卷。新增MCP_AUTH_TOKEN等环境变量，数据库启动时增量迁移。具体行为见specs/mcp-tools/spec.md；实际验证与待验项见docs/week06-verification.md。
