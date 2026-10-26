# 认证与访问令牌

### 令牌等同用户

公开 API 用**访问令牌**鉴权，令牌与用户一一对应：

- 权限**完全继承**令牌所属用户，判定走与界面相同的角色解析，公开 API 不另建权限体系；
- 令牌**不带 scope**：数据库列 `at_scope` 已预留，首个版本未实现，因此现在所有令牌权限相同；
- 一个用户可以持有多个令牌，其中一个失效不影响其它令牌。

### 三种传法

```bash
export API=https://git.example.com
export TOKEN=<你的访问令牌>

# 1. GitHub 风格：Authorization: token <明文>
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"

# 2. Bearer 风格
curl -s -H "Authorization: Bearer $TOKEN" "$API/api/v1/user"

# 3. query 参数（给无法设置请求头的 CI 使用）
curl -s "$API/api/v1/user?access_token=$TOKEN"
```

细节：

- 前缀大小写不敏感（`token` / `TOKEN` / `Bearer` 均可），前缀与明文之间必须是**恰好一个空格**，
  且整行只能分成两段；
- 同时提供请求头与 query 时，**请求头优先**；
- 其他写法（例如 `Authorization: Basic ...`、只有前缀没有明文）一律视为**未携带令牌**；
- `?access_token=` 会让令牌进入访问日志、代理日志与浏览器历史，能用请求头时优先用请求头。

### 生命周期

- 令牌明文**只在创建时返回一次**，服务端只保存 SHA-256 摘要，丢失后无法找回，只能重新签发；
- 令牌所属账号被禁用或软删除后，令牌**立即失效**；
- 公开 API 当前不提供令牌的到期时间、吊销与轮换端点。

### 当前限制：还没有管理界面

访问令牌的**管理界面**与**内部创建端点**在首个版本都尚未实现。目前需要由运维侧通过脚本
或数据库签发：生成一段高熵明文（实现为 32 字节随机数的 base64url 无填充编码），把它的
SHA-256 十六进制摘要写入 `cc_access_tokens`（所属用户主键 `u_key`、令牌名 `at_name`、
摘要 `at_sha256`，状态为正常），明文交给使用者保管。后续版本会提供 UI 与创建端点，
届时本文档同步更新。

> 请勿在文档、Issue 或日志中粘贴真实令牌。

### 401 排查

| 现象 | 常见原因 |
|---|---|
| `401`，`message` 为「缺少访问令牌」 | 既没带请求头也没带 `?access_token`；或请求头写成 `Basic`、或只有 `token` 而没有明文 |
| `401`，`message` 为「访问令牌无效」 | 明文抄错、含多余空格或引号；令牌已失效；所属账号被禁用或软删除 |
| 确认令牌正确但仍 `401` | 用的是界面登录的 Cookie 会话，公开 API 只认访问令牌，不读会话 Cookie |
| 匿名端点返回 `404` 而不是 `401` | 属预期行为：`/repos/{owner}/{repo}` 的令牌是**可选**的，无效令牌按匿名处理，私有仓库对匿名不可见 |

需要令牌的端点（`/api/v1/user`、`/api/v1/user/repos`）在令牌缺失或无效时返回 `401`，
响应体同样是 `{message,url}` 形态：

```json
{"message":"缺少访问令牌","url":"https://git.example.com/doc/cn/api/overview.md"}
```
