# Apply and verify

- [x] 增加每班教师配置、个人会话和消息持久化表。
- [x] 实现 Bearer 路由权限、配置快照、同轮提示词/技能/本班检索和追问历史。
- [x] 实现上游模型 SSE 解析、下游事件流、取消/错误保存和 Nginx 无缓冲。
- [x] 增加模拟网关引导/直接模式与真正逐段 SSE；保留真实模型接入配置。
- [x] 实现聊天主界面、个人历史、教师编辑、引用查看与停止生成。
- [x] 增加 OpenSpec、README、Go 单元用例与 smoke_assistant.py。
- [ ] 运行 Go 单元测试与 Compose 集成验收；当前执行环境缺少 Go/Docker，Docker 构建已配置 go test。
- [ ] 记录本机真实输出并归档变更。
