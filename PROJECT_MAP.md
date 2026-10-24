# PROJECT_MAP

本文件是项目的**结构与调用链地图**，用来回答"某个能力在哪、请求怎么走、
改一处会牵动谁"。

维护约定见 `AGENTS.md` §3：

- 模块 / 职责 / 接口签名 / 调用链变化时，**在同一逻辑单元内**更新本文件；
- 废弃内容**只标注不删除**（`（已废弃 YYYY-MM-DD：原因）` / `（已迁移至 X）`）；
- 末尾的「人工备注」由人维护，代理只可追加，不得重写既有内容。

> 最后核对：2026-10-24（性能整改落地后）

---

## 1. 顶层结构

| 路径 | 作用 |
|---|---|
| `backend/` | Go 后端（单一 module `codefever-next/backend`），全部服务与工具 |
| `www/view/` | React SPA（React 17 + Redux + MUI v4 + webpack 4） |
| `www/view/build/` | **入库的前端构建产物**（Dockerfile 只 COPY，不在镜像内构建） |
| `www/static/` | 服务端渲染页面共用的静态资源（vendor、图片、按发布号分版本的自有静态） |
| `backend/deploy/` | nginx vhost 模板、supervisor 配置、SQL 初始化脚本 |
| `codefever-data/` | 运行时数据（MySQL、裸仓库、附件、配置软链），可用 `CODEFEVER_DATA_DIR` 覆盖 |
| `docs/` | 架构分析、迁移计划与实施状态、性能审查与整改方案 |
| `.github/workflows/ci.yml` | CI：gofmt / build / 交叉编译 / vet / test + compose 校验 |

---

## 2. 后端进程（5 个二进制 + 6 个工具）

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
| `page-compare` / `pagediff` | 服务端渲染页面的 PHP 产物比对 |
| `smoke-write` | 以 multipart 驱动**写端点成功路径**并断言，需隔离环境 |
| `stress` | 稳定性/性能门禁：QPS、p50/p95/p99、按端点分布、pprof 采样 |

---

## 3. 后端包职责（`backend/internal/`）

| 包 | 职责 |
|---|---|
| `httpapi` | HTTP 服务器装配、路由、中间件链（recover → log → limit → normalize）、API 分发、页面渲染入口 |
| `contract` | 请求解析（JSON / form / multipart）、PHP 兼容的错误码与响应信封 |
| `handlers` | 各域 HTTP handler（user / admin / group / repository / internal / login / webvitals） |
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

## 5. 前端结构（`www/view/src`）

| 路径 | 职责 |
|---|---|
| `index.js` | 入口：样式、polyfill、Redux store、Router、web-vitals 上报（动态 import） |
| `routes/MainRoutes.js` | 路由表；34 个页面全部 `React.lazy`，按业务域分 7 个 chunk |
| `components/App.js` | 应用外壳（主题、i18n、Picker provider） |
| `components/Main.js` | 主内容外壳；路由参数变化时拉取组/仓库配置（副作用在 `componentDidUpdate`） |
| `components/unit/` | 通用 UI 原子件（CodeViewer、ObjectViewer、RefList、空态、复制按钮…） |
| `components/view/` | 页面级组件（按业务域分文件） |
| `components/view/unit/` | 业务域内共用件（仓库卡片、提交行、成员项、MR 行…） |
| `components/view/admin/` | 管理端页面（独立 chunk） |
| `reducers/` | `DataStore` / `DrawerStates` / `NotificationStates`，全部不可变更新 |
| `data_providers/` | 请求封装（`main.js` + User/Group/Repository/Admin） |
| `helpers/` | 纯逻辑（格式化、过滤、校验、网络、滚动触发、Markdown 渲染器） |
| `config/` | 常量与映射（`Constants.js` 的扩展名→高亮语言、枚举、状态码） |
| `lang/` | 双语消息（zh-cn / en-us） |
| `assets/styles/` | 全局样式（`App.styl`）、`markdown-body.css`（本地内置的 github-markdown-css） |

前端 chunk 划分（`webpack.config.js` 的 `webpackChunkName`）：

`admin` / `settings` / `group` / `repository` / `repository-cfg` /
`merge-request` / `misc`，其余为 Markdown 渲染器与 prism 主题等按需块。

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
  `009_perf_index_activity_rule`。
- 关键表：`cc_users`、`cc_sessions`、`cc_repositories`、`cc_groups`、
  `cc_group_members`、`cc_repository_members`、`cc_merge_requests`、
  `cc_activities`、`cc_events`、`cc_notification_content`、
  `cc_notification_users`、`cc_notification_refuse`、
  `cc_repository_webhook_{s,events,logs}`、`cc_proteced_branch_rules`
  （表名拼写为历史遗留，勿改）。
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
