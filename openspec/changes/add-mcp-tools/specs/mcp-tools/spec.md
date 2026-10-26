# MCP工具增量规范

## 新增要求

### 独立服务
必须实现受Bearer保护的POST /mcp及GET /health，支持initialize、notifications/initialized、tools/list、tools/call；提供formula_hint和unit_convert，返回教学提示，不计算题目最终得数。缺少/错误凭据401；无效方法/参数返回可读JSON-RPC错误。

### 登记和范围
仅教师可登记/修改/连接/断开/删除本班服务器、读取工具和本班审计；学生403，外班ID与不存在ID均404。服务器凭据不得返回前端。重复连接替换缓存不重复计数；失败和断开清除缓存。登记、缓存、私有会话、调用记录重启后保留。

### Skill与模型
Skill允许knowledge_search和tool@server引用。保存时必须验证本班连接与发现状态。本轮只把选中Skill的候选工具声明传LLM，不允许全部服务器工具泄露到候选集。LLM选择调用，宿主重新校验权限、已发现schema和调用预算；返回作为数据加入后续模型消息，不可改变系统策略。

### 结果与审计
先发送meta/tool_call状态再delta，消息保留tool_calls；记录每次可归属服务器的ok/error/denied、轮次、用户、时间、参数与结果。已删或未存在服务器不可造归属审计；仍在响应显示denied。审计失败不得执行。工具错误或结果不可信不得当作成功回传。

### 验收
提供Swagger、独立服务自检、逐接口脚本、模型参数查看及同伴范围/无Token/断开/本班审计四项手工步骤。自动测试不得代替同伴验证，未执行的真实环境测试不得写为通过。
