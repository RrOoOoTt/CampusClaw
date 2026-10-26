# 第五至六课逐接口测试

先启动项目，保持本机.env有SEED_TEACHER_A_PASSWORD、SEED_STUDENT_A_PASSWORD、SEED_TEACHER_B_PASSWORD、SESSION_SECRET。项目根目录运行 `python apitest/run_all.py`，或直接运行一个 `cases/.../METHOD.py`。

请求使用标准库urllib，数据库检查通过Docker Compose db容器中的mysql命令，不新增Python依赖。报告在reports/report.html，不提交Git。仅操作当前执行唯一命名的测试夹具；结束时删除测试会话、资料、技能并清理测试智能体。问答用当前CHAT_MODEL，会产生少量真实模型调用。

测试包括状态、返回字段、绑定、消息正文与MySQL一致性。技能开关与既有会话停用通过meta验证路由；真实模型是否避免得数仍需人工复核。跨用户脚本不能替代课堂同伴复核。执行失败将生成FAIL报告；不得把没有执行的测试写为通过。


第六课新增10个逐接口脚本，合计25个。运行前还需：

1. `.env` 设置32字符以上MCP_AUTH_TOKEN；`docker compose up -d --build` 启动mcp-demo。
2. `docker compose exec -T mcp-demo /mcp-server selfcheck` 独立验证无Token/错Token401、握手、发现、两个工具。
3. `python apitest/run_all.py` 验证登记、更新、重复连接不重复、工具清单、断开拒绝、试调、审计、模型请求记录。模型工具测试要求当前模型支持tools，未调用工具会真实FAIL，不自动跳过或假报成功。

夹具连接独立命名的同仓MCP服务登记项，不覆盖原教师登记项；测试会建立并清理自己的数据。先前15项PASS不代表新增10项已通过。完整Compose/MySQL、真实模型及同伴检查需在本机完成，见docs/week06-verification.md。
