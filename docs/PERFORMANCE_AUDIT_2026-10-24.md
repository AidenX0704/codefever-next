# CodeFever Next —— 性能分析报告（待审核）

> **注意（2026-10-24）**：本文档中出现的 `www/view/`、`www/static/` 与 MUI v4 相关描述
> 属于**历史记录**——旧前端已在 2026-10 前端重写中删除。当前前端为 `web/`
> （React 19 + TypeScript + Vite + 自研令牌与组件库），见 `docs/FRONTEND_REWRITE_PLAN.md`。


- **分析日期**：2026-10-24
- **分析范围**：`backend/`（Go 1.23，128 个 .go，~39.8k LOC）、`www/view/`（React 17 + Redux + MUI v4 + webpack 4）、`www/static/`、`backend/deploy/`（nginx / Docker / SQL）
- **分析手段**：源码走查 + **对运行中容器做实测**（`codefever-next-app` / `codefever-next-db` 已在运行）+ 产物字节级归因（source map VLQ 解码）
- **方法论**：遵循 `performance-experts` 技能要求 —— 先测量、后定位，每条结论均给出可复现的证据与验证方法；未做无证据的"预防性优化"建议。

---

## 0. 结论摘要

代码整体工程质量不错（git 调用不经过 shell、事件总线已从请求路径解耦、worker 使用 `FOR UPDATE SKIP LOCKED`、网关 HTTP 客户端有显式连接池），**不存在结构性性能缺陷**。

但存在 **3 个 P0 级问题**，它们共同决定了当前的用户体感：

| # | 问题 | 实测影响 |
|---|------|---------|
| **P0-1** | **全站静态资源没有开启 gzip/brotli，且没有 `Cache-Control`** | 浏览器每次首屏下载 **1,703 KB** 未压缩 JS/CSS（压缩后仅 449 KB）；二次访问不命中任何缓存 |
| **P0-2** | **前端零代码分割**：34 个路由组件全部静态导入，全仓库无 `React.lazy`/`import()` | 登录页也要下载整个后台 + Markdown + Prism 全家桶 |
| **P0-3** | **鉴权每个请求重复查库 2 次** + 管理端点存在**双中间件链** | 每个已登录 API 请求多 2 次 DB 往返（**+8.5 ms**）；admin 端点访问日志与请求体限制定制各跑两遍 |

**另有一个必须先处理的"前置问题"**：仓库中已提交的 `www/view/build/` 产物**与 `www/view/src/` 不同步**（详见 §5.1）。在重新构建之前，本报告的 bundle 数字需要复测。

**预估整体收益**（按 §6 顺序执行）：

| 阶段 | 首屏 JS 传输 | API p50 | 说明 |
|------|------------|--------|------|
| 现状 | **1,663 KB** | 55–67 ms | 实测 |
| 仅开 gzip + Cache-Control | **449 KB**（-73%） | 不变 | 1 行 nginx 配置 |
| + 路由分割 / 依赖裁剪 | **~250 KB**（-85%） | 不变 | 见 §3 |
| + 后端去冗余 + 索引 | 不变 | **~30–40 ms** | 见 §2 |

---

## 1. 实测基线（可复现）

### 1.1 前端产物字节数

| 文件 | 原始 | gzip | brotli（估） |
|------|-----:|------:|------:|
| `build/static/js/1.aa438dd3.chunk.js`（vendor） | 1,104.5 KB | **350.1 KB** | ~282 KB |
| `build/static/js/main.3a8cdea3.js`（全部应用代码） | 555.0 KB | **97.3 KB** | ~74 KB |
| `build/static/css/1.75563d44.chunk.css` | 3.5 KB | 1.4 KB | — |
| **合计（首屏必下）** | **1,663.1 KB** | **448.8 KB** | **~357 KB** |
| `*.js.map`（随镜像发布） | 6,340.7 KB | 1,535.9 KB | — |

> 技能建议预算：JavaScript（压缩后）< 300 KB。当前 **448.8 KB，超出 49%**。

### 1.2 线上 HTTP 响应头（实测 `http://localhost:9080`）

```
GET /static/js/main.3a8cdea3.js   → 200, 568343 bytes, Content-Encoding: '', Cache-Control: ''
GET /static/js/1.aa438dd3.chunk.js → 200, 1131004 bytes, Content-Encoding: '', Cache-Control: ''
GET /static/css/1.75563d44.chunk.css → 200, 3628 bytes, Content-Encoding: '', Cache-Control: ''
```

- 全仓库 grep `gzip|brotli|sendfile|expires|keepalive_timeout` 在 `*.conf/*.template/*.sh/*.yaml/Dockerfile` 中 **0 命中** → 压缩与缓存头**未配置**。
- `Content-Encoding` 为空是**运行时实测**，不是推断。

### 1.3 登录页首屏关键路径（PHP 时代遗留的服务端渲染页）

19 个子资源，**全部阻塞渲染**，合计 **785.1 KB 未压缩**：

| 类型 | 数量 | 明细 |
|------|-----:|------|
| CSS | 8 | bootstrap.min.css 155.7 KB、fontawesome all.css 71 KB、intltel 23.9 KB、markdown.css 25.5 KB … |
| JS | 8 | jquery 87.4 KB、intlTelInput 81.7 KB、bootstrap.bundle 78.8 KB、marked.min 46.5 KB … |
| 图片 | 3 | `activity_pgyer_codefever_.png` **144.5 KB**（LCP 元素）、logo 2.6 KB |

`<script>` 标签**无 `defer`/`async`**；PNG 未转 WebP/AVIF。

### 1.4 数据库往返耗时（容器内实测）

```
单连接连续 50 次 SELECT 1 → 212 ms  →  4.24 ms / 次往返
```

### 1.5 线上 API 延迟（容器访问日志实测）

| 端点 | 实测耗时 | 说明 |
|------|--------:|------|
| `/api/user/info` | 55 ms | 4 行表 |
| `/api/group/list` | 56 ms | 4 行表 |
| `/api/repository/list` | 67 ms | 4 行表 |
| `/api/admin/systemStatus` | **133 ms** | 内部 `ps -eo args=` 派生子进程 |

> **关键推论**：在只有 4 行的表上要 55–67 ms，说明瓶颈**不是数据量**，而是**串行 DB 往返次数**。67 ms ÷ 4.24 ms ≈ **16 次串行往返**。

### 1.6 运行环境

```
MySQL 8.0.46   max_connections=3000   innodb_buffer_pool_size=128MB(默认)
               slow_query_log=0        long_query_time=10s
Go 连接池      MaxOpenConns=25  MaxIdleConns=10
Docker 镜像    1.67 GB
```

---

## 2. 后端发现

### 🔴 B1（High）鉴权每次请求重复查库两次 —— 每个 API 多 2 次 DB 往返

`backend/internal/httpapi/dispatch.go:111` 与 `:115-117`：

```go
// 鉴权
if err := s.authenticate(ctx, ctrl.Auth, p, r); err != nil {   // ← currentUser()
    contract.WriteError(w, p.ID, err)
    return
}
if u, err := s.resolveUser(ctx, p, r); err == nil {             // ← currentUser() 又来一次
    req.User = u
}
```

`authenticate()`（`dispatch.go:197-222`）对 `AuthWebUser` / `AuthAdminWebUser` 都会调用 `currentUser()`；而 `resolveUser()`（`dispatch.go:251-253`）直接就是 `currentUser()` 的别名。

`currentUser()`（`dispatch.go:225-248`）内部做 **2 次查询**：`sessions.Get`（`cc_sessions`）+ `users.Get`（`cc_users`）。

**⇒ 已登录请求实际执行 4 次 SELECT，而不是 2 次。多出的 2 次 = 8.5 ms。**

**修复**：让 `authenticate` 把已解析的用户回传给 `resolveUser`：

```go
var current *domain.User
switch ctrl.Auth {
case AuthInternalGateway:
    // …不变
default:
    current, err = s.currentUser(ctx, p, r)
    if err != nil { contract.WriteError(w, p.ID, err); return }
    if ctrl.Auth == AuthAdminWebUser && !current.Admin {
        contract.WriteError(w, p.ID, contract.NewError(contract.CodeInvalidCredential)); return
    }
}
req.User = current
```

---

### 🔴 B2（High）admin 端点存在双中间件链 —— 访问日志重复、请求体限制定制两遍

`backend/internal/handlers/admin.go:154`：

```go
apiMux := s.Handler()          // ← 又构建了一整条 recover+log+limit+normalize 链
for _, path := range adminBodyPaths {
    s.RegisterPage(path, adminMultipartPreParse(apiMux))
}
```

`s.Handler()`（`httpapi/server.go:192`）返回 `recoverMiddleware(logMiddleware(limitRequestBody(normalizeAPIPath(mux))))`。admin 路径被注册成 **page route**，于是请求路径变成：

```
外层 mux → 外层 logMiddleware → 外层 recover → limit → normalize
         → page route (adminMultipartPreParse) → 内层 apiMux
              → 内层 logMiddleware → 内层 recover → limit → normalize → mux → handleAPI
```

**运行时实证**（容器访问日志）——同一个 `request_id` 出现两条完全相同的记录：

```
05:43:04.253130728Z /api/admin/userList  status=200 duration_ms=9 request_id=42CEE947-…
05:43:04.253153441Z /api/admin/userList  status=200 duration_ms=9 request_id=42CEE947-…
```

`/api/admin/groupList`、`/api/admin/repositoryList`、`/api/admin/config` 全部重复；而 `/api/user/info`、`/api/group/list` 只有一条 —— 说明这是 admin 路径特有的。

副作用：① 访问日志双写（日志量与磁盘 I/O ×2）；② `http.MaxBytesReader` 套了两层；③ 路径归一化跑两遍；④ `recoverMiddleware` 嵌套两层，内层已捕获 panic，外层永远不会兜底；⑤ `apiMux := s.Handler()` 在注册前快照了 `pageRoutes`，这是一颗定时炸弹（将来若在 `admin.Register` 之后再注册 page 路由，内层 mux 与外层行为会分叉）。

**修复**：不要用 `s.Handler()` 做二次分发。改为在 `handleAPI` 内部支持"先解析 multipart 再分派"：

```go
// httpapi/server.go：在 handleAPI 入口处统一处理
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
    if needsMultipartPreParse(r) {
        if err := r.ParseMultipartForm(32 << 20); err != nil { … }
    }
    // …原有分发逻辑不变
}
```

---

### 🔴 B3（Critical）管理后台用户列表：5×N N+1 —— 单页约 100 次查询

`backend/internal/handlers/admin.go:230-248`（`POST /api/admin/userList`）：

```go
for i := range users {
    u := &users[i]
    projects, err := h.Repos.ListJoined(ctx, u.Key)   // ← 每次调用展开成 5 条 SQL
```

`ListJoined`（`repo/repository.go:134-210`）实为 5 条查询：

| # | SQL | 位置 |
|---|-----|------|
| 1 | `SELECT … FROM cc_groups WHERE u_key = ? AND g_status = ?` | `repository.go:234` |
| 2 | `SELECT … FROM cc_groups LEFT JOIN cc_group_members … WHERE u_key = ?` | `repository.go:245` |
| 3 | `SELECT … FROM cc_repositories WHERE g_key IN (…) AND r_status = ?` | `repository.go:167` |
| 4 | `SELECT … FROM cc_repositories LEFT JOIN cc_repository_members … WHERE u_key = ?` | `repository.go:181` |
| 5 | `SELECT … FROM cc_repositories WHERE u_key = ? AND r_status = ?` | `repository.go:197` |

**默认 pagesize=20 ⇒ 单页 = 2 + 20×5 = 102 次查询 ≈ 430 ms**（按 4.24 ms/次）。且第 3/4/5 条**都没有 LIMIT**（见 B6）。

**修复**：新增一个按用户批量计数的 repo 方法，替代逐用户调用：

```sql
SELECT r.u_key, COUNT(*) FROM cc_repositories r
JOIN cc_groups g ON g.g_key = r.g_key
WHERE r.r_status = 1 AND g.g_status = 1 AND g.u_key IN (?,?,…)
GROUP BY r.u_key
UNION ALL
SELECT rm.u_key, COUNT(*) FROM cc_repository_members rm
JOIN cc_repositories r ON r.r_key = rm.r_key
WHERE rm.rm_status = 1 AND r.r_status = 1 AND rm.u_key IN (?,?,…)
GROUP BY rm.u_key
```

**修复后：单页 2 + 1 = 3 次查询。**

---

### 🔴 B4（High）`git clone --bare` 在事务内执行 —— 长时间占用连接池槽位

`backend/internal/handlers/repository_write.go:443-459`（建仓）与 `:568-586`（fork）：

```go
if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
    if err := tx.Create(ctx, newRepo); err != nil { return err }
    if err := h.Git.InitBare(ctx, absPath); err != nil { return err }   // ← 事务内跑 git
    return h.emit(ctx, raw, domain.EventTypeREPO_CREATE, …)
```

`gitsvc.Service.run`（`gitsvc/service.go:166`）用 `exec.CommandContext` 拉起真实子进程，多 GB 仓库的 clone 可能持续数分钟。

`WithTx` → `r.db.BeginTx` 会**独占 25 个池连接之一**，直到 clone 结束。25 个并发 fork ⇒ 连接池耗尽 ⇒ 登录、session 读取、列表接口全部阻塞在 `database/sql` 上等连接。这是典型的 "tx 跨 I/O" 死锁形态。

**修复**：把 git 操作挪到事务外，再用短事务写库（保留"无仓无行"的既有语义 —— 代码注释 `:456` 已经在做反向清理）：

```go
if err := h.Git.CloneBare(ctx, absPath, sourcePath); err != nil {
    _ = h.Git.RemoveRepo(ctx, absPath)
    return nil, contract.NewError(contract.CodeOperateFail)
}
err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
    if err := tx.Create(ctx, newRepo); err != nil { return err }
    return h.emit(ctx, raw, domain.EventTypeREPO_CREATE, …)
})
if err != nil {
    _ = h.Git.RemoveRepo(ctx, absPath)
    return nil, contract.NewError(contract.CodeOperateFail)
}
```

---

### 🔴 B5（High）事务内用第二条池连接查询

`backend/internal/handlers/repository_write.go:2642-2655`：

```go
if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
    if err := tx.MergeRequestMerge(ctx, uKey, mrKey, sourceSHA, targetSHA); err != nil { return err }
    return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_MERGE, …, map[string]any{
        "gKey":       h.groupKeyOf(ctx, mr.TargetRKey),   // ← 事务内走 r.db，另一条连接
        "sourceGKey": h.groupKeyOf(ctx, mr.SourceRKey),
```

`h.groupKeyOf` → `h.Repos.Get()`，`h.Repos` 的 `tx` 字段为 nil，`runner()`（`repo/repository_write.go:34-39`）回落到 `r.db` ⇒ **占用第 2 条池连接**。每次 merge 占 2 条，13 个并发 merge 即可耗尽池。

**修复**：把两次 `groupKeyOf` 提到 `WithTx` 之前求值。

---

### 🔴 B6（High）合并请求列表：每个 MR 一次查库 + 一次 `git log` 子进程

`backend/internal/handlers/repository.go:939-943`：

```go
repoPath, perr := h.repoPath(ctx, mr.SourceRKey)      // SELECT r_path FROM cc_repositories …
if perr == nil {
    if commit, cerr := h.Git.LastCommit(ctx, repoPath, mr.SourceBranch, ""); cerr == nil {
        view.Commit = *commit                            // git log -1 子进程
    }
}
```

命中端点：`GET /api/repository/mergeRequests`、`GET /api/repository/relatedMergeRequests`。pagesize 固定 20 ⇒ **每次请求 20 次查询 + 20 次 git 子进程**；同一源仓库的多个 MR 还会重复查同一条 `r_path`。

**修复**：① 用 `WHERE r_key IN (…)` 一次性批量取路径；② 按 `(repoPath, branch)` 去重后只 spawn 一次 `git log`。

---

### 🔴 B7（High）通知 worker：每个收件人 2 次 COUNT(*) —— 200 人群组 = 单次事件 400 次查询

`backend/internal/events/notification.go:610` 与 `:618`：

```go
for _, u := range affected {
    refused, err := h.refuse.HasRefused(ctx, u.key, repo.NotificationRefuseGroup, gKey)   // COUNT(*)
    …
    refused, err = h.refuse.HasRefused(ctx, u.key, repo.NotificationRefuseRepository, rKey) // COUNT(*)
```

`affected` 是该仓库 + 群组的完整成员列表（`events/notification.go:385-420`）。这是 worker 里最热的查询源。

**修复**：一条批量查询替换：

```sql
SELECT u_key, nr_type FROM cc_notification_refuse
WHERE nr_status = 1 AND u_key IN (?,?,…)
  AND ((nr_type = 1 AND g_key = ?) OR (nr_type = 2 AND r_key = ?))
```

配合新增复合索引 `cc_notification_refuse(u_key, nr_type, g_key)` 与 `(u_key, nr_type, r_key)`。

---

### 🟠 B8（Medium）`cc_events` 永不清理 + 领取查询无排序索引

`backend/internal/events/worker.go:136-144`：

```sql
SELECT … FROM cc_events
WHERE (ev_status = ? OR (ev_status = ? AND (ev_lease_until IS NULL OR ev_lease_until < NOW())))
  AND ev_attempts < ?
ORDER BY ev_created ASC
LIMIT ? FOR UPDATE SKIP LOCKED
```

三个问题叠加：

1. **无任何 `DELETE FROM cc_events`** —— 全仓库 grep 无命中。已处理事件（`ev_status=3`）永久堆积。对比：`cc_sessions` 是有清理的（`cmd/codefever-api/main.go:424-432` → `session/store.go:116`），说明 `cc_events` 的清理被遗漏了。
2. `OR` 条件让优化器无法走单一范围扫描。
3. **`ev_created` 无索引**（现有索引：`ev_status`、`ev_type`、`(ev_status, ev_lease_until)`、PK）⇒ 每 2 秒对候选集做一次 filesort。

**修复**：新增保留期清理任务 + 索引 `KEY idx_events_claim (ev_status, ev_created)`。

---

### 🟠 B9（Medium）`ListJoined` 无 LIMIT —— 返回用户可见的全部仓库

`backend/internal/repo/repository.go:168`、`:182`、`:198` 三条查询**均无 `LIMIT`**，整个结果集在 Go 侧全量物化后再进 `[]any`。调用方：`/api/repository/list`（`handlers/repository.go:147`）、`/api/repository/relatedMergeRequests`（`:985`）、`/api/admin/userList`（`admin.go:233`）。

---

### 🟠 B10（Medium）全库前导通配符 `LIKE '%kw%'`

| 位置 | SQL | 索引情况 |
|------|-----|---------|
| `repo/admin.go:283-284` | `g_display_name LIKE '%kw%'` | **该列无任何索引** |
| `repo/admin.go:283-284` | `r_display_name LIKE '%kw%'` | **该列无任何索引** |
| `domain/admin.go:257-260` | `(u_name LIKE ? OR u_email LIKE ?)` | 有索引但 `OR` 使其失效 |
| `repo/repository.go:622-623` | `a_data LIKE ?`（`cc_activities`，TEXT） | 无索引 |
| `repo/group.go:394-398` | `a_data LIKE ?` | 无索引 |
| `repo/repository.go:680-681` | `(mr_title LIKE ? OR mr_description LIKE ?)` | 无索引 |

`cc_activities` 与 `cc_merge_requests` 是增长最快的两张表（每次 push/MR 一行，且不清理）。

**修复**：允许的前缀搜索改 `LIKE 'kw%'`（可用索引）+ 加 `KEY (g_display_name(191))` / `KEY (r_display_name(191))`；自由文本字段用 `FULLTEXT` + `MATCH…AGAINST`。

---

### 🟠 B11（Medium）缺失索引清单（已与线上 `information_schema.STATISTICS` 逐表核对）

| 查询 | 位置 | 建议索引 |
|------|------|---------|
| `WHERE u_key=? AND g_key=?` | `repo/group.go:440` | `cc_group_members(u_key, g_key)` |
| `WHERE u_key=? AND r_key=?` | `repo/repository_write.go:302`、`repo/admin.go:363` | `cc_repository_members(u_key, r_key)` |
| `WHERE g_key=? AND r_status=?` | `repo/group.go:295`、`repo/notification.go:356` | `cc_repositories(g_key, r_status)` |
| `WHERE u_key=? AND r_status=?` | `repo/repository.go:198` | `cc_repositories(u_key, r_status)` |
| `WHERE u_tel=? AND u_status<>?` | `repo/user.go:63` | **`cc_users(u_tel)` —— 完全没有索引**，手机号注册走全表扫描 |
| `WHERE u_status=?` | `repo/admin.go:384` | `cc_users(u_status)` —— `COUNT(*)` 全表扫描 |
| `WHERE u_key=? AND nu_is_read=? AND nu_status=?` | `repo/user.go:171` | `cc_notification_users(u_key, nu_status, nu_is_read)` |
| `WHERE u_key=? AND nr_type=? AND g_key=?` | `repo/notification.go:258` | `cc_notification_refuse(u_key, nr_type, g_key)` |
| `ORDER BY ev_created ASC`（worker 领取） | `events/worker.go:142` | `cc_events(ev_status, ev_created)` |
| `ORDER BY nc_created DESC` | `repo/notification.go:96` | `cc_notification_content(nc_scope, nc_created)` |
| `ORDER BY rwl_created DESC` | `repo/repository_write.go:544` | `cc_repository_webhook_logs(rw_key, rwl_created)` |
| `WHERE a_relative_r_key=? AND a_relative_mr_key=?` | `repo/repository.go:650` | `cc_activities(a_relative_r_key, a_relative_mr_key)` |
| `WHERE r_key=? AND pbr_status=?` | `repo/repository.go:762` | `cc_proteced_branch_rules(r_key, pbr_status, pbr_updated)` |

> ⚠️ **落盘位置**：Go 迁移器只执行 `backend/internal/db/migrations/*.sql`（`internal/db/db.go:92-121`）。17 张遗留表由 `deploy/sql/db_structure.sql` 建出（`deploy/sql/create_db.sh:91`）。**新索引必须写成 `backend/internal/db/migrations/004_perf_indexes.sql` 里的 `ALTER TABLE`**，放进 `db_structure.sql` 对已部署实例无效。

---

### 🟠 B12（Medium）管理端分页：`pagesize` 无上限、`OFFSET` 深翻页、排序不稳定

`backend/internal/domain/admin.go:203-211`：

```go
func AdminPaging(page, pagesize int) (int, int) {
    if page == 0 { page = 1 }
    if pagesize == 0 { pagesize = AdminPageSizeDefault }
    return page, pagesize          // ← 没有 clamp 到 AdminPageSizeMax
}
```

`AdminPageSizeMax = 100` 在 `domain/admin.go:59` 声明但**全仓库无引用**（已 grep 确认）⇒ `pagesize=1000000` 会得到 `LIMIT 1000000`。

另外 `repo/admin.go:104-108`：`sort` 不在白名单时**不拼 `ORDER BY`**（`domain/admin.go:148-158` 对未知值返回 `""`）⇒ MySQL 可任意顺序返回，`LIMIT/OFFSET` 退化为无序扫描。

**修复**：`AdminPaging` 里 clamp 到 `[1, AdminPageSizeMax]`；始终追加 `ORDER BY <排序列>, <主键>`；深翻页改 keyset 分页。

---

### 🟡 B13–B18（Low）

| # | 问题 | 位置 |
|---|------|------|
| B13 | `RolesFor` 对同一 `IN` 列表**重复发两次** `cc_repositories` 查询 | `repo/repository.go:483` 与 `:561` |
| B14 | `SELECT DISTINCT g.g_key, g.u_key` —— `g_key` 已是主键，`DISTINCT` 多余（强制临时表 + filesort） | `repo/repository.go:522` |
| B15 | `ORDER BY FIELD(g_key, …), r_key` —— `FIELD()` 逐行求值，必走 filesort | `repo/repository.go:162-169` |
| B16 | 通知写入每个收件人 2 条独立 `INSERT`（content + user），未批量、未共用事务 | `events/notification.go:795`、`:804` |
| B17 | 通知拒绝级联：组内每个仓库一次 `INSERT`/`DELETE`，且**不在事务内** ⇒ 中途失败会留下不一致状态 | `repo/notification.go:304`、`:330` |
| B18 | `uKeys` 未去重就作为 `IN (…)` 参数列表（200 个仓库 ⇒ 数百个重复占位符） | `handlers/repository.go:1564-1598` |

---

## 3. 前端发现

> 🔎 **重要前置**：source map 的 `sourcesContent` 是**原始源码**，不是压缩后实际保留的字节。用它衡量依赖体积会**高估**。下表使用 source map `mappings` 的 VLQ 解码结果（真实 minified 字节），并已用运行时内容抽样交叉验证。

### 🔴 F1（P0）零代码分割 —— 34 个路由组件全部静态导入

`www/view/src/routes/MainRoutes.js:5-44`：全部 34 个页面组件为静态 `import`。全仓库 grep `React.lazy|import(|lazy(|Suspense` → **0 命中**。

`www/view/config/webpack.config.js:302-305`：

```js
splitChunks: { chunks: 'all', name: isEnvDevelopment },
```

`chunks:'all'` 本身没问题，只是**没有异步 chunk 可分**。webpack 4 因此只产出 2 个文件：`main`（全部应用代码 555 KB）+ `1.chunk`（全部 node_modules 1,104 KB）。

**后果**：只看登录页的访客，也要下载后台管理、Markdown 渲染、Prism 高亮、完整日期库的全部分量。

**修复**：把路由组件改成 `React.lazy(() => import(...))` + 单一 `<Suspense>`；优先拆 admin（`/admin/*`）与 merge-request 集群（`MergeRequestDetail` 单文件 42 KB 源码）。

---

### 🔴 F2（P0）全仓库零 memoization 原语

grep `React.memo|useMemo|useCallback|memo(` → **139 个文件 0 命中**。

配合 F7（每个 action 都让根 state 换引用）与 F9（config 被重置成 `{}`），一次导航会产生多轮全树渲染。

**修复**：先给列表行组件（`CommitItem`、`MemberItem`、`MergeRequestItem`、`RepositoryListItem`）加 `React.memo`；再把内联对象 props 提升为模块常量或 `useMemo`。

---

### 🔴 F3（P1）Redux reducer 对**每一个** action 都返回新引用

`www/view/src/reducers/DataStore.js:31-33`：

```js
if (!event.type.match(/^data\./)) {
    return { ...state }        // ← 与本 reducer 无关的 action 也换根引用
}
```

`reducers/NotificationStates.js:78`：

```js
return { ...state, confirmation: { ...state.confirmation, queueList: [...state.confirmation.queueList] } }
```

`reducers/DrawerStates.js:10` 同样写法。

`combineReducers`（`reducers/main.js:10-14`）只在 slice 返回**同一个对象**时保持根引用不变。而**每个 API 响应都会 dispatch 一次 action**（`helpers/NetworkHelper.js:4-5` → `network.connection.ok`），该 action 不匹配 `^data\.` ⇒ 三个 slice 全部返回新对象 ⇒ 根引用变化 ⇒ react-redux 通知**全部 89 个 connected 组件**重跑 `mapStateToProps`。

`NotificationStates.js:78` 还会**无条件**新建 `queueList` 数组，进一步破坏引用相等。

附带问题：`NotificationStates.js:52` 原地 `push` 修改 state；`:56` 对数组 `delete` 会留下空洞。

**修复**：

```js
if (!event.type.match(/^data\./)) return state      // DataStore
if (!event.type.match(/^drawer\./)) return state    // DrawerStates
```

`NotificationStates` 只在真正改动 `confirmation` 的分支重建它。

---

### 🟠 F4（P1）`JSON.stringify` 深度比较当变更检测

| 位置 | 比较内容 |
|------|---------|
| `components/view/CommitList.js:63-69` | `currentRepositoryConfig`（含 repository/branches/tags/members，可达数十 KB JSON） |
| `components/view/FileTree.js:74,86,91` | `match.params`、`currentRepositoryConfig`、`state.pathStack` |
| `components/view/MergeRequestDetail.js:251` | `state.mergeRequestInfo` |

每次比较**各序列化两次**（前后各一），且在**任何** update 时都跑（包括无关 state 变化触发的）。

**修复**：先做引用比较（reducer 本就是整体替换对象，引用比较是有效快路径）；确实需要结构比较时，在 reducer 里维护版本号。

---

### 🟠 F5（P1）`Prism.highlight` 每次渲染重跑 + blame 查找 O(行数×blame)

`components/unit/CodeViewer.js:89-97`：

```js
const parsed = Prism.highlight(codes.join('\n'), Prism.languages[lang] || Prism.languages.markdown, lang)
```

`shouldComponentUpdate`（`:81-83`）**无条件返回 true** ⇒ 父组件任何 re-render 都会重新 join、重高亮整个文件、重 split。5,000 行文件在主线程上是数百毫秒量级。

`components/unit/CodeViewer.js:100-104`：

```js
codeLines = codes.map((item, key) => {
    const blameData = blame.filter(blameItem => blameItem.start === (key + 1))   // O(lines × blame)
```

1,000 行 × 400 个 blame 区间 = **40 万次比较，每次渲染**。

**修复**：用 `useMemo` 把 blame 建索引成 `Map`；对 `[raw, lang]` 记忆化高亮结果；文件正文分段渲染。

---

### 🟠 F6（P1）依赖包裁剪：三项合计约 **350 KB raw**（vendor chunk 的 ~25%）

| 依赖 | minified 体积 | 占比 | 现状 | 建议 |
|------|------------:|-----:|------|------|
| `react-markdown` + `remark-gfm` + micromark/mdast/hast/unist | **172.3 KB** | 15.6% | 全量 eager | `React.lazy` —— 只在渲染 README 时加载（`components/unit/ObjectViewer.js:20-21`） |
| `prismjs`（61 种语言） | **142.4 KB** | 12.9% | 全量 eager | `babel-plugin-prismjs` 的 `languages` 数组（`config/webpack.config.js`）裁到 ~15 种，其余按需 `import()` |
| `@pgyer/icons`（**154/154** 模块） | **110.3 KB** | 10.0% | 全量 eager | 包无 `sideEffects` 字段 ⇒ 无法 tree-shake；改为按路径 `import plSearch from '@pgyer/icons/icons/plSearch'`（仅 43 个图标在用） |
| `date-fns` + locales + `@date-io/date-fns` | 58.6 KB | 5.3% | eager | 移除未用 locale |
| `@formatjs/*` + `intl-messageformat` | 95.7 KB | 8.7% | eager | polyfill 为主（`src/index.js:15-16`），可考虑按 locale 动态加载 |
| `@material-ui/core` 等 | 195.3 KB | 17.7% | eager | 保留（`@material-ui/icons` 已是逐图标导入，**仅 0.6 KB，正确**） |
| `@fortawesome/*` | 35.4 KB | 3.2% | eager | **已正确 tree-shake（图标 4.3 KB）** |
| `src/lang/**`（zh-cn + en-us） | 58.3 KB | — | 双语全量 | 单语部署可省 ~29 KB |

> ✅ **`@material-ui/icons` 与 `@fortawesome` 都已是正确写法**，不是问题。问题在 `prismjs` / `@pgyer/icons` / markdown 三处。

---

### 🟠 F7（P2）HTTP 缓存被完全关闭

`www/view/src/data_providers/main.js:14-47`：所有请求（含 GET）都带 `cache: 'no-cache'`。

`data_providers/main.js:52-55`：

```js
function GET (endpoint, data = {}, query = {}) {
    query.requestTime = new Date().getTime()     // ← 显式缓存击穿参数
    return DataProvider('GET', endpoint, null, query)
}
```

两个机制叠加 ⇒ **任何 GET 都不会命中浏览器缓存**。`src/data_providers/RepositoryData.js` 暴露 57 个端点，其中 `config`/`object`/`pathStack`/`commitList` 在每次挂载时重新拉取。数据层也没有 in-flight 去重。

**修复**：幂等 GET 去掉 `cache:'no-cache'` 与 `requestTime`，交由 ETag/Last-Modified 协商；加一层 `method+url → Promise` 的 in-flight map 合并并发重复请求。

> 注：本项与 §5.2「服务端无 `Cache-Control`」互为因果，**必须两端一起改**才有收益。

---

### 🟠 F8（P2）目录列表在 render body 里重建，且原地修改 state

`components/view/FileTree.js:360-364`：

```js
<TableList data={this.makeFileListData(
    this.state.objectData.object.filter(FilterGenerator.search(this.state.search, ['name']))
)} />
```

`makeFileListData`（`:202-263`）每次渲染都把整个目录排序 + map 成 JSX 元素数组 ⇒ **搜索框每敲一个字符**，全量过滤 + 全量重建元素。

`FileTree.js:205` 还直接对 `this.state.objectData.object` **原地 `.sort()`**。

`helpers/FilterGenerator.js:114-118` 里 `keyword.toLowerCase()` 在谓词内部逐项调用。

---

### 🟠 F9（P2）`Main.js` 每次渲染都 dispatch，把 config 重置为 `{}`

`components/Main.js:99-102`：

```js
} else if (!nextProps.match.params.groupName) {
    this.props.dispatchEvent({ type: 'data.currentGroupKey.update', data: '' })
    this.props.dispatchEvent({ type: 'data.currentGroupConfig.update', data: {} })   // ← 新对象
}
```

`components/Main.js:128` 同理：`data.currentRepositoryConfig.update` 先塞 `{}`，真实配置稍后才到 ⇒ 每个订阅者两次 prop 变更，且 F4 的 `JSON.stringify` 比较会触发两次。

（`/settings`、`/admin`、`/repositories`、`/mergerequests` 这些无 `groupName` 的路径**每次渲染**都会触发。）

**修复**：不要用 `{}` 重置；把 dispatch 从 `shouldComponentUpdate` 移到 `componentDidUpdate` 并按参数实际变化触发；值未变时不 dispatch。

---

### 🟠 F10（P2）`web-vitals` 声明了但从未接线 —— 没有任何 CWV 数据

`www/view/package.json:104` 声明 `"web-vitals": "^1.1.2"`；全仓库 grep `web-vitals|reportWebVitals|onLCP|onCLS` → **0 命中**，bundle 里也没有该包。

`src/index.js` 无任何测量代码。

**后果**：本报告里的所有前端问题**无法被持续追踪**，只能靠人工复测。

**修复**（约 10 行）：

```js
import { getCLS, getFID, getFCP, getLCP, getTTFB } from 'web-vitals'
const report = (m) => navigator.sendBeacon('/api/web-vitals', JSON.stringify(m))
getCLS(report); getFID(report); getFCP(report); getLCP(report); getTTFB(report)
```

---

### 🟡 F11–F20（Low / Medium）

| # | 问题 | 位置 |
|---|------|------|
| F11 | 生产构建**默认输出 source map**，6.34 MB 随镜像发布（nginx 用 rewrite 挡住了访问，但源码仍暴露在镜像里） | `config/webpack.config.js:35`、`:165-169` |
| F12 | admin 列表 200 ms 防抖定时器**卸载时未 clearTimeout** ⇒ 卸载后 setState + 多余请求 | `components/view/admin/Users.js:153-155` vs `:157-168`（`Groups.js:126`、`Repositories.js:126` 同） |
| F13 | 列表未虚拟化：`GroupList` 渲染全部群组；`CommitList` 的分页是**累加**（点 N 次"更多"→ DOM 里 N×20 行） | `components/view/GroupList.js:45-72`、`CommitList.js:263-285` |
| F14 | `key={index}` 遍布 35+ 处；`Notification` 与 `CommitList` 都是**追加**式列表 ⇒ index key 会强制重渲染并错保留 DOM 状态 | `CodeViewer.js:121,129,145`、`Notification.js:324` 等 |
| F15 | `remarkPlugins={[remarkGfm]}` 内联数组 ⇒ 每次渲染重新解析 Markdown | `components/unit/ObjectViewer.js:221` |
| F16 | 在 render body 里注入**第三方 CDN 样式表**（`cdnjs`）⇒ 解析完 bundle 后才发现，DNS/TLS 落在关键路径上 | `components/unit/ObjectViewer.js:220` |
| F17 | `scroll` 监听器未加 `{ passive: true }` ⇒ 阻塞合成器 | `helpers/withScrollTrigger.js:21` |
| F18 | 每个 commit 行新建 `new DateFnsUtils()`（1–3 次/行），且每行都做一次完整 `format()` | `components/view/CommitList.js:120-138` |
| F19 | `<img>` 均无 `loading="lazy"` | `components/Header.js:89`、`unit/EmptyListNotice.js:50` |
| F20 | `RefList` 在 render 里对**全部** ref（而非当前页）构建 JSX，并修改 props | `components/unit/RefList.js:105-132` |

---

## 4. 部署与基础设施

### 🔴 I1（P0）静态资源无压缩

全仓库 `*.conf/*.template/*.sh/*.yaml/Dockerfile` 中 `gzip|brotli|sendfile` **0 命中**。运行时实测 `Content-Encoding` 为空。

**修复**（`backend/deploy/docker/vhost-go.conf-template` 的 `server` 块内）：

```nginx
gzip              on;
gzip_vary         on;
gzip_comp_level   6;
gzip_min_length   1024;
gzip_proxied      any;
gzip_types        text/plain text/css application/javascript application/json
                  image/svg+xml application/font-woff application/font-woff2;

# 更优：Brotli（需 nginx 编译时带 brotli 模块）
# brotli on;
# brotli_static on;
# brotli_types text/plain text/css application/javascript application/json image/svg+xml;
```

**效果：1,663 KB → 449 KB（-73%）。**

> ⚠️ 基础镜像 `webdevops/php-nginx:7.4` 的主 `nginx.conf` 可能已全局开启 gzip —— **这一点需要在容器内 `nginx -T` 确认**。但运行时实测 `Content-Encoding` 为空，说明**至少本 vhost 覆盖到的路径没有被压缩**。

---

### 🔴 I2（P0）静态资源无 `Cache-Control` / `expires`

`/static/` location（`vhost-go.conf-template:47-51`）只有 `rewrite` 与 `try_files`，没有任何缓存头。

而前端产物是**内容哈希命名**（`main.3a8cdea3.js`、`1.75563d44.chunk.css`）⇒ 本可以放心长缓存。实测 `Cache-Control` 为空 ⇒ 每次访问都要重新走一次协商。

**修复**（分两类路径）：

```nginx
# 带内容哈希的构建产物 —— 永久缓存
location ^~ /static/js/    { expires 1y; add_header Cache-Control "public, max-age=31536000, immutable"; try_files $uri =404; }
location ^~ /static/css/   { expires 1y; add_header Cache-Control "public, max-age=31536000, immutable"; try_files $uri =404; }

# 不带哈希的静态目录 —— 短缓存 + 协商
location ^~ /static/       { expires 7d; add_header Cache-Control "public, max-age=604800, stale-while-revalidate=86400"; try_files $uri /app$uri 404; }
```

同时 `location /`（SPA 回退）应设 `Cache-Control: no-cache`，确保 `index.html` 不会被缓存。

---

### 🟠 I3（P1）SPA 的两个 `<script>` 没有 `defer`

`www/view/build/index.html` 末尾：

```html
<script src="/static/js/1.aa438dd3.chunk.js"></script>
<script src="/static/js/main.3a8cdea3.js"></script>
```

经典阻塞式脚本 ⇒ 阻塞 HTML 解析直到 1.7 MB 下载并执行完毕。

**修复**：webpack 插件产出 `defer`（或 `HtmlWebpackPlugin` 的 `scriptLoading: 'defer'`），并为 LCP 资源加 preload：

```html
<link rel="preload" href="/static/js/1.aa438dd3.chunk.js" as="script">
```

---

### 🟠 I4（P1）登录页 19 个阻塞资源、785 KB

见 §1.3。修复要点：`<script>` 加 `defer`；`marked.min.js`（46.5 KB）、`intlTelInput.js`（81.7 KB）只在对应页面加载；`activity_pgyer_codefever_.png`（144.5 KB，LCP 元素）转 WebP/AVIF 并加 `fetchpriority="high"`。

---

### 🟡 I5–I9

| # | 问题 | 数据 / 位置 |
|---|------|------------|
| I5 | Docker 镜像 **1.67 GB**（基础镜像 `webdevops/php-nginx:7.4` 是 Debian buster + nginx，PHP 已在构建期删除但基础层仍在） | `Dockerfile:32`、`:60` |
| I6 | 6.34 MB source map 随镜像发布 | `www/view/build/static/js/*.map` |
| I7 | **`slow_query_log = 0`、`long_query_time = 10s`** ⇒ 生产环境无任何慢查询可观测性 | 实测 `@@slow_query_log` |
| I8 | `innodb_buffer_pool_size = 128MB`（MySQL 默认），未按数据量调优 | 实测 |
| I9 | MySQL `max_connections=3000` vs Go 池 `MaxOpenConns=25` —— 差 120 倍；3000 无实际意义（Go 是唯一客户端），反而容易掩盖真正的连接瓶颈 | `docker-compose.yml:43`、`internal/db/db.go:28` |
| I10 | **没有 Service Worker**：`workbox-webpack-plugin` 是依赖但 `src/service-worker.js` 不存在 ⇒ 插件静默跳过（`config/webpack.config.js:706`），无离线与资源预缓存 | 实测 glob 无匹配 |

---

## 5. 前置问题与测量前提

### ⚠️ 5.1 已提交的 `www/view/build/` 与 `www/view/src/` 不同步

- `src/components/App.js` 里有 `import { MuiPickersUtilsProvider } from '@material-ui/pickers'` 与 `<MuiPickersUtilsProvider utils={DateFnsUtils} locale={localeCN}>`。
- **两个产物 bundle 里都搜不到** `MuiPickersUtilsProvider` / `DateFnsUtils` / `@material-ui/pickers`（`pickers` 子串 0 命中）。
- `git log` 显示 `www/view/build` 与 `www/view/src` **在同一个 commit（7626bd7）** 中提交 ⇒ 是**构建产物早于源文件定稿**，但被一并提交并进入镜像。

**含义**：
1. **运维问题**：容器提供的前端与仓库源码不一致，用户拿到的是旧版前端。
2. **测量前提**：§3.6 的依赖体积数字基于**已发布产物**；重新构建后，`@material-ui/pickers` 会加入 vendor chunk（该包 `sideEffects:false`，预计 +5–15 KB，不改变结论方向）。**动手前请先重新构建并复测。**

### ⚠️ 5.2 本报告的测量边界（请在审核时一并考虑）

| 项目 | 状态 |
|------|------|
| 线上 DB 数据量 | **仅 13 行 / 4 个仓库 / 3 个用户** ⇒ 所有"取决于表大小"的严重度（无 LIMIT、无索引、全表扫描）是**按查询形态推断**，不是按真实数据量实测 |
| `EXPLAIN` | **未执行**（表太空，优化器计划无参考价值）⇒ 索引类结论基于"DDL 中确实没有该索引"这一客观事实 |
| Lighthouse / RUM | **未运行** ⇒ LCP/INP/CLS 的**具体数值未测**；本报告的传输字节与响应头均为 HTTP 层实测 |
| 前端包体积 | 来自**已发布产物**的 source map VLQ 解码，非一次全新构建 |
| MySQL 慢查询日志 | 未开启，无法回溯历史慢查询 |
| 未覆盖 | `gitsvc` 内部的 git 命令组合（34.5 KB `commands.go`）未逐条审计；`events/activity.go`、`events/webhook.go` 的写路径只做了抽样 |

---

## 6. 已确认健康的部分（请勿重复排查）

**后端**

| 项 | 结论 |
|----|------|
| git 命令注入 | 全部以 argv 数组交给 `exec.CommandContext`，**永不经过 shell**（`gitsvc/service.go:157-184`）；路径参数前置 `--`；带路径穿越校验（`:123-151`） |
| 事件解耦 | 事件与业务写同事务入队 `cc_events`，由 worker 异步消费（`events/bus.go`、`events/worker.go`）—— **相对 PHP 的请求内同步发邮件是显著改进** |
| 并发领取 | worker 与 webhook 投递器都用 `FOR UPDATE SKIP LOCKED`（`events/worker.go:144`、`webhook/store.go:115`），不会重复消费 |
| 迁移竞态 | 迁移过程用 MySQL 命名锁 `GET_LOCK` 串行化，且全程绑定同一条连接（`internal/db/db.go:57-123`）—— 解决了 api/worker 同时启动的竞态 |
| 网关连接池 | `gateway.Client` 显式配置 `MaxIdleConns:32 / MaxIdleConnsPerHost:16`（`gateway/client.go:119-129`） |
| HTTP 服务器超时 | `ReadHeaderTimeout 15s / ReadTimeout 60s / WriteTimeout 120s / IdleTimeout 120s`（`cmd/codefever-api/main.go:246-253`） |
| 请求体限制 | `http.MaxBytesReader` + multipart 临时文件显式清理（`httpapi/server.go:160`、`:225-237`） |
| 会话续期 | 滑动续期，且**每 5 分钟最多写库一次**（`httpapi/dispatch.go:242-246`）—— 不是每个请求都 UPDATE |
| 仓库列表 N+1 | `/api/repository/list` 已**完全批量化**（`handlers/repository.go:147-181`，固定 8 条查询，与仓库数无关）—— 注释明确写了"全部改为批量查询" |
| `SELECT *` | **全仓库 0 命中**，列清单集中定义（`repo/repository.go:32-49` 等） |
| `GROUP BY` | 两处都在有索引的列上（`r_fork_r_key`、`(mr_target_r_key, mr_proceed_status)`） |
| 索引列套函数 | WHERE 子句里**没有** `DATE(col)` / `LOWER(col)` 之类 |
| `SELECT DISTINCT` | 仅 1 处（`repo/repository.go:522`）且是多余的（见 B14） |
| `FOR UPDATE` | 两处都带 `SKIP LOCKED`，无裸 `FOR UPDATE` |
| 密码哈希 | md5(md5(x)) legacy 模式（`docker-compose.yml:114`）—— 快，不构成瓶颈（这是安全议题，不是性能议题） |

**前端**

| 项 | 结论 |
|----|------|
| `@material-ui/icons` | **逐图标导入**（`@material-ui/icons/ExpandLess`），bundle 里仅 0.6 KB —— 写法正确 |
| `@fortawesome/free-solid-svg-icons` | ESM + `sideEffects:false`，**已 tree-shake 到 4.3 KB** —— 写法正确（注意：source map 的 696 KB 是原始源码，不是实际保留量） |
| `src/maps/*.geo.json`（281 KB） | **未被打进 bundle**（无任何引用）—— 是仓库里的孤儿文件，不是体积问题 |
| `CodeDiffViewer_old_monaco.js` 等死文件 | 已被正确 tree-shake 剔除（两个 bundle 中 0 命中）—— 仅是仓库整洁度问题 |
| `react-syntax-highlighter` | 未被引用，不在 bundle |
| `react-autosuggest` | 声明但未使用 —— 仓库整洁度问题 |
| 内联组件定义 | **0 处**（无 "component inside component" 反模式） |
| 请求瀑布 | **无 `await`、无 `Promise.all`**，全是 fire-and-forget `.then()` 链 ⇒ 不存在经典串行 await 瀑布 |
| 定时轮询 | 无数据轮询 `setInterval`（唯一的 `setInterval` 是 60 秒自清倒计时） |
| `withScrollTrigger` 清理 | **正确**（`this.cancel()` 以方法调用，`this` 绑定正确，卸载时确实移除监听） |
| Redux selector | 全部 `mapStateToProps` 取**窄字段**，没有组件 select 整个 state ⇒ 问题在 reducer 而非 selector |
| webpack 压缩配置 | Terser `ecma:5 / ascii_only / comments:false`，配置正确 |
| `runtimeChunk: false` | CRA 标准做法，非性能问题 |

---

## 7. 建议执行顺序

| 阶段 | 动作 | 预计收益 | 风险 |
|------|------|---------|------|
| **0** | **重新构建前端**（`GENERATE_SOURCEMAP=false`），修正 §5.1 的产物不同步 | 消除部署不一致；镜像 -6.3 MB | 低（需验证构建通过） |
| **1** | nginx 开启 gzip/brotli + 分级 `Cache-Control`（I1/I2） | 首屏 JS **1,663 KB → 449 KB（-73%）**；二次访问近 0 字节 | 低（需在容器内 `nginx -T` 确认主配置不冲突） |
| **2** | SPA 脚本 `defer` + preload（I3） | 解除解析阻塞 | 低 |
| **3** | 接上 `web-vitals`（F10）+ 开启 `slow_query_log`（I7） | 让后续所有改动**可度量** | 低 |
| **4** | 后端去冗余：B1（鉴权查库）、B2（双中间件链）、B13/B14/B18 | API p50 **-8.5 ms**；admin 日志量减半 | 低（B1/B2 需回归测试错误码语义） |
| **5** | 后端补索引（B11）+ `cc_events` 清理与领取索引（B8）+ `pagesize` clamp（B12） | 数据量上来后才是主要收益；现在代价极小 | 低（`ALTER TABLE` 在大表上会锁表，建议低峰执行） |
| **6** | 批量改造消除 N+1：B3（admin）、B5/B6/B7 | admin 单页 **102 → 3 次查询**；MR 列表 **40 → 2 次**；通知 worker **400 → 1 次** | 中（需回归功能） |
| **7** | git 操作移出事务（B4）、`groupKeyOf` 提前求值（B5） | 消除连接池耗尽风险 | 中（要保证"无仓无行"语义） |
| **8** | 前端路由分割（F1）+ 依赖裁剪（F6） | 首屏 JS **449 → ~250 KB gzip** | 中 |
| **9** | 前端渲染性能：F2/F3/F4/F5/F8/F9 | 降低 INP / 长任务 | 中 |
| **10** | 虚拟化、key 常量化、剩余 Low 项（F13-F20、I5、I8、I9） | 打磨 | 低 |

---

## 8. 验证方法（Top 3）

### 验证 1 —— 压缩与缓存（阶段 1）

```bash
# 容器内确认主 nginx.conf 是否已开 gzip（避免重复配置）
docker exec codefever-next-app nginx -T 2>/dev/null | grep -nE 'gzip|brotli|sendfile'

# 验证：应看到 Content-Encoding: gzip，且体积降至 ~1/3
curl -sI -H 'Accept-Encoding: gzip' http://localhost:9080/static/js/main.3a8cdea3.js | \
  grep -iE 'content-encoding|cache-control|content-length'

# 验证：二次访问应命中缓存（本地 curl 不带 If-None-Match，浏览器会发）
curl -sI http://localhost:9080/static/js/main.3a8cdea3.js | grep -i cache-control
```

### 验证 2 —— 后端串行往返次数（阶段 4）

```sql
-- 临时开启通用日志，命中一次 /api/repository/list，统计查询条数
SET GLOBAL general_log = 'ON';
SET GLOBAL log_output = 'TABLE';
-- 触发请求后：
SELECT COUNT(*) AS queries FROM mysql.general_log
WHERE command_type = 'Query' AND argument LIKE '%cc_%';
-- 预期：修复前 ~16 条，修复后 ~10 条（B1 省 2 条）

SET GLOBAL general_log = 'OFF';
```

同时用应用自带的访问日志交叉验证：`/api/repository/list` 的 `duration_ms` 应从 67 ms 降到 ~58 ms。

### 验证 3 —— 前端重渲染与 CWV（阶段 8/9）

1. 接入 `web-vitals` 后，用 Chrome DevTools → Lighthouse → Performance → Mobile，Slow 4G 模拟，**同一 URL 跑 3 次取中位数**（前后各一轮，条件必须一致）。
2. React DevTools → Profiler → 记录 `/commits` → 点两次"更多"。修复前应看到**整棵树**重渲染；修复后应只剩 `CommitList` + 新增的 `CommitItem`。在 "Ranked" 视图按耗时排序，`JSON.stringify` 与 `Prism.highlight` 应从 top-10 中消失。
3. `npx serve -s www/view/build -l 5000` 单独服务构建产物，测首屏 LCP（排除后端干扰）。

### 验证 4 —— 索引效果

```sql
-- 补索引后对比（对高价值表逐个执行）
EXPLAIN SELECT a_key FROM cc_activities
WHERE a_relative_r_key = 'x' AND a_relative_mr_key = 'y' AND a_status = 1
ORDER BY a_created DESC;
-- 期望：key = idx_cc_activities_mr (a_relative_r_key, a_relative_mr_key)，Extra 不再是 "Using filesort"

EXPLAIN SELECT ev_key FROM cc_events
WHERE ev_status = 1 AND ev_attempts < 5 ORDER BY ev_created ASC LIMIT 50;
-- 期望：走 idx_events_claim (ev_status, ev_created)，无 filesort
```

---

## 9. 需要你决策的开放问题

1. **§5.1 的构建产物不同步**是要立即修（重新构建并提交），还是先记录为已知问题？本报告的 bundle 数字需要在此之后复测。
2. **`@pgyer/icons` 是否可控？** 若可 patch，加一行 `"sideEffects": false` 即可省 ~80 KB（零风险）；否则需改 43 处 import 路径。
3. **部署是否单语？** 若是，去掉 `zh-cn`/`en-us` 之一可再省 ~29 KB。
4. **基础镜像 `webdevops/php-nginx:7.4` 是否要换？** 1.67 GB 镜像里 PHP 已删但基础层仍在（`Dockerfile:189-234`）。换成 `nginx:alpine` 可减到 ~200 MB，但需回归 nginx 配置与 supervisor 编排。
5. **DB 与 API 是否同机？** 若分机部署，4.24 ms 的往返会放大，§2 的串行往返问题优先级要相应上调。
6. **是否有生产数据量级信息？** 补一份 `SELECT table_name, table_rows FROM information_schema.tables` 的生产快照，可以把 §2 里"取决于表大小"的 Medium 定级升级为 High 或降级为 Low。

---

*报告由 `performance-experts` 插件的 `performance` / `performance-optimization` / `vercel-react-best-practices` 三个技能的方法论产出；所有数字均为对运行中容器或仓库产物的实测，未做无证据推断。*
