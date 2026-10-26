# 端点参考

当前共 4 个端点，全部只读。示例中 `$API` 指部署地址（如 `https://git.example.com`），
`$TOKEN` 为访问令牌明文（见 [认证与访问令牌](auth.md)）。

### 通用约定

- **只接受 `GET` 与 `HEAD`**：其余方法返回 `405` + `Allow: GET, HEAD`（`HEAD` 便于探活）；
- 成功响应是**裸 JSON**：单个对象或数组，没有信封包装；
- 响应头固定为 `Content-Type: application/json; charset=utf-8` 与 `Cache-Control: no-cache, must-revalidate`；
- 出错时状态码非 `200`，响应体为 `{"message": "...", "url": "..."}`（见 [错误、限制与约定](errors.md)）；
- 所有端点**都不分页**，也没有 `Link` 头。

### GET /api/v1/user

需要令牌，返回令牌所属用户。

```bash
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"
```

```json
{
  "id": 7341916824063314,
  "login": "giserxiao",
  "full_name": "giserxiao",
  "email": "dev@example.com",
  "avatar_url": "https://git.example.com/static/avatar/2f1c.png"
}
```

可能错误：`401`（缺少或无效令牌）、`500`（内部错误）。

### GET /api/v1/user/repos

需要令牌，返回该用户**已加入**的仓库，列表位于 JSON 数组顶层，**不分页**。
没有任何仓库时返回 `[]`（不是 `null`）。

```bash
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user/repos"
```

```json
[
  {
    "id": 3227920133129147,
    "owner": "dev",
    "name": "hello-world",
    "full_name": "dev/hello-world",
    "description": "示例仓库",
    "private": true,
    "fork": false,
    "empty": false,
    "default_branch": "main",
    "html_url": "https://git.example.com/dev/hello-world",
    "clone_url": "https://git.example.com/dev/hello-world.git"
  }
]
```

可能错误：`401`（缺少或无效令牌）、`500`（读取仓库列表失败）。

### GET /api/v1/users/{username}

匿名可读，按用户名读取用户基本资料，成功响应与上一节的用户对象相同。

```bash
curl -s "$API/api/v1/users/giserxiao"
```

可能错误：

- `404`：用户不存在（已软删除的账号同样视为不存在）；
- `422`：`{username}` 为空白（例如 `/api/v1/users/%20`）；
- `500`：内部错误。

### GET /api/v1/repos/{owner}/{repo}

令牌**可选**。`{owner}` 先按**仓库组名**解析，找不到再按**用户名**解析——
`CodeFever` 的仓库挂在仓库组下，对外用「组名/仓库名」表达，与常见的「owner/仓库名」结构对齐。

```bash
# 带令牌：可以读到自己的私有仓库
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/repos/dev/hello-world"

# 匿名：不可读的仓库一律 404
curl -s "$API/api/v1/repos/dev/hello-world"
```

成功响应是单个仓库对象（字段见下文「对象字段：仓库」）。

可能错误：

- `404`：仓库不存在，**或**当前调用者没有读权限——私有仓库对无权限者返回 `404` 而不是 `403`，
  以免泄露「该仓库存在」这一事实；匿名访问同样落在这里；
- `422`：`{owner}` 或 `{repo}` 为空白（例如 `/api/v1/repos/%20/%20`）；
- `500`：内部错误。

### 对象字段：用户

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | 数字 | 由内部 32 位 hex 主键的 SHA-256 摘要前 8 字节右移 11 位推导，跨请求稳定 |
| `login` | 字符串 | 用户名 |
| `full_name` | 字符串 | 当前实现与 `login` 同值 |
| `email` | 字符串 | 账号邮箱 |
| `avatar_url` | 字符串 | 头像地址，**可能为空串** |

> `id` 是一段**不透明**的标识：恒为非负整数，且不超过 JS 安全整数上限
> （`9007199254740991`），因此 JavaScript 的 `Number` 能精确表示、可直接比较。
> 但它**不是自增序号**，也不保证连续——请按标识符原样处理，不要从中推断顺序或数量。
> 同一对象的 ID 在可见期内恒定；如将来确需扩容，会改用字符串 ID 并记入 `CHANGELOG.md`。

### 对象字段：仓库

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | 数字 | 同上，由仓库主键推导，跨请求稳定 |
| `owner` | 字符串 | 仓库组名；取不到所属组时**回退为仓库名** |
| `name` | 字符串 | 仓库名 |
| `full_name` | 字符串 | `owner/name` |
| `description` | 字符串 | 仓库描述，可为空串 |
| `private` | 布尔 | 当前**恒为 `true`**：数据表还没有可见性列，所有仓库都按私有处理 |
| `fork` | 布尔 | 是否为 `Fork` 出来的仓库 |
| `empty` | 布尔 | 当前**恒为 `false`** |
| `default_branch` | 字符串 | 默认分支名，**可能为空串**（例如还没有任何提交） |
| `html_url` | 字符串 | 仓库网页地址 `<站点地址>/<owner>/<name>` |
| `clone_url` | 字符串 | Git 克隆地址 `<站点地址>/<owner>/<name>.git` |
