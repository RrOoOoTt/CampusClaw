# Design

复用 sessions 表，Token 由 crypto/rand 生成 32 字节，hex 编码。登录返回 access_token，数据库存 HMAC-SHA256 摘要。requestToken 严格校验 Authorization 的单一 Bearer 凭据与 64 位十六进制格式；不从 Cookie、URL 或请求体读取 Token。auth 查询会话、过期时间、角色与班级；logout 删除同一 Token 的会话。

前端统一 apiFetch，使用 sessionStorage 保存当前标签页 Token，所有业务 fetch 均添加 Authorization，401 清除本地 Token，成功登出清除。文件下载经带头的 fetch 获取 Blob。Nginx 显式转发 Authorization。旧 Cookie 在登录/登出时清除，但不具备认证效力。

服务器只接受已存在且未过期的会话。客户端不得用 Token 内自称的角色/班级获取权限。Token 不通过 URL 传输。JavaScript 可读取 sessionStorage，因此维持 React 转义、Markdown skipHtml 和生产 HTTPS 的要求。
