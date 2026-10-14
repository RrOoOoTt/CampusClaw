# Use Authorization Bearer authentication

## Why
作业明确要求受保护接口通过 Authorization: Bearer 认证。当前随机会话 Token 通过 Cookie 传递，需要替换传递方式并提供可截图的请求头证据。

## Changes
登录返回 access_token、token_type 和 expires_in；鉴权只读 Bearer 请求头。前端统一添加请求头，包括文件下载与检索。保持随机会话摘要、12 小时有效期、登出撤销和班级权限。更新验收脚本与部署说明。

## Impact
无需迁移数据库或重置数据；升级后需要重新登录。仅持有旧 Cookie 的客户端返回 401。仍使用 opaque Token，不要求 JWT。
