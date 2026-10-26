# Requirements

## Agent ownership and bindings
教师CRUD范围为创建、读取、修改、启停及绑定本班技能和材料；名称1–64字、提示词不超过2000字、班内名称唯一。跨班绑定404。学生列出和直读仅启用智能体，教师可读取停用智能体进行管理。

## Conversation identity
新会话要求本班启用agent_id，不存在、他班、停用一律404。会话归属只认登录用户和班级，后续消息的agent_id/class_id不能覆盖会话选定值。停用智能体后已有会话仍可继续。

## Skills
字段文件支持名称、适用条件、启用、工具、学科；前三项必填，工具仅knowledge_search。重复/未知字段、非法工具、空正文与超过8000字返回可读400。技能名称班内唯一；写操作仅教师。条件按逗号/空白词组词法包含计分，绑定且启用候选中取最高分；同分ID小者；零分不选。

## Retrieval and generation
每轮重读角色提示词、技能绑定及材料绑定。只在本班绑定材料与识别学科内混合检索前4条。角色与选中技能放系统消息，编号材料放用户消息；历史最近6条完成消息，每条最多500字符。角色留空fallback。解题引导名称命中时不写最终答案、选项、得数和完整过程。无检索结果仍作答，引用[]，增量和最终文本均移除[n]。提示词规范不能替代事实核验或完整提示注入防护。

## Protocol and documentation
fetch POST Bearer + SSE meta/delta/done/error，meta先带引用和selected_skill。Go处理器@api注解生成OpenAPI3.0.3，swagger资产本地打包，文档路由在业务路由之前挂载。主要请求/响应schema、错误码与认证说明可试调。

## Tests
apitest每URL/方法一脚本，run_all生成本地HTML报告且不入Git。至少登录、检索、提示词/技能、创建会话、发送消息，并核对MySQL数据。自动测试不冒充同伴交叉验证。例外记录与执行证据见docs/week05-verification.md。
