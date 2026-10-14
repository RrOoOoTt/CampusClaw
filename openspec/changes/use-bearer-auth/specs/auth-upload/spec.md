# MODIFIED Requirements

## Requirement: Bearer authentication
登录成功 MUST 返回 access_token、token_type=Bearer、expires_in=43200。受保护接口 MUST 只使用 Authorization: Bearer <token> 识别会话，验证有效期并从数据库获取角色和班级。

### Scenario: Successful login and authenticated request
WHEN 正确口令登录且携带返回 Token 请求 /api/me THEN 返回 200 和本人的角色班级。

### Scenario: Legacy Cookie or invalid credentials
WHEN 未携带 Authorization、仅携带 cc_session Cookie、错误 scheme、重复头、非法 Token 或未知会话 THEN 返回 401，不泄漏业务数据。

### Scenario: Expired or revoked token
WHEN 会话过期或登出删除会话后重用原 Token THEN 返回 401。

### Scenario: All frontend operations
WHEN 用户执行材料读取、上传、下载、检索、问答或重建索引 THEN 请求必须携带 Bearer 头且继续遵守教师权限与本班隔离。
