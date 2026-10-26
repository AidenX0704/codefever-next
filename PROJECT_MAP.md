# PROJECT_MAP

本文件是项目的**结构与调用链地图**，用来回答"某个能力在哪、请求怎么走、
改一处会牵动谁"。

维护约定见 `AGENTS.md` §3：

- 模块 / 职责 / 接口签名 / 调用链变化时，**在同一逻辑单元内**更新本文件；
- 废弃内容**只标注不删除**（`（已废弃 YYYY-MM-DD：原因）` / `（已迁移至 X）`）；
- 末尾的「人工备注」由人维护，代理只可追加，不得重写既有内容。

> 最后核对：2026-10-25（公开 API `/api/v1` 与访问令牌体系落地后）

---

## 1. 顶层结构

| 路径 | 作用 |
|---|---|
| `backend/` | Go 后端（单一 module `codefever-next/backend`），全部服务与工具 |
| `www/` | （**已删除 2026-10-24**：旧前端 `www/view`、`www/static` 与 `www/app` 软链全部移除；镜像内 `www/` 由 Dockerfile 用 `web/public` + `web/build` 生成） |
| `web/` | **唯一前端**（React 19 + TypeScript + Vite，自研设计令牌 + 组件库，零 UI 框架依赖）；产物 `web/build` 入库，Dockerfile 只 COPY；方案与进度见 `docs/FRONTEND_REWRITE_PLAN.md` |
| `backend/deploy/` | nginx vhost 模板、supervisor 配置、SQL 初始化脚本 |
| `codefever-data/` | 运行时数据（MySQL、裸仓库、附件、配置软链），可用 `CODEFEVER_DATA_DIR` 覆盖 |
| `docs/` | 架构分析、迁移计划与实施状态、性能审查与整改方案 |
| `.github/workflows/ci.yml` | CI：gofmt / build / 交叉编译 / vet / test + compose 校验 |

---

## 2. 后端进程（5 个二进制 + 4 个工具）

生产进程：

| 二进制 | 职责 | 入口 |
|---|---|---|
| `codefever-api` | HTTP API、服务端渲染页面（登录/注册/文档等）、迁移、清理任务 | `cmd/codefever-api/main.go` |
| `codefever-worker` | 消费 `cc_events`，驱动通知 / 活动流 / webhook 投递 | `cmd/codefever-worker/` |
| `codefever-hook` | git hooks 回调（post-receive 等） | `cmd/codefever-hook/` |
| `codefever-http-gateway` | Git over HTTP（`net/http/cgi` 包装 git-http-backend） | `cmd/codefever-http-gateway/` |
| `codefever-ssh-gateway` | Git over SSH | `cmd/codefever-ssh-gateway/` |

开发/验证工具（不进生产镜像）：

| 工具 | 用途 |
|---|---|
| `contract-collect` / `contract-compare` | HTTP 契约基线采集与比对（保证 PHP→Go 语义一致） |
| ~~`page-compare` / `pagediff`~~ | ~~服务端渲染页面的 PHP 产物比对~~（**已删除 2026-10-24**：SSR 页面被 SPA 取代，比较对象不复存在） |
| `smoke-write` | 以 multipart 驱动**写端点成功路径**并断言，需隔离环境 |
| `stress` | 稳定性/性能门禁：QPS、p50/p95/p99、按端点分布、pprof 采样 |

---

## 3. 后端包职责（`backend/internal/`）

| 包 | 职责 |
|---|---|
| `httpapi` | HTTP 服务器装配、路由、中间件链（recover → log → limit → normalize）、API 分发、**SPA 壳下发**（`spa.go`）。页面模板层 `pages.go` / `templates/` / `pages_test.go` **已删除 2026-10-24**（前端重写） |
| `httpapi/publicapi` | **公开 REST API `/api/v1`**（2026-10-25 新增）：面向外部 CI / 脚本，与内部 `/api/*` **完全隔离** —— 自建 mux + `Wrap()` 前置分流、裸 JSON 响应、错误体 `{"message","url"}`、令牌鉴权（令牌 ≡ 用户）、私有资源越权返回 **404**（不泄露存在性）。**不复用 `contract.Envelope`**（PHP 兼容契约，123 条基线锁定）。 |
| `contract` | 请求解析（JSON / form / multipart）、PHP 兼容的错误码与响应信封 |
| `handlers` | 各域 HTTP handler（user / admin / group / repository / internal / login / webvitals / doc） |
| `repo` | 数据访问层；事务封装（`WithTx`）、批量查询、保留期清理 |
| `domain` | 领域类型与纯逻辑（状态常量、取值映射、分页口径 `ClampPaging`、事件类型） |
| `db` | 连接池（`MaxOpenConns=25`）、`//go:embed` 版本化迁移执行器（`GET_LOCK` 串行化） |
| `events` | 事件总线（与业务写同事务入队）、worker（`FOR UPDATE SKIP LOCKED` 领取）、各类事件处理器 |
| `webhook` | webhook 投递器（独立队列与重试） |
| `gitsvc` | git 命令封装：argv 直传（不经 shell）、路径穿越校验、超时、hooks 安装 |
| `githttp` / `gitssh` | Git over HTTP / SSH 协议接入 |
| `gateway` | 内部网关 HTTP 客户端（显式连接池） |
| `session` | 服务端会话存储（滑动续期，最多 5 分钟写库一次） |
| `auth` / `authkeys` | 口令校验、Token 比较、密钥处理 |
| `storage` | 磁盘存储抽象（上传文件、附件） |
| `mailer` | 邮件发送 |
| `config` / `appdir` | 配置装载（`config.yaml` / `env.yaml`）、路径解析 |

---

## 4. HTTP 请求链路

### 4.1 API 请求

```
nginx (vhost-go.conf-template)
  └─ location ~* ^/(user|file|api…)?/…   → proxy_pass 127.0.0.1:9001
       └─ httpapi.Server.Handler()
            ├─ recoverMiddleware
            ├─ logMiddleware            （每个 request_id 只应出现一条访问日志）
            ├─ limitRequestBody         （http.MaxBytesReader）
            ├─ normalizeAPIPath
            └─ mux
                 ├─ 字面量 page route（/user/login、/user/logout、/api/web-vitals）
                 └─ /api/{ctrl}/{action} → handleAPI
                      ├─ contract.Parse            （读 body 并**复位请求流**，multipart 才能解析）
                      ├─ authenticateWithUser      （一次解析：cc_sessions + cc_users 各 1 次）
                      ├─ 动作查找（未移植动作 → JSON 0x0105）
                      └─ handler → repo → MySQL
```

### 4.1.1 公开 API `/api/v1`（2026-10-25 新增，与 4.1 并行且**互不影响**）

```
请求 /api/v1/**
  └─ httpapi.Server.Handler()
       └─ publicapi.Router.Wrap(handler)      ← **在主 mux 之前**分流
            ├─ 路径不是 /api/v1/**  → 透传给内部主 mux（4.1 的链路，逐字不变）
            └─ 路径是 /api/v1/**   → 公开 API 自建 mux
                 ├─ authenticate / optionalUser   （Authorization: token <明文> | Bearer | ?access_token=）
                 │    └─ repo.AccessTokenRepo.BySHA256（**只存 SHA-256**）→ 令牌 ≡ 用户
                 ├─ canReadRepo / resolveRepo    （私有资源越权 → **404**，不是 403）
                 └─ 裸 JSON 响应 / 错误体 {"message","url"}   （**不复用 contract.Envelope**）
```

> **为什么分流在主 mux 之前**：主 mux 已有 `/api/{ctrl}/{action}` 通配路由，
> 再注册 `/api/v1/` 会构成 Go `ServeMux` 的**歧义模式**（两者都能匹配
> `/api/v1/action` 且谁都不比谁更具体）→ **进程启动即 panic**。
> 内部 `/api/*` 是 PHP 兼容契约载体、不可移除，故公开 API 只能自建 mux。
>
> `normalizeAPIPath` 另有 `publicapi.IsPublicAPIPath(path)` 豁免，
> 属**纵深防御**（分流在截断之外，公开路径本就不会被截断）。

### 4.2 Git over HTTP

```
nginx  location ~* ^/[0-9a-z_.-]+/[0-9a-z_.-]+\.git → 127.0.0.1:27555
  └─ codefever-http-gateway → githttp → gitsvc（exec git-http-backend）
```

> 注意：不要开 `proxy_request_buffering off`。Go 的 `net/http/cgi` 拒绝
> `Transfer-Encoding: chunked`，关闭缓冲后大 pack 会被 chunked 转发而直接 400。

### 4.3 事件链路

```
handler（业务写）
  └─ events.Bus.Emit(ctx, raw *sql.Tx, …)      ← 与业务写**同一事务**入队 cc_events
       └─ codefever-worker（每 2 秒轮询，FOR UPDATE SKIP LOCKED 领取）
            ├─ notification  → cc_notification_content/users（每批 100 人多行 INSERT）
            ├─ activity      → cc_activities
            └─ webhook       → cc_repository_webhook_events → webhook 投递器
```

---

## 5. 前端结构

> **`www/view/src`（旧 React 17 + Redux + MUI v4 + webpack 4）已于 2026-10-24 随
> 前端重写整体删除**，其结构说明保留在 `docs/FRONTEND_REWRITE_PLAN.md` 与历史提交里。
> 当前前端见 §5.1。

### 5.1 新前端 `web/`（重写进行中，2026-10-24 起）

替代关系见 `docs/FRONTEND_REWRITE_PLAN.md`（删除清单、令牌口径、分批顺序的唯一事实源）。

| 路径 | 职责 |
|---|---|
| `vite.config.ts` | 构建配置：产物 `web/build`、`assetsDir=static/js`（对齐 nginx 一年 immutable）、现代目标 es2022、`/api` 等路径 dev 代理到 Go 9001 |
| `src/theme/theme.css` | **全仓库唯一的视觉字面值来源**（颜色/间距/圆角/阴影/字号），逐项抄自旧 `@pgyer/mui-theme-preset` 与 `theme1.js`；另有 `--cf-field-surface`（**输入区表面色的唯一来源**，2026-10-25 新增，用于中和 Chrome 自动填充的 UA 蓝底并按变体给出正确盖色） |
| `src/theme/tokens.ts` | 令牌的类型化引用 + `space(n)`（等价旧 `theme.spacing`） |
| `src/theme/global.css` | 全局基础样式与工具类（对应旧 `App.styl`） |
| `src/components/unit/` | 自研组件库（磁盘上 **27** 个目录）：Button / IconButton / SquareIconButton / Icon / TextField / SelectField / Choices / Form / Table(+DataTable/Pagination) / Dialog / Confirm / Dropdown / Tabs / Tooltip / Toast / Avatar / Tag / Marker / Card / Loading / EmptyState / CopyButton / AppLink / FormattedTime / CodeViewer / DiffViewer（文件级 diff：汇总条 + inline·side-by-side 布局切换 + 逐文件折叠 + 复制路径 / 查看 @hash / 展开完整文件）/ FilterBar（页签+搜索+排序的工具条，分支·标签·活动三处共用）。⚠️ `TextareaField` / `Checkbox` / `Switch` **尚未实现**（此前清单误列，2026-10-25 校正），Release / Issue 表单落地前须先补齐 |
| `src/components/layout/` | 外壳布局：NotificationDrawer（通知抽屉，四个分类 + 已读/清空）、AppShell（固定页头 + 白底内容区）、Header（仓库/组/新建/语言/帮助/管理后台/用户菜单）、RepositoryLayout（仓库上下文外壳）、SettingsLayout（个人设置外壳）、AdminLayout（管理后台外壳）、Sidebar（左侧上下文导航，可折叠） |
| `src/components/view/` | 页面级组件（按业务域，逐批落地）：`auth/` = AuthLayout + 认证页样式（批 1）；`unit/` = RepositoryRow / GroupCard / list.css（批 2） |
| `src/components/view/unit/` | 业务域共用件：`RepositoryRow`、`GroupCard`、`CommitRow`、`MembersManager`（仓库成员与组成员共用）、`AdvancedSettings`（仓库/组高级页共用）、`ActivityFeed`（41 个活动码的表驱动渲染）、`MergeRequestRow`（MR 行：标题/状态徽标/目标仓库名/分支链接/最后提交人/编辑于/评审人组/更新时间）、`MergeRequestToolbar`（MR 页签+搜索+排序+主操作）、`MergeRequestListBody`（MR 行列表+空态+查看更多）、`MergeRequestReviewers`（评审人头像组+已/未评审角标）、`AdminMembersDialog`（后台组/仓库成员弹窗）、`SettingsNotificationMatrix`（通知矩阵：组+仓库+角色徽标）、`RepositoryHeader`（仓库头：Fork/克隆地址/6 项计数）、`RefListView`（分支·标签列表共用）、`ObjectViewer`（文件内容富视图）、`MarkdownView`、`RepositoryEmpty`（空仓库引导）、`CommitRefs`（父提交+包含该提交的分支/标签）；样式集中在 `list.css` / `commit-row.css` / `merge-request.css` / `repository.css` |
| `src/api/client.ts` | 统一请求层：multipart 编码（`key[sub]` / `key[]`、剥 `_SAFE`/`_BINARY`）、`requestTime`、信封与错误码表、`0x0101` 跳登录；另有 `postForm()`（表单页的 urlencoded 信封 `{code,message,extra}`） |
| `src/api/types.ts` | 后端归一化结构的类型（字段名与 Go 侧逐字一致，勿改名） |
| `src/api/{user,group,repository,admin}.ts` | 端点封装：`user`（info / updateBasicInfo / updatePassword / uploadAvatar / MFA / SSH / 邮箱 / 通知）、`group`（list / config / create / update / updateName / 成员 / deleteGroup / activities / mergeRequests）、`repository`（list / config / object / commitList / commitDetail / fileChanges / branch·tag 增删 / create·fork / mergeRequest 全套 / activities / blameInfo / refListContainSHA / webhook 全套 / protectedBranch）、`admin`（users 分类与排序 / groups / repositories / 成员增删 / addUser / checkPassword / config） |
| `src/api/doc.ts` | `/api/doc/tree`、`/api/doc/content`（Go 侧新增的文档 JSON 端点） |
| `src/api/auth.ts` | 认证端点：`/user/login`、`/user/register`、`/user/verify`、`/user/resetPassword`、`/user/getResetPasswordCode` |
| `src/features/app-data/` | `AppDataProvider`：页头与列表共享的用户/仓库/组三份数据（对应旧 Redux `DataStore`） |
| `src/features/repository/` | `RepositoryProvider`：按「组名 + 仓库名」解析仓库主键 → 加载 `repository/config`，找不到即 404 |
| `src/features/group/` | `GroupProvider`：按组名解析组主键 → 加载 `group/config`，找不到即 404 |
| `src/config/` | `uac.ts`（角色与权限矩阵）、`icons.ts`（按需图标出口）、`constants.ts`（头像 URL 前缀）、`languages.ts`（扩展名→Prism 语言）、`drawer.ts`（仓库上下文 / 个人设置导航项与 active 判定）、`webhookEvents.ts`（25 个 Webhook 事件与文案键） |
| `src/helpers/prism.ts` | Prism 语言注册（顺序敏感）+ 按「语言+全文」的 8 条 LRU 高亮缓存 |
| `src/helpers/members.ts` | 提交邮箱 → 仓库成员解析（未命中标记临时用户，对应旧 `getUserInfo`） |
| `src/helpers/slug.ts` | 标识名规范化 `makeSlug`（逐字对齐旧 `NetworkHelper.makeSlug`：非 `\w` 一律换下划线） |
| `src/helpers/useDebouncedValue.ts` | 通用防抖值（管理后台三个列表页的 200ms 搜索防抖共用） |
| `src/helpers/format.ts` | 数值/体积/日期分组格式化（`formatNumber` / `formatBytes` / `dayKey`） |
| `src/pages/repository/` | 仓库页：`FileTree`（分支/标签下拉、面包屑、目录行、blob 分流、README 渲染、submodule、文件名过滤、blame 模式）、`CommitList`（服务端搜索 + 每页 20 条 + 日期分组 + 子目录路径）、`CommitDetail`（详情 + diff + 父提交展开 + 浏览文件）、`BranchList`/`TagList`（默认/已合并/保护标记、更新者、增删、筛选排序分页）、`CreateBranch`/`CreateTag`、`SettingsGeneral`（显示名/描述/头像 + 30·200 字校验）、`SettingsMembers`（仓库成员/组成员分表 + 增删改与所有权转移）、`SettingsBranch`（默认分支 + 保护分支规则，角色集合语义）、`SettingsWebhook`（列表/新建/编辑/删除/投递日志，含 secret）、`SettingsAdvanced`（改 slug/改创建者/删除仓库，两级确认）、`MergeRequestList`（全部/待合并/已合并/已关闭页签+角标、搜索、排序、每页 10 条+查看更多）、`MergeRequestCreate`（源/目标仓库与分支+评审人，全局态用 `global`）、`MergeRequestDetail`（详情+三 Tab+diff+评审/合并/关闭+评审人管理+右侧活动栏）、`ActivityList`（仓库动态：分类页签/搜索/翻页） |
| `src/pages/MergeRequestBoard.tsx` | 全局 `/mergerequests` 与组内 `/groups/{名}/mergerequests` 看板：页签+角标（全局按 `relatedRepository` 命中仓库累加、组内按同组仓库累加）、搜索、排序、每页 10 条+查看更多、创建合并请求入口 |
| `src/pages/admin/` | 管理后台：`Dashboard`（用量卡可点击/资源阈值配色/服务状态）、`Users`（四个分类页签+排序+添加用户与初始口令回显+设管理员口令校验+重置口令）、`Groups`（图标列/创建时间/成员管理）、`Repositories`（同构+组继承成员标记）、`Settings`（config 结构化表单 + 7 条校验） |
| `src/pages/Doc.tsx` | 文档中心（`/doc/{lang}/{path}`）：菜单树 + Markdown 渲染（react-markdown + remark-gfm） |
| `src/pages/RepositoryList.tsx` | 仓库列表（`/repositories`、`/groups/{组}/repositories`、`/repositories/forklist/{仓库主键}`）：页签（全部/我参与的/我创建的）、fork 上下文（首 tab 变「fork 自 X」） |
| `src/pages/GroupList.tsx` | 仓库组列表（`/groups`） |
| `src/pages/NewRepository.tsx` | 新建仓库（`/repositories/new`、`/groups/{组}/repositories/new`） |
| `src/pages/NewRepositoryFork.tsx` | Fork 仓库（`/repositories/fork/{源仓库主键}`） |
| `src/pages/NewGroup.tsx` | 新建仓库组（`/groups/new`） |
| `src/pages/NewMergeRequest.tsx` | 全局新建 MR（`/mergerequests/new`、`/groups/{组}/mergerequests/new`）：复用 `MergeRequestCreate` 的 `global` 态 |
| `src/pages/group/` | 仓库组：`ActivityList`（组动态）、`SettingsGeneral`（组名 5–30 字校验/描述 200 字/头像）、`SettingsMembers`（成员管理）、`SettingsAdvanced`（改 slug/删除） |
| `src/pages/settings/` | 个人设置：`Profile`（基本信息含可编辑 role/头像/口令/MFA 二维码+更换设备）、`Email`（主邮箱更换 + 提交邮箱管理 + 120 秒重发倒计时）、`SSHKey`（列表/新增/删除 + 指纹列 + 整行公钥容忍）、`Notification`（位掩码开关 + 组/仓库级通知矩阵 + 拒收列表恢复） |
| `src/i18n/supplement.ts` | 旧词表缺失的补充键（运行时覆盖自动生成的表，避免手改生成文件） |
| `src/assets/icons/` | 154 个图标数据（由 `scripts/extract-icons.mjs` 从 `@pgyer/icons@0.0.16` 本地化） |
| `src/i18n/` | `zh-cn.ts` / `en-us.ts` 由 `scripts/extract-lang.mjs` 自动生成（961 键 × 2：Go SSR 词表 + 旧 SPA 词表）；`I18nProvider` + `useT()`（支持 `{s}`/`{n}`） |
| `src/pages/auth/` | 认证四页：Login / Register / Verify / ResetPassword（独立全屏，不套 AppShell） |
| `src/pages/` | 路由页面：`DesignSystem`（`/__design` 预览页）、`NotFound` |
| `src/router.tsx` | 路由表；全部页面 `lazy`；react-router 8 不支持路径正则，业务校验下沉到页面 |
| `src/assets/images/` | 复用素材（16 张，来自 `www/static/images`，图名保持原样） |
| `scripts/extract-lang.mjs` | 从 Go 语言表 + 旧 SPA 词表抽取 i18n（批 7 随旧前端退役） |
| `scripts/extract-icons.mjs` | 把 `@pgyer/icons` 的图标数据本地化到 `src/assets/icons/` |
| `scripts/serve-preview.mjs` | 视觉验证用静态服务器 + 最小 mock API（`vite preview` 会把 `/api`、`/user` 代理到后端导致 502） |

前端门禁（AGENTS §8.3 待批 7 同步）：`npm run typecheck && npm run lint && npm run build`，产物 `web/build` 入库。

---

## 6. 数据层

- **MySQL 8.0**；连接池 `MaxOpenConns=25`。
- **两套建表来源**（重要）：
  - 17 张遗留表 + 初始数据：`backend/deploy/sql/db_structure.sql`、
    `db_data.sql`，由 `create_db.sh` 在**全新安装**时导入；
  - 版本化迁移：`backend/internal/db/migrations/*.sql`，由 `internal/db` 用
    `//go:embed` 加载并在 `GET_LOCK` 下串行执行，记录于 `cc_schema_migrations`。
  - ⇒ **对已部署实例生效的变更必须写成 migrations**，写进 `db_structure.sql` 无效。
- 当前迁移：`001_sessions`、`002_events`、`003_events_lease`、
  `004_perf_index_users`、`005_perf_index_events`、`006_perf_index_members`、
  `007_perf_index_repositories`、`008_perf_index_notification`、
  `009_perf_index_activity_rule`、`010_access_tokens`。
- 关键表：`cc_users`、`cc_sessions`、`cc_repositories`、`cc_groups`、
  `cc_group_members`、`cc_repository_members`、`cc_merge_requests`、
  `cc_activities`、`cc_events`、`cc_notification_content`、
  `cc_notification_users`、`cc_notification_refuse`、
  `cc_repository_webhook_{s,events,logs}`、`cc_proteced_branch_rules`
  （表名拼写为历史遗留，勿改）、`cc_access_tokens`
  （**公开 API 访问令牌**，2026-10-25 新增；**只存 SHA-256 摘要，不存明文**，
  `at_scope` 列预留但第一版不实现 —— 令牌 ≡ 用户）。
- 保留期：`internal/repo/retention.go` + `codefever-api` 的每日任务；
  事件（已处理 30 天 / 死信 60 天，**待处理与处理中永不删**）、已读站内信
  90 天 + 孤儿内容行、webhook 日志 90 天；`cc_activities` 默认不清理。

---

## 7. 部署

```
Dockerfile（两阶段）
  ├─ gobuilder: golang:* → 编译 5 个二进制（-trimpath -ldflags="-s -w"）
  └─ runtime:  webdevops/php-nginx:7.4（PHP 已移除，仅复用 nginx + supervisor）
        ├─ COPY www/（含预构建的 view/build）
        ├─ ln -s view/build www/app        ← SPA 回退目录
        └─ supervisor 拉起 5 个进程
```

- 站点配置：`backend/deploy/docker/vhost-go.conf-template`
  （gzip + 分级 `Cache-Control`；`/static/js|css` 一年 immutable，
  其余 `/static/` 7 天 + swr，SPA 壳 `no-cache`）。
- 编排：`docker-compose.yml`（`db` + `codefever`），数据落在 `codefever-data/`。
- MySQL 参数：`slow_query_log=1`、`long_query_time=0.2`、`max_connections=3000`
  （真实连接瓶颈在 Go 侧的 25，不要靠调大此项"解决"连接问题）。

---

## 8. 人工备注

> 本节由人维护。代理只可追加，不得重写或删改既有内容。

- 2026-10-24：本文件首次创建，内容基于性能整改期间的实地核对。
  若发现与实际不符，请在下方追加更正条目，不要直接改写上文。

- 2026-10-24：前端重写收口 —— 旧前端（`www/view` 195 文件、`www/static` 52 文件）与
  Go 的 SSR 页面层（`pages.go`、`templates/`、`pages_test.go`、`testdata/pages`、
  `cmd/page-compare`、`cmd/pagediff`、`web_captcha.go`）已删除；`/user/*`、`/doc/*` 的 GET
  改为下发 SPA 壳（`internal/httpapi/spa.go`）。API 契约基线（`internal/contract` +
  `testdata/contract*`）未受影响，仍是当前的功能回归网。

- 2026-10-25：校正 `src/components/unit/` 清单 —— `TextareaField` / `Checkbox` / `Switch`
  此前被误列为已存在，磁盘上**并不存在**；实际存在的 `Choices` 此前漏列。
  按本节约定以追加方式更正，未删改原始描述之外的内容。

- 2026-10-25：输入框表面色统一 —— 新增设计令牌 `--cf-field-surface`
  （`src/theme/theme.css`），并在组件层（`src/styles/components.css`）全局中和
  Chrome / Edge 自动填充的 UA 强制蓝底（此前多邮箱页邮件+密码框会出现
  「文字底部一条蓝带、输入区被割裂成三段」）。影响**全站所有输入框**，
  修复落在组件层而非单个页面。

- 2026-10-25：新增**公开 REST API `/api/v1`** 与**访问令牌体系**（见 §3 `httpapi/publicapi`
  行、§4.1.1 链路图、§6 `cc_access_tokens`）。内部 `/api/*`（PHP 兼容契约，123 条基线）
  **一行未改**。公开 API 与内部 API **不共用响应包络**：前者裸 JSON + `{"message","url"}`，
  后者仍是 `{request-id,code,message,data}`。令牌**只存 SHA-256**，明文仅创建时返回一次。
  令牌管理 UI 与内部端点、Release 功能**尚未实现**。
