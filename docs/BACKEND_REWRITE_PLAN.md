# CodeFever Community 后端重写方案（Go 1.25）

> 决策依据：`docs/ARCHITECTURE_ANALYSIS.md`（架构分析）+ 运行态实测 + MySQL 8.0.46 兼容性实测 + 浏览器 UI 取证。

---

## 0. 决策摘要

| 决策项 | 结论 | 状态 |
|---|---|---|
| 重写语言 | **Go 1.25** | ✅ 用户已确认 |
| 数据库 | **MySQL 8**（从 5.7.31 升级） | ✅ 用户已确认 |
| Go 网关 | **保留**（`http-gateway` / `ssh-gateway` 不重写） | ✅ 用户已确认 |
| 前端 | **零改动**（保留 `www/view` 源码与 `www/view/build` 产物） | ✅ 用户硬约束 |
| 部署形态 | 单二进制 + 现有 supervisor 编排，替换 php-fpm 那一个进程 | 本方案建议 |

**一句话方案**：用 Go 重写 `application/` 全部 PHP 逻辑，**逐字保持** 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调 + 5 个服务端渲染页面的外部行为，使 `www/view` 前端产物与两个 Go 网关**一行不改**即可继续工作；数据库先独立升级到 MySQL 8 并让 PHP 后端在 8.0 上继续跑，作为重写期间的回归基准。

---

## 1. 目标与非目标

### 1.1 目标

1. **外部行为零变更**：URL 形状、HTTP 方法、参数来源、响应信封、错误码数值、静态资源路径、Cookie 名称 —— 全部逐字保持。
2. **UI 风格零漂移**：SPA 产物一字不改；9 个服务端渲染页面的 HTML 结构与静态资源引用逐字保持。
3. **修掉现有 P0 缺陷**：密码哈希、命令注入、越权读取、事务缺失、异常处理、CSRF/Cookie 安全。
4. **保留业务资产**：权限矩阵、事件类型、通知模板、git 语义、保护分支规则等按原语义搬运。
5. **MySQL 8 就位**：完成 5.7 → 8.0 升级并消除零日期依赖。

### 1.2 非目标（明确不做）

- ❌ 不改前端源码、不重装前端依赖（`@pgyer/mui-theme-preset` 声明为 `latest`，重装即漂移）。
- ❌ 不改两个 Go 网关的源码与部署路径（`http-gateway/main`、`ssh-gateway/shell/main` 的落位决定 `../env.yaml` 与 `../../env.yaml` 的解析）。
- ❌ 不改 URL 形状（不做 RESTful 重设计）。
- ❌ 不合并 Go 网关进新后端（SSH 网关涉及 pty/shell/密钥同步，独立进程更安全）。
- ❌ 不实现 `user/getCode`、`user/confirmTel`（前端调用了但后端从未存在，运行时必然 404 —— 保持"不存在"即可，属死代码）。
- ❌ 不实现 `RepositorySettingJob.js` 的 3 个幽灵端点（`repository/jobTemplates` 等，页面未注册进路由）。

---

## 2. 兼容性红线（逐条不可变更）

### 2.1 路由与形状

| 项 | 逐字值 |
|---|---|
| API 前缀 | `/api/<controller>/<action>`；动作与 HTTP 方法组合定位（`admin/config` 同时有 GET 与 POST） |
| 内部回调前缀 | `/apiv1/internal/<action>`（`searchPublicKey` / `authSSH` / `authHTTP` / `hooks`） |
| 动词分派 | CI `_remap` 语义：`segment(3)` 为动作名，方法名 = `<action>_<method小写>`；不存在 → `0x0105`（404） |
| 无动作 | 无 `index_get`/`index_post` → 缺 action 一律 404 |
| 静态资源 | `/static/js/main.<hash8>.js`、`/static/css/<id>.<hash8>.css`、`/static/media/...`、`/static/images/...`、`/static/<14位数字>/images/...`（nginx rewrite） |
| SPA 产物 | `www/app` → `www/view/build`（符号链接）；`/app/index.html` 回退 |
| 文件下载 | `/file/fetch/avatar/<fileKey>` → 二进制（白名单仅 `avatar`） |
| 页面路由 | `/user/login`、`/user/logout`、`/user/register`、`/user/resetPassword`、`/user/verify` |
| SPA 路由 | `/repositories`、`/settings/*`、`/admin/*`、`/groups/*`、`/:groupName/:repositoryName/*` —— **由前端 router 处理，后端只提供 index.html 回退** |

### 2.2 响应信封

```json
{"request-id":"<uuid大写无横线>","code":0,"message":"Success","data":{...}}
```

- 字段顺序与拼写：`request-id`（body 内，连字符小写）/ `code` / `message` / `data`；响应头另带 `Request-ID`。
- `code` 为**十进制整数**（源码是十六进制常量，JSON 中呈现为 `513`、`1025`…）。成功 = `0`。
- 成功 `message` 恒为英文 `"Success"`（API 层无中文成功文案）。
- 失败 `data` 默认为 `[]`（JSON 数组）。
- `paging` 字段：**现有实现永不出现**（`setTotal()` 无调用点）。新后端**可以**补上，但必须确认前端不因多出字段而改变行为 —— 建议先不加，严格对齐。
- 响应头：`Content-Type: application/json`、`Cache-Control: no-cache, must-revalidate`、`Content-Length`、`Request-ID`。

### 2.3 错误码（**数值不可改**，前端 `ErrorCodeParser.js` 逐个依赖）

| 码 | 十进制 | HTTP | message | 前端行为 |
|---|---|---|---|---|
| `0x0000` | 0 | 200 | Success | 成功 |
| `0x0101` | 257 | 403 | Invalid Login Credential | **3 秒后跳 `/user/login`** |
| `0x0102` | 258 | 400 | Method Not Allowed | — |
| `0x0103` | 259 | 400 | Accept and Content-Type Is Required In Header | — |
| `0x0104` | 260 | 400 | Content-Length Is Not Match The Content Size | — |
| `0x0105` | 261 | 404 | Endpoint Not Found Or Not Allowed Current Request Method | — |
| `0x0106` | 262 | 403 | Action Not Allowed | 权限不足 |
| `0x0200` | 512 | 400 | empty | — |
| `0x0201` | 513 | 400 | Missing Parameters | 参数缺失 |
| `0x0202` | 514 | 400 | Missing Amount Or Amount Out Of Range | 名称格式非法 |
| `0x0301` | 769 | 404 | Group Not Found | — |
| `0x0302` | 770 | 404 | Repository Not Found | 仓库不存在 |
| `0x0401`–`0x0412` | 1025–1042 | 400/403/404 | 见分析文档 §6 | 业务细分 |

**阈值语义（前端硬编码）**：`code < 0x0400` → 业务错误弹 toast；`code > 0x0400` → 视为系统错误（`admin/Groups.js:292`）。新增错误码必须落在正确区间。

### 2.4 请求处理规则

| 规则 | 现有行为 | 新后端要求 |
|---|---|---|
| `Accept` | 只认 `application/json`（→json）与 `*/*`（→binary）；其余 → `0x0103` | **逐字复刻**（含复合头 `application/json, text/plain, */*` 被拒的行为） |
| `Content-Type` | `application/json`→json；`multipart/form-data`→form；`application/x-www-form-urlencoded`→form-text；无→get；其他→`0x0103` | 逐字复刻 |
| 参数来源 | query（22 端点）/ body-`parsed`（84 端点）/ `$_FILES`（3 端点） | 逐字复刻；**注意 multipart 是前端 POST 的唯一形态** |
| `Content-Length` | 必须精确匹配 body 长度，否则 `0x0104`；**multipart 豁免** | 逐字复刻 |
| GET 带 body | 4 个 internal 端点用 GET + JSON body | 必须容忍 |
| 分页 | 现状：query 传参被 `is_int` 判定恒假 → **永远 page=1,pagesize=20** | 见 §5.4「已知缺陷的处置」 |
| 字段名编码 | 嵌套对象展开为 `key[subkey]`；数组展开为 `key[]`（重复字段）；`_SAFE`/`_BINARY` 后缀剥离 | Go 侧用 `r.Form`/`r.MultipartForm` 天然支持 `key[]` 与 `key[subkey]`；**后缀剥离需手工实现** |

**实测的执行顺序**（必须逐字复刻，否则前端行为会变）：

```
Request::parse()（Accept / Content-Type / Content-Length 校验）
   → 鉴权（APIAuth）
   → 方法查找（<action>_<method> 不存在 → 0x0105）
   → 业务处理
```

实测证据：
| 请求 | 结果 |
|---|---|
| `GET /api/user/info`（无 Accept） | **400** `0x0103`（Accept 校验**先于**鉴权） |
| `GET /api/user/info`（有 Accept，未登录） | **403** `0x0101` |
| `GET /api/user/create`（有 Accept，未登录） | **403** `0x0101`（鉴权**先于**方法查找，故不是 404） |
| `POST /api/repository/create`（已登录，缺参） | **400** `0x0201` `Missing Parameters` |
| `GET /api/nope/nope`（controller 不存在） | **404** + `Content-Type: text/html` + `nopage/index.php` 的 HTML（**不是 JSON 信封**） |

> ⚠️ 最后一行是易漏点：**未知 controller 与已知 controller + 未知动作是两套响应**。前者是 HTML 404 页（需移植 `nopage/index.php`），后者是 JSON `0x0105`。

### 2.5 认证

| 项 | 现有 | 要求 |
|---|---|---|
| 会话 Cookie 名 | `codefever_community`（`env.yaml: session.name`） | 保持同名 |
| 有效期 | 864000 秒（10 天） | 保持 |
| Cookie 属性（实测） | `Path=/`、`Secure=False`、**`HttpOnly=False`**、无 `SameSite`；值长 916 字节（CI2 加密 Cookie） | **新后端改 `HttpOnly` + `SameSite=Lax` + 生产环境 `Secure`**（前端 `credentials:'same-origin'` 不受影响，Cookie 值格式本就可变） |
| 存储 | CI2 加密 Cookie（mcrypt），**不在 DB** | 改为服务端会话表；**切换时需一次重新登录**（见 §12.2） |
| 网关 Token | `Token` 请求头 vs `env.yaml: gateway.token` | 路径与 header 名不变；**比较方式升级为 `hmac.Equal`** |
| 密码 | `md5(md5(x))` | 新后端**双轨**：识别旧哈希并验证通过 → 登录即升级为 argon2id |
| 登录响应 | `{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}`，但 **响应头是 `Content-Type: text/html; charset=UTF-8`**（非 application/json） | 信封与 Content-Type **都要逐字保持** |

### 2.6 网关与钩子

| 项 | 逐字值 |
|---|---|
| authHTTP 请求 | `GET {gateway.authHTTP}`，头 `Accept: application/json` + `Content-Type: application/json` + `Token: <token>`，body `{user,pass,repo,action}` |
| authHTTP 响应 | `data:{storage,uid,user,addr,path}`；`uid = "<u_key>|<r_key>"`；`path` = git-storage 绝对路径 |
| authSSH 请求 | `GET {gateway.authSSH}`，同头，body `{userID,repo,action}` |
| authSSH 响应 | `data:{uid,path,action}` |
| hooks 请求 | `POST {gateway.hooks}`，头同上，body `{uid,args,stdin}` |
| hooks 响应 | `data:{remote}`；`code==0` → 钩子 exit 0；`code!=0` → 钩子打印 `data.remote` 并 exit 255 |
| 失败映射 | Go 网关把**非 404 一律改写 401** → 后端"仓库/分组不存在"必须用 **404 / `0x0302`** |
| `PGYER_UID` | `"<u_key>|<r_key>"` 两段格式，贯穿 CGI env / `os.Setenv` / hooks → 后端 |
| 保护分支拒绝 | `0x040F`（403）+ `data.remote` = `"…You are not allowed to push code to protected branch <branch> !"` |

### 2.7 配置键（`env.yaml` / `config.yaml` 结构保持不变）

Go 网关只依赖 4 个键：`gateway.token`、`gateway.authHTTP`、`gateway.authSSH`、`cli.http`。
新后端需读取：`mysql.*`、`session.*`、`totp.salt`、`path.file`、`path.git`（**本次要让它真正生效**）、`cli.git`、`cli.shell`、`cli.http`、`users.*`、`gateway.*`、`host`、`ssh`、`allowRegister`、`email.*`。

### 2.8 数据表与目录

- 17 张表名与列名**全部不变**（含 `cc_proteced_branch_rules` 的拼写错误 `proteced`）。
- `git-storage/<32-hex>` 扁平布局、全 bare 仓库、`hooks → ../../misc/hooks` 软链 —— **不变**。
- `file-storage/avatar/<前5字符散列>/<key>` —— 不变。

---

## 3. 新架构总览

```
                    ┌─────────── nginx :80（保持不变）───────────┐
   浏览器 ─────────▶│ /api /apiv1 /user /file → upstream:api     │
   git (HTTP) ─────▶│ /*.git → :27555                            │
   git (SSH) ──────▶│ /static /app → www/static, www/view/build   │
                    └──────────────┬─────────────────────────────┘
                                   │ FastCGI 换为 HTTP 反代
                    ┌──────────────▼─────────────────────────────┐
                    │ codefever-api (Go 1.25, 单二进制)            │
                    │  httpapi ─▶ contract ─▶ handlers            │
                    │                  │                          │
                    │            domain（业务规则）                 │
                    │             ├─▶ repo（MySQL 8）             │
                    │             ├─▶ gitsvc（exec argv，无 shell） │
                    │             ├─▶ events ─▶ queue（DB 表）      │
                    │             └─▶ storage（file-storage）       │
                    └────────────────────────────────────────────┘
                                   │ 异步
                    ┌──────────────▼─────────────────────────────┐
                    │ codefever-worker (Go 1.25, 同一仓库)         │
                    │  事件消费 → 站内信 / 邮件 / Webhook 投递      │
                    └────────────────────────────────────────────┘

   保留不动：http-gateway (Go) / ssh-gateway/shell (Go) / sshd / cron / nginx
```

### 3.1 目录结构（新建 `backend/`）

```
backend/
├── go.mod                        module codefever-community/backend
├── cmd/
│   ├── codefever-api/main.go     HTTP 入口
│   └── codefever-worker/main.go  后台任务消费者
├── internal/
│   ├── config/                   env.yaml + config.yaml 加载（yaml.v3），密钥不落日志
│   ├── contract/                 ★ 契约兼容层（信封 / 错误码 / 参数解析 / Accept&Content-Type）
│   │   ├── envelope.go           request-id、code、message、data、paging
│   │   ├── errorcode.go          31 个错误码常量 + HTTP 映射
│   │   ├── parse.go              Accept/Content-Type/Content-Length/query/body/files
│   │   └── errors.go             统一 error → code 的映射
│   ├── httpapi/
│   │   ├── router.go             net/http ServeMux（Go 1.22+ 增强路由，零路由依赖）
│   │   ├── middleware.go         panic recovery / 请求 ID / 结构化日志 / 计时
│   │   ├── auth.go               session 中间件 + admin 中间件 + gateway token 中间件
│   │   └── pages.go              9 个服务端渲染页（Go html/template）
│   ├── handlers/                 user / admin / group / repository / internal
│   ├── domain/                   ★ 业务规则（可直接搬运）
│   │   ├── access.go             权限矩阵 + 角色继承 + REPO_ACTION_MAPPING
│   │   ├── protected_branch.go   保护分支匹配
│   │   ├── event.go              39 个事件类型 + 载荷 schema
│   │   ├── notification.go       位掩码语义 + 受众计算 + 模板占位符
│   │   └── mr.go                 MR 状态机 + 合并模式语义
│   ├── repo/                     MySQL 访问层（显式 SQL，禁 ORM 隐式行为）
│   ├── gitsvc/                   git CLI 封装（argv 数组，无 shell）
│   ├── events/                   事件总线 + 派发（写 DB 后由 worker 消费）
│   ├── notify/                   站内信 + 邮件（SMTP）
│   ├── webhook/                  投递器（签名 / 重试 / 死信）
│   ├── session/                  MySQL 会话存储
│   ├── storage/                  头像文件存储
│   └── auth/                     argon2id + 旧 md5(md5) 双轨 + TOTP + token
├── migrations/                   版本化 SQL（go:embed）
└── templates/                    9 个服务端渲染页的 Go 模板（从 PHP 逐字移植）
```

### 3.2 依赖选择（刻意保持极简）

| 用途 | 选型 | 理由 |
|---|---|---|
| HTTP 路由 | **标准库 `net/http.ServeMux`**（Go 1.22+ 支持 `POST /api/{action}` 模式） | **零路由依赖**；`/api/<ctrl>/<action>` 天然可表达 |
| DB 驱动 | `github.com/go-sql-driver/mysql` | 支持 `caching_sha2_password` |
| 查询 | **显式 SQL + `sqlc` 生成**（备选 `sqlx`） | 避免 ORM 的 N+1 与隐式行为；SQL 可审计、可按索引调优 |
| YAML | `gopkg.in/yaml.v3` | 与 PHP `yaml_parse_file` 语义对齐 |
| 密码 | `golang.org/x/crypto/argon2` | — |
| TOTP | `github.com/pquerna/otp`（RFC 6238） | 现有登录 2FA 已是标准 GoogleAuthenticator |
| 日志 | 标准库 `log/slog`（JSON） | 零依赖 |
| 后台任务 | 自建 DB 队列表 + `FOR UPDATE SKIP LOCKED` | 不引入 Redis/MQ |

> **不引入** ORM、DI 框架、Web 框架、Redis。理由：本项目的复杂度在"业务规则"而非"技术栈"，减少依赖即减少迁移风险；Redis 属于新基础设施，用户已明确"升级 MySQL 8、保留 Go 网关"，未授权新增中间件。

---

## 4. 契约兼容层（最关键模块）

这是"前端零改动"的技术前提。**所有对外部行为的兼容都收敛在这一个包里**，其余代码可以自由重构。

### 4.1 信封

```go
type Envelope struct {
    RequestID string `json:"request-id"`
    Code      int    `json:"code"`
    Message   string `json:"message"`
    Data      any    `json:"data"`
    Paging    *Paging `json:"paging,omitempty"` // 默认不出现，保持与现状一致
}
```

- `request-id`：**大写带连字符的 UUID**（实测形如 `AA0F618B-0560-5B08-0E3B-99946AD3C1C9`），body 与响应头 `Request-ID` 一致。
- `Data` 失败时序列化为 `[]`（空数组）而非 `null` —— 用自定义 `MarshalJSON` 或在构造时给默认 `[]any{}`。

### 4.2 参数解析

```go
type Parsed struct {
    Query   map[string][]string
    Body    map[string][]string   // form / form-text 的 $_POST 语义
    JSON    map[string]any         // application/json
    Files   map[string][]*File     // $_FILES
    Accept  string                // json | binary
    Method  string
    Token   string                // HTTP_TOKEN
}
```

关键兼容点：
- `application/x-www-form-urlencoded` → **必须解析**（登录页 `/user/login` 的 POST 就是这个形态，PHP 侧 `$_POST` 读取）。
- `multipart/form-data` → 前端所有写操作的形态，必须支持 `key[]` 重复字段与 `key[subkey]` 嵌套展开。
- `_SAFE` / `_BINARY` 后缀剥离（`VaribleHelper.js:21`）→ 后端收到的字段名是 `avatar`，不是 `avatar_BINARY`。
- 保留**空值与缺失的区别**：`$_POST` 中存在但为空的字段（如 `description=''`）必须能被读取为 `""`，不能与"缺失"混淆。Go 的 `r.Form` 天然区分（`Has` 检查）。

### 4.3 错误映射

```go
func (e *Error) Code() int        // 返回十进制错误码
func (e *Error) HTTPStatus() int  // 由错误码表推导
```

所有 handler 返回 `error`，由中间件统一转成信封。**禁止 handler 直接写响应**。

### 4.4 权限校验统一入口

```go
func (s *Service) RequireRepositoryPermission(ctx, rKey, uKey, perm UACPerm) error
func (s *Service) RequireGroupPermission(ctx, gKey, uKey, perm UACPerm) error
```

所有资源级端点必须走这两个入口 —— 这直接消灭现有 11 处越权读取缺口。

---

## 5. 各域实现要点

### 5.1 认证与会话

| 项 | 方案 |
|---|---|
| 会话存储 | 新表 `cf_sessions(id CHAR(32) PK, u_key, payload JSON, created_at, expires_at, last_seen_at, ip, ua)`；Cookie 只存 32 位随机 session id |
| Cookie | `codefever_community=<id>; Path=/; HttpOnly; SameSite=Lax; Max-Age=864000`（生产加 `Secure`） |
| 兼容期 | 切换窗口内**接受** PHP 的旧 CI2 加密 Cookie 会导致解析失败 → 走 §12.2 的重新登录方案 |
| 密码哈希 | `auth.Verify(pw, stored)`：`stored` 以 `$argon2id$` 开头 → argon2id；否则按 `md5(md5(pw))` 比较（**比较后立即升级**） |
| 密码哈希参数 | argon2id, m=64MB, t=3, p=2, keyLen=32, saltLen=16（OWASP 2024 基线） |
| 登录 2FA | 现有 `u_2fa` 是标准 RFC 6238（`GoogleAuthenticator`）→ 直接用 `pquerna/otp` 兼容；**`u_2fa` 现为明文存储 → 改为加密存储（AES-GCM，密钥取 `totp.salt`）** |
| 邮箱验证码 | 现有 `Utility/TOTP.php` 是**自定义非标算法**（`sha256(md5(i).md5(salt))` + 非标准截断，±300s 窗口），纯服务端闭环 → 可替换为标准 6 位码 + 5 分钟窗口 + 5 次尝试上限。**替换期间在途验证码（10 分钟内）会失效，属可接受降级** |
| 网关 Token | `hmac.Equal` 常量时间比较；后续可加 `Token` + `X-Timestamp` + `X-Nonce` 签名（Go 网关同步改，属可选增强） |

### 5.2 数据层

**原则**：显式 SQL + 绑定参数（消灭 LIKE 拼接）、显式事务边界、批量查询消除 N+1。

| 现有 N+1 | 新实现 |
|---|---|
| `repository_model.php:24-49` 每仓库 2 条 SQL | 单条 JOIN + GROUP BY 聚合出 fork 数与 MR 计数 |
| `repository_model.php:51-67` 每成员 2 条 SQL | 成员列表一次性 `IN` 查询 + 提交邮箱批量查 |
| `repository_model.php:173-194` 每 MR 1 SQL + 1 git 进程 | 评审人批量查；"最近提交"改为批量 `git log --no-walk` |
| `api/repository.php:157` 列表内调 `getMembers()` | 列表接口只返回摘要（成员数），详情接口再拉成员 |

**事务边界（新增）**：

| 操作 | 事务范围 |
|---|---|
| 创建仓库 | 事务内插 `cc_repositories`；git init 失败 → 事务回滚 + 清理磁盘目录 |
| Fork 仓库 | 事务内插行；`git clone --bare` 失败 → **必须检查退出码并回滚**（修掉现有孤儿记录缺陷） |
| 创建 MR | 事务内插 MR + 评审人；**MR ID 改用 DB 自增或 `SELECT ... FOR UPDATE`**，不再用 `max(mr_id)+1` |
| 通知写入 | `cc_notification_content` + `cc_notification_users` 同一事务 |
| changeOwner | 改角色 + 改 owner + 删旧成员 同事务 |
| 保护分支规则 | 写规则 + 发事件 同事务（事件表与业务表一致） |

**主键生成**：保留 `md5(time()+rand()+rand()+rand()+uniqid())` 的 32 位 hex 形态（`UUID::getKey()`），但改用 `crypto/rand` + 时间戳混合，**输出格式不变**（32 位小写 hex）。`r_path` 目录名格式不变。

### 5.3 Git 层（最大重写机会与最大风险源）

**现状**：所有 git 操作经 `Command::wrapArgument()` 转义后**拼成 shell 字符串**交 `exec()`，且 `batch()` 丢弃退出码。

**新实现**：

```go
func (g *Git) run(ctx context.Context, dir string, args ...string) (stdout string, err error) {
    cmd := exec.CommandContext(ctx, g.bin, args...)   // ← argv 数组，永不经 shell
    cmd.Dir = dir
    cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "GIT_TERMINAL_PROMPT=0")
    var out, errBuf bytes.Buffer
    cmd.Stdout, cmd.Stderr = &out, &errBuf
    if err := cmd.Run(); err != nil {
        var ee *exec.ExitError
        if errors.As(err, &ee) { return out.String(), fmt.Errorf("git %s: %w (exit %d): %s", args[0], err, ee.ExitCode(), errBuf.String()) }
        return "", err
    }
    return out.String(), nil
}
```

- **彻底移除** `wrapArgument` 与 `Command::batch()`。
- 退出码通过 `*exec.ExitError.ExitCode()` 精确判定 → 修掉"成功但无输出 vs 失败"不可区分的问题。
- 所有用户输入（commit keyword、path、branch、tag、commitSHA）作为**独立 argv 元素**传递；路径类参数前置 `--` 防 option 注入。
- 保留 `--pretty` 输出协议（分隔符 `\ue000\ue001\ue000`）或改用 `--format` 显式字段 —— **契约快照阶段对比后决定**，前端只读 JSON 字段，格式可换但字段名不可换。

**需要保持的 git 语义**（`domain` 层保证）：

| 语义 | 现有位置 | 要求 |
|---|---|---|
| 仓库创建 = `git init --bare` | `repository_model.php:652` | 保持 |
| Fork = `git clone --bare <src> .` | `:662` | 保持 |
| 仓库 hooks = `rm -r hooks && ln -s ../../misc/hooks hooks` | `:653-654` | **相对路径软链必须保持**（`git-storage/<uuid>` 的 `../../misc/hooks` = 根 `misc/hooks`） |
| 仓库目录权限 `chmod -R 0777` | `:655` | 改为 `0750` + 属主 `git`（安全修正，不影响功能） |
| 需要工作树的操作（merge/blame） | `Utility/Workspace.php` `/tmp/workspace/<uuid>` | 保持机制，换 Go 实现 |
| 合并模式 | `repository_model.php:1017-1110` | `message=NULL`→FF；`message`+`squash=false`→`--no-ff`；`message`+`squash=true`→`--squash`+commit |
| 提交身份 | `repository_model.php:644-645,1049-1052` | `GIT_COMMITTER_NAME`=邮箱 `@` 前段；`GIT_COMMITTER_EMAIL`=邮箱；`GIT_AUTHOR_*` 同 |
| 分支/标签名校验 | `constants.php` 正则 | 保持（`0x0202` 的判定依据） |

**仓库名连字符问题（顺手修掉）**：现状 nginx `[0-9a-z_]+` ❌ / Go 网关 ✅ / PHP `[\w\d_]+` ❌。重写时统一为 **`[0-9a-zA-Z_.-]+`**，三端同步（nginx vhost + Go 网关正则 + 新后端校验），并在建仓时**显式拒绝**含非法字符的仓库名（返回 `0x0202`），避免"建得出来但推不上去"。

### 5.4 已知缺陷的处置（关键决策点）

| 现有缺陷 | 处置 | 理由 |
|---|---|---|
| query 分页恒为 page=1 | **修正为真实分页** | 前端 `pagesize/page` 本就传了；修正只会让分页生效，不破坏契约。但需在契约快照中把"现状行为"与"目标行为"分别固化，避免误判为回归 |
| `paging` 字段永不出现 | **补上**（可选 `page`/`pagesize`/`total`） | 前端未读取 → 零风险；补上对调试有价值 |
| 建分支/建标签成功反而返回 `0x040C` | **修正为成功** | 明显是逻辑反转 bug；契约快照会记录"现状是错的"，需在快照里标注为**已知缺陷**，不作为回归基线 |
| `commitDetail` 等 11 处越权 | **补上鉴权** | 安全缺陷，必须修 |
| `getRepositoryWebhookLogs` 无归属校验 | **补上归属校验** | 安全缺陷 |
| `notification.setGroupOrRepoNotification` 已损坏（NULL 方法调用） | **修复并补测试** | 属于"功能实际不可用"，修复是净收益 |
| admin "已禁用 MFA" 列表恒空 | **统一写入/筛选语义**（写 NULL，筛选 IS NULL） | 一致性缺陷 |
| `md5(md5())` | **双轨 + 自动升级** | 安全 |

> **契约快照必须区分三类**：① 必须逐字保持的（信封、错误码、字段名、路径）；② 应保持业务语义但允许修 bug 的（权限判定、git 行为）；③ 允许改进的（分页、响应附加字段）。混淆这三类会导致"为了通过快照而保留 bug"。

### 5.5 事件、通知与 Webhook

**现状**：事件在请求线程内**同步内联**执行 —— Activity 写库、通知写库、**发邮件（阻塞 shell）**、webhook 写事件表，全部在请求内完成。

**新设计**：

```
handler ──事务内──▶ cc_events 表（新增）
                         │
                         └──▶ codefever-worker 轮询/订阅 ──▶ ActivityHandler
                                                        ├─▶ 写 cc_activities
                                                        ├─▶ UserNotificationHandler
                                                        │     ├─▶ 写 cc_notification_content/users
                                                        │     └─▶ 邮件（SMTP，带重试）
                                                        └─▶ WebhookHandler
                                                              └─▶ cc_repository_webhook_events → 投递
```

- 39 个事件类型与载荷 schema **原样搬运**（`Constant/EventType.php`）。
- 事件 payload 序列化格式不变（前端 `/api/user/notifications` 的 `data` 字段必须是 **JSON 字符串**，不是对象 —— 见 `VaribleHelper.js:248` 的 `JSON.parse`）。
- 通知受众计算规则（`UserNotificationHandler.php:202-231`）与位掩码语义（`constants.php:162-164`，**位=1 表示关闭**）原样搬运。
- 模板占位符 `{{$user@users:u_name}}` 与解析引擎原样搬运。
- 邮件：现有实现依赖系统 `mail` 命令且**恒返回 true**（失败不可感知）→ 改为标准 SMTP 客户端（`net/smtp` 或 `go-mail`），带失败重试与死信记录。**SMTP 配置需要新增 `env.yaml` 键**（`smtp.host/port/user/password/from`）—— 这是唯一需要扩展的配置结构，需在方案评审时确认。

### 5.6 服务端渲染页面（UI 一致性的隐藏战场）

**这是本次重写中最容易被低估的部分。** 登录/注册等页面**不是 SPA**，而是 PHP 服务端渲染：

| 页面 | 模板 | 行数 | 路由 |
|---|---|---|---|
| 登录 | `application/views/newpage/user/login.php` | 153 | `GET/POST /user/login` |
| 注册 | `.../register.php` | 197 | `/user/register` |
| 重置密码 | `.../reset_password.php` | 231 | `/user/resetPassword` |
| 2FA 验证 | `.../verify.php` | 142 | `/user/verify` |
| 登录后跳转页 | `.../login_activity.php` | 3 | — |
| 公共头/尾 | `.../header_include.php` / `footer_include.php` | 29 / 22 | — |
| 国家下拉 | `.../countries_dropdown_menu.php` | 28 | — |
| 文档详情 | `application/views/doc/detail.php` | 130 | `/doc/...` |
| 通用提示页 | `base/{error,success,disable}_message.php` | 108 | — |
| 落地页 | `nopage/index.php` | 24 | — |

**好消息**：这些页面用的静态资源在 `www/static/vendor/`（bootstrap-4.4.1 / fontawesome-free-5.13.0 / intltel / jquery-3.5.0 / jquery-validation-1.19.1 / poper-1.16.0 / toastr）与 `www/static/images/`，**由 nginx 托管，完全不动**。

**因此 UI 一致性的做法是**：
1. 把 PHP 模板**逐字**移植为 Go `html/template`：**DOM 结构、class 名、元素顺序、属性、资源引用路径一律不改**，只把 `<?php ... ?>` 求值替换为 Go 模板表达式。
2. `countries_dropdown_menu.php` 的 1287 行国家数据（`config/countries_item.php`）转成一份 JSON/Go map 静态资源。
3. **验收方式：像素级截图 diff** —— 用内置浏览器对 5 个页面在 PHP 侧与 Go 侧各截一张图，逐像素比对（见 §11）。

**注意**：`/user/login` 的响应信封与 `/api/*` **不同** —— 它是 `{code, message, extra}`（`application/controllers/base.php:101-108` 的 `_ajax_message()`），**没有 `data`、没有 `request-id`**。登录成功返回 `{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}`。这个信封也必须逐字复刻。

### 5.7 文件与头像

- 存储布局不变：`file-storage/avatar/<前5字符散列>/<key>`；DB 只存 key。
- 下载端点 `/file/fetch/avatar/<key>` 保持：白名单仅 `avatar`，直出二进制 + `Content-Type`，**不走 JSON 信封**。
- **补上安全校验**（现状缺失）：MIME/扩展名/大小校验 + 路径穿越防护（`filepath.Clean` + 前缀校验）。
- 现有上传字段名恰为 `avatar`（前端 `_BINARY` 后缀剥离的结果），新后端按 `avatar` 接收。

### 5.8 内部回调端点（4 个）

| 端点 | 新实现要点 |
|---|---|
| `authHTTP` | 邮箱 + 密码（`md5(md5())` **或** argon2id 双轨）→ 角色 → `REPO_ACTION_MAPPING` → 返回 `{storage,uid,user,addr,path}`；**"仓库不存在"必须用 404 / `0x0302`** |
| `authSSH` | `userID` + `repo` + `action` → 角色 → 返回 `{uid,path,action}` |
| `hooks` | `uid` 拆两段；`post-receive` 逐行解析 stdin（**必须恰好 3 段**）→ 发事件；`pre-receive` 命中保护分支 → `0x040F` + `data.remote`；`update` → 现状是**静默成功**（`code=0`, `remote:''`），保持 |
| `searchPublicKey` | 仓库内无调用方（外部 SSH 公钥查询服务使用）→ 按 `internal.php:21-37` 逐字实现 |

`uid` 格式 `"<u_key>|<r_key>"` 与 `PGYER_UID` 传递链是**跨进程硬契约**，任一端改了格式 push 即断。

---

## 6. MySQL 8 迁移

### 6.1 策略：**先升库，再换应用**

把 DB 升级与后端重写**解耦**：先让 MySQL 5.7 → 8.0 原地升级，**PHP 后端继续在 8.0 上跑**并通过全链路回归 —— 这样重写期间有一个已知正确的回归基准。

### 6.2 迁移步骤（**已实测**）

> **实测结论已更新**：首轮分析曾判断需要改 DDL 与种子数据（3 处改动）。
> 深入实测后发现**只需 1 处改动**即可做到**零行为变更**：
> 本项目的零日期是**有语义**的，不是脏数据，因此正确的做法不是改写数据，
> 而是**放开 MySQL 8 默认新增的 `NO_ZERO_DATE` / `NO_ZERO_IN_DATE` 限制**。

**为什么零日期必须原样保留**：PHP 侧 `strtotime('0000-00-00 00:00:00')` 返回 **-62169984000**（**不是 false**），该值经 `(int)` 转换后进入 API 响应（`repository_model.php:39` 的 `'updated' => (int) strtotime($item['r_updated'])`、`group_model.php:31`、`repository_model.php:2138`）。若把零日期改成 `NULL` 或 `0000-01-01`，epoch 会分别变成 0 与 -62167219200，**前端可见的 `updated` 字段会变化**。

| 步 | 动作 | 验证 |
|---|---|---|
| 0 | 备份：`mysqldump --single-transaction --routines --triggers` | 备份可恢复 |
| 1 | **唯一改动**：`misc/docker/docker-entrypoint.sh:23` 的 sql_mode 改为<br>`'ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'` | 移除 `NO_AUTO_CREATE_USER`（8.0.3+ 已删除，否则 `ERROR 1231`）<br>移除 `NO_ZERO_DATE` / `NO_ZERO_IN_DATE`（保留零日期语义） |
| 2 | `db_structure.sql` / `db_data.sql` / `db_update_20220215.sql` | **零改动**。实测在放宽后的 sql_mode 下，17 张表全部建成、种子数据原样写入、零日期值逐字保留 |
| 3 | 用 `mysqldump` 把 5.7 数据导入 8.0（`--single-transaction --set-gtid-purged=OFF`） | 行数与关键表计数一致（实测 3 用户 / 2 组 / 3 仓库 / 1 SSH key） |
| 4 | 校验隐式默认一致性 | 实测 5.7 与 8.0 对同一份 DDL 的 `datetime NOT NULL` 隐式默认**同为 `0001-01-01 00:00:00`**，无需处理 |
| 5 | 校验零日期存储值 | 实测两版本**完全一致**（`0000-00-00 00:00:00` 与 `0001-01-01 00:00:00` 均原样保留） |
| 6 | PHP 后端在 8.0 上跑全链路回归：登录、列表、Git HTTP/SSH clone+push、钩子回调 | 回归通过后再开始 Go 开发 |

**已应用的改动**（`misc/docker/docker-entrypoint.sh`）：

```sh
# NO_AUTO_CREATE_USER —— MySQL 8.0.3+ 已移除该模式，MySQL 8 下设置会报 ERROR 1231，故移除。
# NO_ZERO_DATE / NO_ZERO_IN_DATE —— 移除以保留本项目的零日期存储语义。
#   cc_* 表的 *_updated / *_deleted / mr_merge_time 等列显式使用 '0000-00-00 00:00:00'，
#   且 PHP 侧 strtotime('0000-00-00 00:00:00') 返回 -62169984000（非 false），
#   该值会经 (int) 转换进入 API 响应（repository_model.php:39 等），必须原样保留。
#   实测：MySQL 5.7 与 8.0 在放开该模式后，对同一份 DDL 与种子数据的隐式默认与存储值完全一致。
mysql ... -e"set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';"
```

> **注意**：`SET GLOBAL` 只影响**新连接**。`create_db.sh` 用的是独立的 `mysql` 客户端调用，因此会继承新的全局值。生产环境更稳妥的做法是把 sql_mode 写进容器内 `my.cnf`。

### 6.3 MySQL 8 注意事项

| 项 | 说明 |
|---|---|
| 认证插件 | MySQL 8 默认 `caching_sha2_password`。PHP 7.4 的 mysqlnd 支持；Go 的 `go-sql-driver/mysql` v1.5+ 支持。**建议容器加 `--default-authentication-plugin=mysql_native_password`**（8.0 可用，8.4 已移除）以降低变量 |
| `GROUPS` 保留字 | MySQL 8.0.2+ 起 `GROUPS` 是保留字（窗口帧单位）。表名 `cc_groups` 不受影响，但**任何裸 `groups` 别名都会语法错误** —— Go 侧 SQL 一律用 `g` 等非保留字别名（本次实测即被该保留字绊了一次） |
| 保留字全集 | 8.0 新增保留字还包括 `CUBE`、`CUME_DIST`、`DENSE_RANK`、`EMPTY`、`EXCEPT`、`FIRST_VALUE`、`GROUPING`、`GROUPS`、`JSON_TABLE`、`LAG`、`LAST_VALUE`、`LATERAL`、`LEAD`、`MEMBER`、`NTH_VALUE`、`NTILE`、`OF`、`OVER`、`PERCENT_RANK`、`PERSIST`、`PERSIST_ONLY`、`RANK`、`RECURSIVE`、`ROW_NUMBER`、`SYSTEM`、`WINDOW`。现有列名均无冲突（实测 17 表建成） |
| `int(11)` 显示宽度 | 8.0.17+ 起废弃，仅告警不报错。**建议迁移时统一清理为 `int`** |
| `utf8mb4_unicode_ci` | 8.0 仍可用（UCA 5.2.0）。**不强制改 `utf8mb4_0900_ai_ci`** —— 改排序规则会改变索引顺序与比较结果，可能影响唯一索引行为（`r_name` / `r_display_name` 唯一性）与 `LIKE` 语义。保持 `utf8mb4_unicode_ci` 是零风险选择 |
| `ONLY_FULL_GROUP_BY` | 5.7 已在用，8.0 继续默认开启 → 现有查询行为不变 |
| 零日期 | 见 §6.2。**保留**，通过放开 sql_mode 实现，而非改写数据 |
| 新增表 | `cc_sessions`（会话）、`cc_events`（事件队列，如需）按同样的 `utf8mb4_unicode_ci` + InnoDB 建 |

---

## 7. 灰度与切换路径

### 7.1 七阶段

| 阶段 | 内容 | 出口判据 |
|---|---|---|
| **P0 契约固化** | 对现有 PHP 后端跑一遍全部 108 端点 + 4 内部端点 + 5 页面，固化 JSON fixture 与截图基线 | fixture 覆盖率 100%，含正常/异常/权限三类 |
| **P1 基础设施** | Go 项目骨架：config / contract / envelope / errorcode / 日志 / panic recovery / DB 连接 / 迁移器 | 单元测试通过，健康检查可用 |
| **P2 只读影子** | Go 后端以只读模式挂在 nginx 后，对 GET 类端点做**影子比对**（同请求同时打两边，比对响应） | 影子流量差异率 < 0.1%（已知缺陷项单独标注） |
| **P3 按域切流** | 切 `user` → `group` → `repository` → `admin`（每域单独切，逐步扩大） | 每域切后 24h 无回归；契约测试全绿 |
| **P4 Git 鉴权切流** | 切 `/apiv1/internal/authHTTP` 与 `authSSH` 到 Go | Git over HTTP/SSH 的 clone/push/保护分支拒绝全部通过 |
| **P5 钩子切流** | 切 `/apiv1/internal/hooks` | post-receive 事件入库、pre-receive 保护分支拒绝文案一致 |
| **P6 PHP 下线** | 移除 php-fpm 与 `application/`、`system/`、`www/index.php`、composerlib | 镜像瘦身、依赖收敛；保留回滚包 |

### 7.2 切流与回滚

- **nginx upstream 粒度切换**：`upstream codefever_api { server 127.0.0.1:9001; }`（Go） vs `:9000`（PHP-FPM）。改一行 + `nginx -s reload` 即完成切流/回滚。
- **数据库保持单写**：切流期间**同一时刻只有一个后端在写**（避免双写冲突）。影子阶段只读，不写。
- **回滚**：nginx upstream 切回 PHP-FPM（< 5 秒）。**数据库在 P3 之前不得有破坏性变更**（如列类型变更）；P4 之后若已改写数据，回滚需先做数据回填。
- **会话切换的代价**：Go 后端不认 CI2 加密 Cookie → 切换瞬间**所有在线用户需重新登录一次**。
  - **推荐方案 A（默认）**：在维护窗口切换，公告"需重新登录"。实现成本 ≈ 0。
  - **备选方案 B**：在 Go 中实现 CI2 会话解码（需复刻 `mcrypt` + `encryption_key` 派生 + `base64(serialize())` 解析）→ 用户无感。成本约 3–5 人日，且需精确复刻 CI 2.1.1 的 `Session::sess_create`/`getEncrypt` 行为（mcrypt 的 DES 或 AES-128 取决于配置）。
  - **不推荐方案 C**：双会话并存（PHP 写、Go 读）→ 需要 session 共享存储，复杂度与收益不匹配。

### 7.3 灰度期双写？

**不做通用双写**。理由：本项目的写操作强耦合 git 磁盘状态与 DB，双写会引入"两边都成功但状态不一致"的新风险。改用**分区切流**（按业务域整体切换）已足够。

---

## 8. 测试策略

### 8.1 契约快照测试（**第一交付物**）

对现有 PHP 后端采集 108+4 个端点，固化 `testdata/fixtures/<端点>.json`，包含：
- 正常响应（脱敏：密码哈希、token、secret、绝对路径）
- 典型错误响应（缺参 `0x0201`、无权限 `0x0106`、未登录 `0x0101`、不存在 `0x0302`、端点不存在 `0x0105`）
- 分页、边界（空列表、超长 keyword、特殊字符）

新后端跑同一套请求 → 与 fixture 做**结构化 diff**（忽略 `request-id`、时间戳、绝对路径）。这就是"前端零改动"的可执行证明。

### 8.2 单元测试（补齐现有 0 覆盖）

| 模块 | 必测 |
|---|---|
| `domain/access.go` | 39 权限码 × 6 角色的矩阵快照；角色继承链；`REPO_ACTION_MAPPING` |
| `domain/protected_branch.go` | 正则/精确匹配；push/merge 双最低角色；`role < DEVELOPER` 一律保护 |
| `domain/notification.go` | 位掩码语义（位=1 关闭）；受众阈值；模板占位符替换 |
| `contract/errorcode.go` | 31 个码 → HTTP 状态映射快照 |
| `contract/parse.go` | 4 种 Content-Type × 3 种参数来源；`_SAFE`/`_BINARY` 后缀；`key[]` 与 `key[subkey]` |
| `auth` | argon2id + 旧 md5 双轨；升级路径 |
| `gitsvc` | argv 构造（含 `--` 防注入、路径含空格与中文） |
| `repo` | 关键 SQL（事务回滚、唯一约束、并发建 MR 不撞号） |

### 8.3 集成测试

- 起 MySQL 8 容器 + Go API，跑全部端点的冒烟。
- Git 全链路：建仓 → HTTP clone → commit → push → 钩子事件入库 → 保护分支拒绝 → SSH clone/push。
- 页面视觉回归：5 个服务端渲染页 + 5 个 SPA 关键页的截图 diff。

### 8.4 安全测试

- 越权用例：非成员读 `commitDetail` / webhook 日志 → 必须 `0x0106`。
- 命令注入用例：commit keyword = `'; rm -rf / #` → 必须被当作字面量。
- SQL 注入用例：keyword 含 `'` / `%` / `_` → 正常返回且不改变语义。
- 路径穿越：`/file/fetch/avatar/../../etc/passwd` → 拒绝。

---

## 9. UI 一致性保障机制（用户硬约束）

### 9.1 一致性从哪来

| 层 | 机制 | 是否需要改动 |
|---|---|---|
| SPA 视觉 | `www/view/build` 构建产物**原样**部署；`www/app` 符号链接不变 | ❌ 不改 |
| 设计系统 | `@pgyer/mui-theme-preset` 在 bundle 内，运行期由前端注入 | ❌ 不改，且**禁止重装前端依赖** |
| 主题变量 | `theme1.js:8` 主色 `#3455db`、字体栈、Paper 阴影、Tab 选中态字重 | ❌ 不改 |
| 静态资源 | `/static/vendor/*`、`/static/images/*` 由 nginx 从 `www/static/` 托管 | ❌ 不改 |
| SPA 路由 | react-router 处理，nginx `try_files $uri /app/index.html` 回退 | ❌ 不改 |
| **服务端渲染页** | 5 个页面 + 4 个公共模板由 PHP → Go 移植 | ⚠️ **必须逐字保持 DOM** |
| **API 契约** | 108+4 端点的 URL/参数/信封/错误码 | ⚠️ **必须逐字保持** |
| 头像/图片 | `/file/fetch/avatar/<key>` 与 `/static/<ver>/images/*` | ⚠️ 路径与响应形态保持 |

### 9.2 可执行的验收清单

**A. 静态资源断言**（全部 200）
```
/static/js/main.e442a514.js   /static/css/1.75563d44.chunk.css
/static/js/1.1be53246.chunk.js /favicon.ico  /manifest.json
/static/images/logo-community.png  /static/images/logo-ico.png
/static/images/default_avatar.png  /static/00000000000000/images/logo-community.png
/static/vendor/bootstrap-4.4.1/css/bootstrap.min.css
/static/vendor/fontawesome-free-5.13.0/css/all.css
/static/vendor/toastr/toastr.css
```

**B. 页面像素回归**（浏览器截图 diff，PHP 基线 vs Go 新版）
| 页面 | 覆盖状态 |
|---|---|
| `/user/login` | 默认态 |
| `/user/login` | 错误提示态（密码错误） |
| `/user/register` | 默认态 |
| `/user/resetPassword` | 默认态 |
| `/user/verify` | 2FA 输入态 |
| `/repositories` | 空列表 + 有数据 |
| `/{group}/{repo}` | 文件树 + README |
| `/settings/sshkey` | 列表态 |
| `/admin/dashboard` | 统计卡态 |

**C. API 契约断言**
- 108 端点 × 正常/异常响应 = 结构化 diff 全绿。
- 错误码数值表全绿。
- 登录页信封 `{code,message,extra}` 全绿。

**D. 行为回归断言**
- 登录 → 落地 `/repositories`。
- `code=0x0101` → 前端 3 秒跳 `/user/login`。
- Git HTTP/SSH clone + push + 保护分支拒绝文案逐字一致。

### 9.3 唯一真实风险

`@pgyer/mui-theme-preset: "latest"`。**纪律**：重写期间禁止 `yarn install` / `npm install` / 更新 `package-lock`；`www/view/build` 作为**不可变制品**对待（建议为其打 `git tag` 或记录 SHA256 清单）。

---

## 10. 风险清单

| # | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R1 | 服务端渲染 5 页的 DOM 漂移导致 UI 不一致 | 中 | 高 | 逐字移植 + 截图 diff + 不改 CSS 引用 |
| R2 | 契约某端点语义被误改（尤其错误码或必填参数） | 中 | 高 | 契约快照 + 影子比对 + 逐域切流 |
| R3 | git 行为与原实现有细微差异（输出格式、退出码语义） | 中 | 高 | 保留 `--pretty` 协议；git 层写**差分测试**：同一仓库用 PHP 与 Go 各跑一遍，比对输出 |
| R4 | 事务/并发改造引入新缺陷（MR ID 撞号、fork 孤儿） | 中 | 中 | 显式事务 + 唯一索引 + 并发用例测试 |
| R5 | MySQL 8 升级引入排序规则/类型差异 | 低 | 中 | 保持 `utf8mb4_unicode_ci`；不改列类型；升库后跑 PHP 全链路回归 |
| R6 | 切流瞬间全员掉线（会话不兼容） | 高 | 低 | 维护窗口 + 公告；备选方案 B 实现 CI2 cookie 解码 |
| R7 | 邮件从系统 `mail` 改为 SMTP 后行为差异（发不出/被拒） | 中 | 中 | 保留 `mail` 作为 fallback；SMTP 配置化 |
| R8 | 工作量低估（尤其 repository 域 52 端点含大量 git 语义） | 中 | 中 | 按域分里程碑；`repository` 域单独排期 |
| R9 | 前端依赖被误升级 | 低 | 高 | 冻结 `www/view`；CI 加校验（package.json/yarn.lock 哈希） |
| R10 | 新增 `cf_sessions`/`cf_events` 表与既有 `cc_*` 风格不一致 | 低 | 低 | 沿用 `utf8mb4_unicode_ci` + InnoDB + 32 位 hex 主键 |

---

## 11. 工作量估算与里程碑

**假设**：1 名熟悉本代码库的资深 Go 工程师全职，或 2 人并行。

| 工作包 | 内容 | 人周 |
|---|---|---|
| W1 | 契约快照采集工具 + fixture（108+4 端点） | 2.0 |
| W2 | Go 骨架：config / contract（信封+错误码+解析）/ 日志 / panic recovery / 迁移器 | 2.0 |
| W3 | 认证：会话存储 / argon2id 双轨 / TOTP / admin 中间件 | 1.5 |
| W4 | `user` 域 24 端点 | 2.0 |
| W5 | `group` 域 13 端点 | 1.5 |
| W6 | `repository` 域 52 端点（含 git 层重写） | 5.0 |
| W7 | `admin` 域 19 端点 | 1.5 |
| W8 | `internal` 域 4 端点 | 1.0 |
| W9 | 5 个服务端渲染页 + 4 个模板移植 | 1.5 |
| W10 | 事件/通知/Webhook 异步化 + worker | 2.5 |
| W11 | MySQL 8 迁移落地（升库 + PHP 回归） | 1.5 |
| W12 | 影子比对 / 集成测试 / 安全测试 / 灰度切流 | 3.0 |
| W13 | 页面视觉回归 + 契约全绿 + 文档 | 1.0 |
| — | **合计** | **≈ 25 人周** |

- 单人：**约 6 个月**（含 20% 缓冲 ≈ 7 个月）。
- 双人并行（W6/W9 有天然串行依赖）：**约 3.5 个月**。

### 里程碑

| 里程碑 | 覆盖 | 可见成果 |
|---|---|---|
| **M1 契约基线** | W1 | 108+4 端点 fixture + 5 页截图基线；可回归的"旧系统真相" |
| **M2 Go 内核** | W2 + W3 | Go 服务能启动、能登录、会话可用、错误码表一致 |
| **M3 业务域全覆盖** | W4–W8 | 108 端点全部实现，契约测试全绿 |
| **M4 UI 对齐** | W9 + W13 | 5 个服务端渲染页与 SPA 页面截图 diff 全绿 |
| **M5 异步化与升库** | W10 + W11 | worker 生效；MySQL 8 上 PHP 与 Go 双跑均通过 |
| **M6 灰度与下线** | W12 | 逐域切流完成，PHP 下线，回滚方案验证通过 |

---

## 12. 立即可执行的第一步（建议）

按 W1 开工，产出三样东西：

1. **契约采集器**：一个 Go 或 Python 脚本，对着运行中的 PHP 后端遍历 108+4 端点，覆盖正常/异常/权限三类请求，落 `testdata/fixtures/`。
2. **页面基线截图**：用内置浏览器对 5 个服务端渲染页 + 5 个 SPA 关键页截图，落 `testdata/screenshots/php-baseline/`。
3. **契约清单文档**：`docs/API_CONTRACT.md`（由本次分析产出物整理而来）—— 逐端点列出 URL、方法、参数来源、必填/可选、鉴权等级、响应字段、错误码。

这三样是整个方案的**验收基础**，也是"前端 UI 风格保持一致"从口头约束变成可执行断言的前提。

---

## 附录 A：环境与验证记录（本方案所依据的实测）

| 项 | 结果 |
|---|---|
| 运行栈 | `codefever-app`（`codefever-community:local`）Up 10h；`codefever-db`（mysql:5.7.31）healthy |
| Web UI | `/user/login` 200（8471 bytes，**PHP 服务端渲染**，非 SPA） |
| SPA | `/app/index.html` 200；`/static/js/main.e442a514.js` 200（568,642 bytes） |
| 登录 | `{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}` |
| Git | HTTP/SSH clone+push 成功；pre-receive·update·post-receive 回调均 Success |
| MySQL 8.0.46 | 三处阻塞实测复现（`ERROR 1231` / `ERROR 1067` / `ERROR 1292`）；**3 步修复后 17 表 + 种子全部建成** |
| 浏览器 UI | 登录/仓库列表/仓库详情/SSH Keys/管理后台 5 页取证通过（登录由用户接管完成） |

## 附录 B：新后端上线自检清单（逐条勾选）

- [ ] `/api/*` 107 个唯一 URL 全部实现，形状与 HTTP 方法一致
- [ ] 4 个 `/apiv1/internal/*` 端点请求/响应字段逐字一致
- [ ] 响应信封字段顺序与拼写一致（`request-id`/`code`/`message`/`data`）
- [ ] 31 个错误码数值与 HTTP 状态映射一致
- [ ] `Accept` / `Content-Type` / `Content-Length` 的拒绝行为一致
- [ ] multipart 的 `key[]` 与 `key[subkey]` 解析一致；`_SAFE`/`_BINARY` 后缀剥离一致
- [ ] Cookie 名 `codefever_community`，10 天有效期，`HttpOnly` + `SameSite=Lax`
- [ ] `uid` 格式 `"<u_key>|<r_key>"`；`PGYER_UID` 传递链不断
- [ ] "仓库不存在" 返回 404 / `0x0302`（Go 网关据此区分 401 与 404）
- [ ] `env.yaml` / `config.yaml` 键结构与 Go 网关读取路径未变
- [ ] `git-storage/<32-hex>` 布局、bare 仓库、`hooks → ../../misc/hooks` 软链未变
- [ ] `/file/fetch/avatar/<key>` 返回二进制，路径穿越被拒
- [ ] 5 个服务端渲染页 DOM 与 CSS 引用逐字未变（截图 diff 全绿）
- [ ] SPA 产物哈希未变（`www/view/build` 未重新构建）
- [ ] 仓库名含连字符可正常 HTTP 访问（nginx/Go/PHP 三端正则已统一）
