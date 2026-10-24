# CodeFever Next —— API 契约回归基线

本目录是"后端整体重写为 Go、前端零改动"的**可执行契约基线**：
把现有 PHP 后端每一个对外可见的行为（端点形状、参数来源、响应信封、错误码、
HTTP 状态映射）固化下来，作为 Go 重写后的逐条比对依据。

> **关于本文档里的 PHP 路径引用**：二次开源时 PHP 源码已从仓库中删除，
> 因此下文出现的 `application/controllers/api/user.php`、`www/index.php:21`
> 等路径是**历史引用**，用于说明契约的来源与由来，不再指向仓库内的文件。
> 契约的行为基准由 `backend/testdata/` 下的基线文件承载。

> **关于站点品牌名**：二次开源时站点名由 `CodeFever Community` 改为 `CodeFever Next`，
> 因此 `admin.config` 的 `email.name` 与页面 `<meta name="description">` 两处
> **有意**与 PHP 基线不同。其余位置保持逐字节一致。详见
> [`backend/testdata/README.md`](../testdata/README.md)。

三个组成部分：

| 文件 | 作用 |
| --- | --- |
| `manifest.yaml` | 端点清单（权威描述）+ 采集安全约束 |
| `README.md` | 本文件：字段含义、运行方式、归一化规则、副作用规避策略、事故记录 |

采集结果落在 `backend/testdata/contract/<id>.json`，一个用例一个文件。

> 基线不携带任何上传用示例文件：三个头像上传端点一律走"缺参"失败分支，
> 不发送任何文件，避免覆盖 `u_avatar` / `g_avatar` / `r_avatar`。

---

## 1. 怎么跑

工作目录固定为 `backend/`：

```bash
cd backend

go build ./...                      # 编译校验
go run ./cmd/contract-collect       # 全量采集 → testdata/contract-verify/
go run ./cmd/contract-collect -verbose            # 逐条打印
go run ./cmd/contract-collect -only user.info     # 只跑一条（调试用）
go run ./cmd/contract-collect -base http://localhost:9080 \
                         -manifest contract/manifest.yaml \
                         -out testdata/contract-verify
```

前置条件：一个可登录的 CodeFever 实例（Go 后端）在 `http://localhost:9080` 可用，
管理员账号可登录。`testdata/contract/` 是**只读基线**，不要用采集器覆盖它；
默认输出目录已改为 `testdata/contract-verify/`。

采集器只读业务数据；唯一的副作用是登录会在 `cc_sessions` 表写一行会话记录
（Go 后端使用服务端会话）。如在意，可在采集结束后调用 `/user/logout` 或清理该行。

采集结束后必须确认被测系统仍然健康（见 §6）。

---

## 2. `manifest.yaml` 字段含义

### 顶层

| 字段 | 说明 |
| --- | --- |
| `base_url` | 被测服务根地址 |
| `login.path` / `login.form` | 登录端点与表单字段。PHP 侧 `/user/login` **只认** `application/x-www-form-urlencoded`；响应体是 JSON，但响应的 `Content-Type` 是 `text/html; charset=UTF-8`（PHP 既有行为，不是 JSON） |
| `session.cookie_name` | 会话 Cookie 名（`codefever_community`） |
| `session.key` | CI 的 `$config['encryption_key']`（= env.yaml 的 `session.key`）。仅用于伪造"非管理员"会话，见 §5 |
| `session.non_admin_user_key` | 非管理员账号的 `u_key`（演示库中 `u_admin=0` 的账号） |
| `gateway.token` | 内部网关令牌（= env.yaml 的 `gateway.token`），请求头 `Token:` |
| `safety.*` | 采集安全约束，见 §4 |
| `demo.*` / `invalid.*` | 演示数据与"不存在的资源 key"引用 |

### `endpoints[]`

| 字段 | 说明 |
| --- | --- |
| `id` | 唯一标识，`<controller>.<action>`；错误路径用例用 `error.<controller>.<场景>` |
| `controller` / `action` | PHP 控制器与动作名（动作名与 `api/base.php` 的 `<action>_<METHOD>` 分派规则对应） |
| `method` | `GET` / `POST` |
| `auth` | `web_user`（登录后的普通用户会话）、`admin_web_user`（管理员会话）、`non_admin_user`（伪造的非管理员会话）、`anonymous`（不带 Cookie）、`internal_gateway`（Token 头） |
| `path` | 请求路径 |
| `params[]` | 参数列表 |
| `params[].name` / `.value` | 参数名与取值 |
| `params[].from` | 取值来源：`query`（URL 查询串）、`body`（multipart 表单字段）、`json`（application/json 请求体）、`file`（multipart 文件字段） |
| `expect_error` | 期望的业务错误码（**十进制**）；`null` 表示期望成功 |
| `expect_status` | 期望的 HTTP 状态码；省略时默认 200。`expect_error: null` 且 `expect_status` 非 200 时，表示"期望的是非信封响应"（HTML 404 页 / PHP 致命错误） |
| `expect_content_type` | 期望的响应 Content-Type 前缀（不区分大小写包含匹配） |
| `readonly` | `true` 表示该 POST 端点的成功路径无副作用，允许带参数调用；见 §4 |
| `omit_accept` | `true` 表示不发 `Accept` 头（用于覆盖 0x0103 分支） |
| `bad_token` | `true` 表示用错误的网关令牌 |
| `notes` | 自由说明（写进基线文件旁的清单，不进 JSON） |

### 参数来源为什么这样分

PHP 侧 `Request::parse()` 的分派规则：

* `Accept` 必须是精确的 `application/json`（或 `*/*`），否则 `0x0103`；
* `Content-Type` 决定参数来源：`application/json` → body 解 JSON，
  `multipart/form-data` → `$_POST` + `$_FILES`，`application/x-www-form-urlencoded` → `$_POST`，
  无 `Content-Type` → 只能取 query；
* **头校验先于鉴权**：缺 `Accept` 时，即使未登录也返回 `0x0103` 而不是 `0x0101`。

前端所有写操作用 multipart、所有读操作用 query，内部网关回调用 JSON body。

---

## 3. 响应信封与归一化规则

### 3.1 信封

```
{"request-id":"<大写带连字符 UUID>","code":0,"message":"Success","data":...}
```

* `code` 是错误码的**十进制**值（`0x0201` → `513`）；
* 失败时 `data` 是空数组 `[]`（PHP 空数组的编码结果），**不是 `null`**；
* `Request-ID` 响应头与 body 里的 `request-id` 相同；
* 分页字段 `paging` 在现有代码里从不出现（`setTotal()` 无调用点）。

### 3.2 归一化规则

落盘前把易变字段换成占位符。全部规则都是确定性的：同样的响应永远得到同样的
占位符，因此基线文件可以逐字节 diff（已用连续两次采集验证聚合哈希一致）。

| 编号 | 规则 | 作用范围 |
| --- | --- | --- |
| R1 | `request-id` → `<REQUEST_ID>` | JSON 中 key 为 `request-id` 的字符串值；`Request-ID` 响应头；`raw_body` 里的同一段 UUID 文本。响应头的**格式**另存为 `request_id_format`（实测 `UUID-8-4-4-4-12-UPPERCASE-HEX`），这样既保留格式契约又不让文件每次都变 |
| R2 | `host` → `<HOST>` | JSON 中 key 为 `host` 的字符串值（值形如 `http://localhost`） |
| R3 | 绝对路径 → `<BASE>` | 任意字符串中的 `/data/www/codefever-community` 前缀（含其后紧跟的 `/`）。PHP 的 `json_encode` 会把 `/` 转义成 `\/`，所以 `raw_body` 里同时替换 `\/data\/www\/codefever-community` 这个转义形态 |
| R4 | Unix 时间戳 → `<TS>` | JSON 中取值落在 `[1.7e9, 2.2e9]` 的整数（含端点） |
| R5 | MFA 随机量 → `<MFA_SECRET>` / `<MFA_QR_IMAGE>` | `user.getMFAData` 每次都生成新的 secret 与二维码。`image` 保留 `data:image/svg+xml;charset=utf-8;base64,` 前缀，只替换 base64 载荷 |
| R6 | 实时资源指标 → `<VM_METRICS>` | `admin.systemStatus` 的 `vm` 子树（cpu / 内存 / 磁盘占用每次都不同） |

R1~R4 是契约要求的基础规则；R5、R6 是为了让基线可 diff 而追加的扩展规则。
R5/R6 在 `body`（JSON 树）与 `raw_body`（原始文本）两侧同时生效 —— `raw_body`
侧用 JSON 值扫描替换，而不是脆弱的正则。

### 3.3 输出文件

`backend/testdata/contract/<id>.json`：

```json
{
  "id": "user.info",
  "method": "GET",
  "path": "/api/user/info",
  "auth": "web_user",
  "status": 200,
  "content_type": "application/json",
  "request_id": "<REQUEST_ID>",
  "request_id_format": "UUID-8-4-4-4-12-UPPERCASE-HEX",
  "expect_error": null,
  "expect_status": null,
  "actual_code": 0,
  "match": true,
  "body": { "code": 0, "data": { "host": "<HOST>", ... }, "message": "Success", "request-id": "<REQUEST_ID>" },
  "raw_body": "{\"request-id\":\"<REQUEST_ID>\",...}"
}
```

* `body`：解析后的 JSON（已归一化）；响应不是 JSON（HTML 404 页、PHP 致命错误的空 body）时为 `null`；
* `raw_body`：原始响应文本（已做 R1/R3/R5/R6 的文本级替换），截断到 8KB；
* `expect_error` / `expect_status` / `actual_code` / `match`：把 manifest 的期望与
  实测结果一起固化，`match` 为 `false` 即表示契约已经漂移。

---

## 4. 副作用规避策略

### 4.1 采集器的硬安全约束

`manifest.yaml` 的 `safety` 段 + 采集器代码（`cmd/contract-collect/main.go`）共同强制：

1. **`forbidden_write_paths`**：列出的路径永远不允许收到 POST/PUT/DELETE。
   当前只有 `POST /api/admin/config`，原因见 §6。
2. **写端点只允许"缺参/空值"请求**：POST/PUT/DELETE 且未标记 `readonly: true`
   的端点，采集器只允许发送"所有参数均为空值或缺省"的请求；只要有一个参数
   带非空值，采集器**拒绝发送**该请求并计为"跳过"。这就从机制上保证写端点
   只能走到"缺参 → 0x0201"或"资源不存在 → 0x0407/0x0106"这类不产生写操作的
   失败分支。
3. `readonly: true` 只授予"成功路径也不写库、不改写服务端文件"的 POST 端点：
   `user.getCountriesCode`、`admin.userList`、`admin.groupList`、
   `admin.groupMembers`、`admin.repositoryList`、`admin.repositoryMembers`、
   `admin.checkPassword`、`repository.getWebhook`、`repository.webhooks`、
   `repository.getRepositoryWebhookLogs`、`repository.getRepositoryWebhookLogData`、
   `repository.checkMergeType`。

采集器在每次采集前后都会统计这些约束的触发情况，违反时**中止而不是照发**。

### 4.2 写端点的覆盖方式

所有写端点（create / update / delete / add / remove / assign / merge …）都走
**缺参或资源不存在**的失败分支，并在 manifest 里用 `expect_error` 标注。
覆盖的失败分支：

| 分支 | 错误码 | 典型端点 |
| --- | --- | --- |
| 必填参数缺失 | `0x0201` (513) | 绝大多数写端点 |
| 目标资源不存在 | `0x0407` (1031) | `admin.setGroupOwner`（该端点没有参数校验，`get('')` 查不到用户 → 0x0407，在 `setMemberRole` 之前返回） |
| 权限不足 | `0x0106` (262) | 不再使用（曾用于上传头像，现已全部改为缺参，避免任何文件上传） |

**没有采集任何文件上传**：`user.uploadAvatar` / `group.uploadAvatar` /
`repository.uploadAvatar` 一律不传文件 → `0x0201`。

### 4.3 只能覆盖未登录分支的端点

`user.revokeMFAData` 登录后没有任何参数或权限分支，必定执行
`setMFASecret($u_key, '')`，会改写 `u_2fa` 与 `u_updated`。为了保持数据库只读，
该端点只记录未登录分支（`0x0101`）。

### 4.4 内部网关端点

`/apiv1/internal/*` 四个端点全部只走只读或缺参分支：

* `authHTTP`（GET + JSON body）：校验账号口令与仓库权限，返回仓库存储路径 —— 纯读；
* `authSSH`（GET + JSON body）：按 `userID` + `repo` 查角色 —— 纯读；
* `searchPublicKey`（GET + JSON body）：`fingerprint`/`publicKey` 留空 → `0x0201`；
* `hooks`（POST + JSON body）：**`args` 传空数组** → 直接走 `reject(0x0201)`，
  不进入 hook 处理分支，因此不写 `cc_activities`、不触发 webhook。
  （该端点只有 POST 形态，`hooks_get` 不存在，用 GET 会得到 `0x0105`。）

---

## 5. "非管理员访问后台"是怎么覆盖的

演示库里唯一的非管理员账号 `test2@pgyer.com` 的 `u_status = 4`（已软删除），
`User_model::getInfoByPwd()` 会过滤掉已删除用户，所以它**无法走登录流程**；
新建一个用户又会写业务数据。

CodeIgniter 3 的 Session 库在本项目里把会话整体放进**客户端 Cookie**
（`sess_use_database = FALSE`，`sess_encrypt_cookie = TRUE`），并用
`CI_Encrypt` 的 XOR 方案加密。因此只要知道 `$config['encryption_key']`
（= env.yaml 的 `session.key`，已在 manifest 里），采集器就能在**客户端**构造出
一个属于任意 `u_key` 的会话 Cookie：

```
CI_Encrypt::encode  =  base64( _xor_merge( _xor_encode(明文, md5(key)) , md5(key) ) )
```

采集器用 `session.key` + `session.non_admin_user_key` 复刻这一算法，生成一个
`userInfo.u_key = 26c20714af8bc20aa85be657a5170e71`（`u_admin = 0`）的会话，
再拿它去访问 `/api/admin/systemStatus` → 期望 `0x0101` (257)。

**全程不写数据库**：会话没有服务端存储，也没有产生任何用户记录。

---

## 6. 事故记录：`POST /api/admin/config` 是"单请求砖化全站"的 P0 缺陷

### 6.1 现象

`application/controllers/api/admin.php` 的 `config_post()`：

```php
$config = yaml_parse_file(dirname(APPPATH) . '/config.yaml');
$input  = json_decode($data['data'], TRUE);      // data 缺失 → null
$final  = array_merge($config, $input);           // PHP 7.4：Warning，返回 null
yaml_emit_file(dirname(APPPATH) . '/config.yaml', $final);   // 把 config.yaml 写成空结构
```

`data` 缺失或不可解析时，`array_merge($config, null)` 在 PHP 7.4 只发一条
Warning 并返回 `null`，`yaml_emit_file()` 随即把 `config.yaml` 覆盖成空结构
（`--- ~`）。

之后**每一次请求**都会在 `www/index.php:21` 崩掉：

```php
make_constants('YAML_', yaml_parse_file(dirname(dirname(__FILE__)) . '/config.yaml'));
//                       ^^^^^^^^^^^^^^^ 返回 null
//  → TypeError: Argument 2 passed to make_constants() must be of the type array, null given
```

结果是登录页和全部 API 永久返回 HTTP 500 —— **单次畸形请求即可砖化整个应用**。
放大这个问题的因素：

* 没有文件锁、没有临时文件 + `rename()` 的原子替换；
* 没有 `.bak` 备份，写失败也没有回滚；
* `yaml_emit_file()` 的写入结果直接成为下一次请求的启动依赖。

（本次采集准备阶段的一次探测请求触发了该缺陷，config.yaml 已恢复，
应用已确认健康：登录 200、`GET /api/user/info` 200、数据库各表行数与采集前完全一致。）

### 6.2 新后端必须怎么做

`POST /api/admin/config` 重写时必须：

1. **先校验 payload**：拒绝非对象/不可解析的 `data`，校验失败返回 `0x0201`，
   不触碰任何文件；
2. **原子写入**：先写同目录临时文件（如 `config.yaml.tmp`），`fsync` 后用
   `rename()` 覆盖目标（同文件系统内 `rename` 是原子的）；
3. **保留备份**：写之前把旧文件复制为 `config.yaml.bak`，成功后再清理；
4. **写失败保留原文件**：任何一步失败都必须保持原 `config.yaml` 不变，
   并返回明确的错误码（例如 `0x0405`）；
5. **启动自检**：启动时校验配置文件的解析结果非空且为 map，否则直接拒绝启动，
   而不是让每个请求都以 TypeError 崩掉。

采集器侧的对应措施：`POST /api/admin/config` 已被列入
`safety.forbidden_write_paths`，采集器**永不**对它发写请求；该 action 只保留
`GET /api/admin/config` 一条只读用例。

---

## 7. 覆盖范围与本次采集统计

### 7.1 端点覆盖

| 来源 | 端点数 | 已采集 |
| --- | --- | --- |
| `application/controllers/api/user.php` | 24 | 24 |
| `application/controllers/api/admin.php` | 19 | 18（`POST /api/admin/config` 因 P0 缺陷不采集，见 §6） |
| `application/controllers/api/group.php` | 13 | 13 |
| `application/controllers/api/repository.php` | 52 | 52 |
| `application/controllers/apiv1/internal.php` | 4 | 4 |
| **合计** | **112** | **111** |

清单与 PHP 控制器是逐条比对过的：把四个控制器里所有
`public function <action>_(get|post)` 抽出来，与 manifest 的
`(controller, action, method)` 求差集，**唯一缺失的是
`admin.config.POST`**（有意排除）。

除端点本体，另有 12 条错误路径用例：

| 用例 | 期望 |
| --- | --- |
| `error.user.missingParams` / `error.admin.missingParams` / `error.group.missingParams` / `error.repository.missingParams` | 每个 controller 各 1 条必填参数缺失 → `0x0201` (513) |
| `error.user.unauthenticated` / `error.admin.unauthenticated` / `error.group.unauthenticated` / `error.repository.unauthenticated` | 每个 controller 各 1 条未登录（不带 Cookie）→ `0x0101` (257) |
| `error.admin.nonAdminAccess` | 已登录但非管理员访问 `/api/admin/*` → `0x0101` (257) |
| `error.endpointNotFound` | `GET /api/nope/nope` → HTTP 404 + `Content-Type: text/html`（HTML 404 页，不是 JSON 信封） |
| `error.missingAcceptHeader` | `GET /api/user/info` 不带 `Accept` 头 → HTTP 400 + code 259 (`0x0103`) |
| `error.internal.badToken` | 内部网关 Token 错误 → `0x0101` (257) |

### 7.2 本次采集统计（`go run ./cmd/contract-collect`）

```
用例总数     : 123
成功(code=0) : 38
失败/非 JSON : 85
跳过         : 0
错误码分布   :
      0 (0x0000) : 38
    257 (0x0101) : 7
    259 (0x0103) : 1
    513 (0x0201) : 73
   1029 (0x0405) : 1
   1031 (0x0407) : 1
HTTP 状态分布   :
  200 : 38
  400 : 75
  403 : 7
  404 : 2
  500 : 1
所有用例均与 manifest 期望一致
```

采集前后数据库各表行数完全一致（users 3 / groups 2 / repositories 3 /
activities 5 / ssh_keys 1 / merge_requests 0 / webhooks 0 /
protected_branch_rules 0 / notification_users 0 / commit_emails 0）。

### 7.3 采集中发现的问题（PHP 侧既有缺陷，如实记录，未修）

| 端点 | 现象 | 说明 |
| --- | --- | --- |
| `repository.fileContent` | **HTTP 500**（空 body） | `Repository_model` 里没有 `geFilesByBranchOrSHA1Hash` 方法（方法名拼写错误，`ge-`），调用即致命错误。该端点当前完全不可用 |
| `POST /api/admin/config` | **单请求砖化全站** | 见 §6 |
| `GET /api/admin/config` | 当前返回 200 | 但它读的 `config.yaml` 一旦被上面的 POST 写成空结构，就必然 500 —— 这是 §6 事故的"病灶" |
| `Request::parse()` 分页归一化 | `page` / `pagesize` 走 query 时永远被重置为 `1` / `20` | PHP 用 `is_int($_GET['page'])` 判定，而 query 参数值恒为字符串。这是既有缺陷，本基线原样记录（重写阶段是否修正由比对结果决定） |

### 7.4 PHP 侧其它值得注意的既有行为（重写时要复刻）

* `data` 为空时是 `[]` 而不是 `null`（前端有 truthy 判断）；
* `code` 输出十进制；
* `0x0106`（Action Not Allowed）用于"权限不足"，`0x0101` 用于"未登录/非管理员"；
* 路由不存在时返回的是 **HTML 404 页**（`text/html`），不是 JSON 信封；
* `/user/login` 的响应 `Content-Type` 是 `text/html; charset=UTF-8`，尽管 body 是 JSON；
* `Accept` 必须是精确的 `application/json`：`application/json; charset=utf-8` 会被判为非法（PHP 侧不做 `explode(';')`）。
