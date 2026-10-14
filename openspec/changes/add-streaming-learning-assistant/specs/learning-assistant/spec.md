# ADDED Requirements

## Requirement: Teacher-owned class configuration
系统 MUST 按登录班级保存教师角色提示词与解题引导开关，学生 MUST NOT 修改。修改 MUST 从下一轮生效，正在生成的轮次使用开始时的配置快照。

### Scenario: Teacher saves configuration
GIVEN A 班教师 WHEN 保存提示词与开关，即使请求夹带 B 班 class_id THEN 只改变 A 班配置，下一轮使用新配置。

### Scenario: Student attempts modification
GIVEN 学生 WHEN PUT 配置 THEN 403，原配置保持不变。

## Requirement: One-turn integration
系统 MUST 在每轮先验证身份和班级，再加载教师提示词、选择解题引导技能、检索本班知识库，并将三者用于本轮回答。

### Scenario: Guided answer with evidence
GIVEN 引导开启且本班检索命中 WHEN 提问 THEN 仅给分析思路、分步提示或检查问题，不直接给最终答案，并展示材料标题、切片和位置。

### Scenario: Skill disabled
GIVEN 教师关闭引导 WHEN 对同一题提问 THEN 可以依据证据直接回答，回答方式与开启时明显不同。

### Scenario: No class evidence
GIVEN 本班没有命中 WHEN 提问 THEN 说明“资料中未找到相关内容”、citations 为空，不编造出处或跨班取证。

## Requirement: Private continuous conversations
系统 MUST 保存会话与消息并支持追问；历史 MUST 来自服务端，用户与班级 MUST 从 Bearer 会话读取。

### Scenario: Follow-up and reload
GIVEN 已完成一轮 WHEN 追问 THEN 最近服务端历史进入上下文；刷新后可读到两轮问题、回答、引用及配置快照。

### Scenario: Another user or class
GIVEN 非会话拥有者 WHEN 读取或写入会话 THEN 404；列表不包含该会话。同班教师也无权查看他人的个人会话。

## Requirement: SSE delivery and persistence
回答 MUST 用 meta、delta、done/error 事件逐段输出。done MUST 表示完成且已保存；连接中断或错误 MUST NOT 伪装成功。前端 MUST 通过 Bearer 请求头访问流。

### Scenario: Incremental stream
WHEN 流式生成 THEN 在完成前显示多个 delta，最终 done 替换完整正文。Nginx 不缓冲整条响应。

### Scenario: Cancellation or failure
WHEN 用户停止或网关断流 THEN 中断状态与已收到的部分正文可保存并回看，无 done 成功事件。

### Scenario: Concurrent request
GIVEN 同一会话正在生成 WHEN 再次发送 THEN 409，不交错写入轮次。
