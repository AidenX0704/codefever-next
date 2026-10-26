# 错误、限制与约定

### 错误体

公开 API 的错误响应固定为 `{message,url}` 两个字段，没有信封、没有错误码：

```json
{
  "message": "仓库不存在",
  "url": "https://git.example.com/doc/cn/api/overview.md"
}
```

- `message`：可直接展示的提示文本；
- `url`：指向文档中心里的公开 API 文档页（中文版 `/doc/cn/api/overview.md`；英文版
  `/doc/en/api/overview.md` 内容一一对应），按部署时的站点地址生成。

成功响应同样是裸 JSON，`Content-Type` 为 `application/json; charset=utf-8`，
并带 `Cache-Control: no-cache, must-revalidate`。

几个真实形态的例子：

```json
{"message":"缺少访问令牌","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"用户不存在","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"读取仓库列表失败","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"未知的 API 端点，请查阅文档确认路径","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"该端点只支持 GET 请求","url":"https://git.example.com/doc/cn/api/overview.md"}
```

> **所有错误都是这两种形态之一**，包括"路径拼错"（`404`）与"方法不对"（`405`）——
> 不存在 `text/plain` 的错误响应，调用方可以无条件按 JSON 解析。

### 状态码

| 状态码 | 含义 | 出现场景 |
|---|---|---|
| `200` | 成功 | 裸 JSON 对象或数组 |
| `401` | 未认证 | 缺少令牌、令牌无效、令牌所属账号被禁用或软删除。仅出现在**必须带令牌**的端点：`/api/v1/user`、`/api/v1/user/repos` |
| `404` | 未找到 | 用户不存在；仓库不存在；私有仓库对当前调用者不可读（**返回 `404` 而非 `403`**，不泄露资源是否存在）；以及**路径不存在**（拼错端点） |
| `405` | 方法不允许 | 公开 API **只接受 `GET` 与 `HEAD`**；其余方法返回 `405` + `Allow: GET, HEAD` |
| `422` | 参数不可处理 | `{username}` / `{owner}` / `{repo}` 为空白，例如 `/api/v1/users/%20` |
| `500` | 内部错误 | 例如读取仓库列表失败 |

### 限制

- 只有 4 个**只读**端点，**只接受 `GET` / `HEAD`**。分支、标签、提交、文件内容、Release 与访问令牌管理端点均未实现；
- **没有分页**，也没有 `Link` 头：列表端点一次返回全部结果，规模较大时请调用方自行处理；
- **没有速率限制**，也**没有 scope 细分**：令牌权限等同于其所属用户的权限；
- **不提供 webhook / 事件订阅**。仓库里配置的 Webhook 属于界面功能，不由公开 API 管理；
- 访问令牌的管理界面尚未提供，当前需要运维侧签发（见 [认证与访问令牌](auth.md)）。

### 变更策略

`/api/v1` 与内部 `/api/*` 各自演进：内部契约受快照基线保护，公开 API 的新增以
**向后兼容**为原则——已有路径、字段与状态码语义尽量不变，新能力通过新增字段或新增端点提供。
如确实需要不兼容的变更，会在仓库的 `CHANGELOG.md` 中记录。

### 文档与代码

本文档的事实来源是 `backend/internal/httpapi/publicapi/`：

- `router.go`：路由表与 `/api/v1` 的命名空间隔离；
- `server.go`：各端点的鉴权方式、响应结构与状态码；
- `publicapi.go`：错误体形态、令牌提取规则与响应头。

端点表发生变化时，请同步更新本目录下的文档。
