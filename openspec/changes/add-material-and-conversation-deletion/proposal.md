# 资料与问答历史删除
教师需要移除过期或误上传的本班资料，用户需要删除自己的问答历史。
新增 DELETE /api/materials/{id} 和 DELETE /api/assistant/conversations/{id}，均经过 Bearer 认证。资料详情中增加删除按钮，会话列表中增加删除按钮，执行前提示不可撤销。
