# 公开 API 概览

`/api/v1` 是 `CodeFever Next` 面向外部脚本、CI/CD 与第三方集成的公开 REST API：
只读、裸 JSON 响应、访问令牌鉴权。当前实现了 4 个只读端点，本文档只描述**已实现**的行为。

> 文中示例统一使用 `https://git.example.com`，请替换为你自己的部署地址。

### 与内部 API 的边界

`/api/v1` 与界面使用的内部 `/api/{controller}/{action}` 是**完全隔离**的两套契约：

| 维度 | 公开 API `/api/v1` | 内部 API `/api/*` |
|---|---|---|
| 路径形态 | `/api/v1/repos/{owner}/{repo}`，段数不定 | `/api/{controller}/{action}` |
| 成功响应 | 裸 JSON（对象或数组） | `{request-id,code,message,data}` 信封 |
| 错误响应 | `{message,url}` | `{request-id,code,message,data}` |
| 请求编码 | 标准 query 参数 | `multipart` |
| 鉴权 | 访问令牌 | 仅 Cookie 会话 |

内部契约受快照基线保护，字段改动会让回归网失效，因此公开 API 有自己的参数解析、
错误码与响应形态，两者只共用底层的仓库查询与权限判定。

另外，`/apiv1/internal/...` 是服务内部网关的回调前缀，与 `/api/v1` **毫无关系**，
既不是公开 API 的一部分，也不能与之混用。

### 30 秒上手

```bash
# 1. 部署地址与访问令牌（令牌目前需要运维侧签发，见「认证与访问令牌」）
export API=https://git.example.com
export TOKEN=<你的访问令牌>

# 2. 当前令牌所属用户
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"

# 3. 当前用户可见的仓库列表（JSON 数组，不分页）
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user/repos"

# 4. 读某个仓库（owner 先按仓库组名解析，再按用户名解析）
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/repos/dev/hello-world"
```

### 端点一览

| 方法与路径 | 鉴权 | 说明 |
|---|---|---|
| `GET /api/v1/user` | 必须带令牌 | 返回当前令牌所属用户 |
| `GET /api/v1/user/repos` | 必须带令牌 | 返回该用户已加入的仓库，不分页，返回 JSON 数组 |
| `GET /api/v1/users/{username}` | 匿名可读 | 按用户名读取用户基本资料，不存在返回 `404` |
| `GET /api/v1/repos/{owner}/{repo}` | 令牌可选 | 读取单个仓库；无读权限的私有仓库返回 `404` 而非 `403` |

成功响应的头部固定为 `Content-Type: application/json; charset=utf-8` 与
`Cache-Control: no-cache, must-revalidate`。

### 权限与可见性

公开 API 不另建权限体系：令牌代表一个用户，能读到的资源与该用户在界面里能看到的完全一致。
私有仓库对无读权限者一律返回 `404`，所以不要用状态码来判断"仓库是否存在"——
对匿名调用者来说，几乎所有仓库都会得到 `404`。

### 接下来

- [认证与访问令牌](auth.md)：三种令牌传法、生命周期与 `401` 排查
- [端点参考](endpoints.md)：逐端点的请求、成功响应示例与对象字段
- [错误、限制与约定](errors.md)：错误体、状态码、当前限制与变更策略
