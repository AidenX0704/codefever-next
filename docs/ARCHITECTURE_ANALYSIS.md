# CodeFever Community 架构分析与可优化点清单

> **注意（2026-10-24）**：本文档中出现的 `www/view/`、`www/static/` 与 MUI v4 相关描述
> 属于**历史记录**——旧前端已在 2026-10 前端重写中删除。当前前端为 `web/`
> （React 19 + TypeScript + Vite + 自研令牌与组件库），见 `docs/FRONTEND_REWRITE_PLAN.md`。


> 采集方式：仓库全量静态阅读 + 运行态实测（容器 `codefever-app` / `codefever-db`）+ 内置浏览器 UI 取证 + MySQL 8.0.46 兼容性实测。
> 证据均带 `文件:行号`，结论可逐条复核。

> **文档状态（2026-10-23 晚）**：本文分析的是 **PHP 时代的旧架构**，用于解释“为什么要
> 重写”。当前仓库已把 `http-gateway/`、`ssh-gateway/` 并入
> `backend/cmd/`，把 `misc/` 迁移到 `backend/deploy/`，并移除了
> `docker-compose.local.yml` 与 `CODEFEVER_BACKEND`。文中这些路径是**历史快照**；
> 当前部署/目录以仓库根 `README.md` 与 `LOCAL_DEPLOY.md` 为准。

---

## 0. 结论摘要

| 维度 | 现状 | 判断 |
|---|---|---|
| 整体架构 | CodeIgniter **2.1.1**（不是 3.x）+ 自研 service 层 + 两个 Go 网关 + React SPA | 分层存在但被"框架胶水 + 隐式依赖"严重侵蚀，**换语言重写后端是正确决策** |
| 前端 | React 17 + MUI v4 + `@pgyer/mui-theme-preset`，与后端仅通过 REST + session cookie 耦合 | **完全可原样保留**。UI 风格一致性 = 前端零改动 + API 契约零变更，不是重绘问题 |
| API 面 | 后端 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调；前端 provider 定义 109 个 URL | 契约面明确且可枚举，**可作为重写的验收基线** |
| 数据 | MySQL 5.7.31，17 张表（14 + 3 webhook） | schema 干净、utf8mb4，**可直接演进到 MySQL 8**（已实测修复路径） |
| 部署 | nginx + php-fpm + sshd + cron + 2 个 supervisor 进程 + Go 网关 | 路径耦合强但**可保留**，重写只需替换 PHP 那一个进程 |
| 安全 | md5(md5()) 密码、shell 字符串拼接、11 处越权读取缺口 | **不能带病迁移**，重写是修掉这些的最佳窗口 |
| 最大单点风险 | `application/models/repository_model.php`（1977 行）承担业务 + git 进程执行 | 重写的首要收益点 |

**核心判断**：这套系统的"业务资产"（权限矩阵、事件类型、通知模板、git 语义）集中在少数几个文件里、且质量不错；"框架胶水"（CI2 ActiveRecord、静态单例、shell 拼接）占了大头且是缺陷来源。**重写 = 保留业务资产、丢弃框架胶水**，这是一次高性价比的重构。

---

## 1. 系统拓扑

```
                          ┌──────────────────────────────────────────────┐
   浏览器 ──── HTTP ─────▶ │ nginx :80                                    │
                          │  ├─ /api/*  /apiv1/*  /user/*  /file/*  → php-fpm│
                          │  ├─ /static/  /app/*                    → 静态  │
                          │  └─ ^/[0-9a-z_]+/[0-9a-z_]+\.git        → :27555 │
                          └───────┬───────────────────────────┬──────────┘
                                  │ FastCGI                 │ HTTP(CGI)
                                  ▼                         ▼
                    ┌─────────────────────────┐   ┌──────────────────────┐
                    │ php-fpm :9000 (user=git)│   │ http-gateway (Go)   │
                    │  www/index.php          │   │  :27555 (localhost) │
                    │  CodeIgniter 2.1.1      │   └──────────┬───────────┘
                    │  api/*  apiv1/internal  │              │ cgi.Handler
                    └────┬──────────────┬────┘              ▼
                         │              │ GET+JSON body   git-http-backend
                         │              │ ◀────────────── (GIT_PROJECT_ROOT)
                         │              │
                         │        ┌─────▼──────────────────────────────┐
                         │        │ MySQL 5.7.31  codefever_community │
                         │        │ 17 张表 (cc_*)                     │
                         │        └────────────────────────────────────┘
                         │
   ssh -p 2222 ──▶ sshd :22
                  └─ authorized_keys 强制命令 ─▶ ssh-gateway/shell/main (Go)
                                                  │ GET+JSON body
                                                  └──▶ apiv1/internal/authSSH

   git push ─▶ <repo>/hooks/post-receive ─▶ misc/hooks/main (PHP CLI)
                                                  │ POST JSON + Token 头
                                                  └──▶ apiv1/internal/hooks
```

**进程清单（supervisor 实际管理）**：`nginx`、`php-fpm`、`sshd`、`cron`（基础镜像自带）+ `codefever-http-gateway`、`codefever-modify-authorized-keys`（仓库自带）。
`ssh-gateway/shell/main` 不常驻，由每次 SSH 连接经 `authorized_keys` 强制命令拉起。

---

## 2. 技术栈与职责

### 2.1 后端

| 层 | 技术 | 位置 | 规模 |
|---|---|---|---|
| 框架 | **CodeIgniter 2.1.1** | `system/core/CodeIgniter.php:36` | 整个 `system/` |
| 入口 | `www/index.php` | `www/index.php:8-21` | YAML → `YAML_*` 常量 |
| 控制器 | `api/{user,admin,group,repository}.php` + `apiv1/internal.php` | `application/controllers/` | 108 + 4 端点 |
| 模型 | 5 个 ActiveRecord 模型 | `application/models/` | `repository_model.php` 1977 行 |
| 服务层 | `application/libraries/service/`（11 个子包） | 自研命名空间层 | 框架胶水 + 业务规则混杂 |
| 事件 | 39 个事件 + 4 个 handler | `application/event_handlers/` | 同步内联执行 |
| 配置 | `env.yaml`（PHP+Go 共用）+ `config.yaml`（仅 PHP） | 仓库根 | 键结构见 §6.4 |
| 依赖 | `application/libraries/composerlib/vendor/`（Guzzle / Endroid QrCode / MaterialDesign Avatars） | **vendor 目录内嵌仓库** | 需随重写剥离 |

### 2.2 前端（**保持不动**）

| 项 | 事实 |
|---|---|
| 框架 | React 17.0.2 + Material-UI **v4.12.2** + redux 7 + react-router-dom 5 + react-intl 5 |
| 构建 | webpack 4.44.2（CRA-like eject 自有 `scripts/`） |
| **设计系统** | `@pgyer/mui-theme-preset` + `@pgyer/essential-component` 0.0.19 + `@pgyer/icons` 0.0.16 |
| 主题注入 | `src/config/colors/theme1.js:4` → `makePreset(originTheme, 'CodeFever')`；`src/components/App.js:27-30` 注入 `MuiThemeProvider` |
| 主色 | `#3455db`（`theme1.js:8`） |
| 全局样式 | 仅 `src/assets/styles/App.styl`（52 行，Stylus），其余全为 MUI `withStyles` 内联 |
| i18n | 100% 内置于 bundle：`src/lang/{zh-cn,en-us}/*.js`；**后端不消费前端 i18n** |
| 构建期变量 | `src/` 内 0 处 `process.env.*` → **后端重写无需注入任何前端变量** |
| 前端测试 | **0 个测试文件**，且 `package.json:152` 的 jest `testRunner` 硬编码他人机器绝对路径 → 本机 `yarn test` 不可用 |

> ⚠️ `@pgyer/mui-theme-preset` 在 `package.json:20` 声明为 `"latest"`。**重写期间严禁重装前端依赖**，否则 UI 会漂移。这是"UI 风格一致"的唯一实际风险点。

### 2.3 Go 网关（**保留**）

`http-gateway/`（HTTP Basic → CGI git-http-backend）与 `ssh-gateway/shell/`（SSH → git-upload-pack/receive-pack）。两者都通过 HTTP 回调 PHP 的 `/apiv1/internal/*` 获取鉴权结果。

---

## 3. 三条主链路

### 3.1 SPA → API

```
React → fetch('/api/<ctrl>/<action>')
      headers: Accept: application/json, codefever-end-env, codefever-end-lang
      POST/PUT/DELETE body: multipart/form-data（非 JSON！）
      GET query: + requestTime=<epoch ms>
   ↓ nginx → php-fpm → api/base.php::_remap → <action>_<method>()
   ↓ APIAuth::auth([web_user]) → CI session cookie 'codefever_community'
   ↓ Controller → Model → (git CLI)
   ← {"request-id":..., "code":0, "message":"Success", "data":{...}}
```

**关键事实**（上一轮曾误判，此处已实测纠正）：
- 前端**所有** POST/PUT/DELETE 用 `multipart/form-data`（`src/data_providers/main.js:18` + `src/helpers/VaribleHelper.js:9-25`），**不是 JSON**。
- PHP 的 `Request::parse()` 分派：`multipart/form-data`→`form`（`$_POST`）；`application/json`→`json`；`application/x-www-form-urlencoded`→`form-text`（同样是 `$_POST`）；无 Content-Type→`get`。
- `Accept` **只认** `application/json`（→json）与 `*/*`（→binary），其余一律 `0x0103`（400）。
- `Content-Length` 必须精确匹配 body 长度（chunked + 非空 body → `0x0104`）；仅 `multipart` 豁免该检查。

### 3.2 Git over HTTP

```
git clone http://host/<group>/<repo>.git
 → nginx location ~* ^\/[0-9a-z_]+\/[0-9a-z_]+\.git → :27555
 → http-gateway: User-Agent 必须 ^git → Basic Auth 必须存在
 → GET http://127.0.0.1/apiv1/internal/authHTTP  (GET + JSON body + Token 头)
     body: {user, pass, repo, action}
 ← data:{storage, uid, user, addr, path}
 → cgi.Handler{Path: cli.http, Dir: git-storage, Env: GIT_PROJECT_ROOT/REMOTE_USER/REMOTE_ADDR/PGYER_UID}
 → git-http-backend
```

失败映射：Go 侧把**一切非 404 的错误一律改写为 401**（`http-gateway/Request/main.go:145-147`）。所以后端必须用 **404** 表示"仓库不存在"，其余（403 凭据错、400 参数缺）最终都变 401。

### 3.3 Git over SSH

```
ssh -p 2222 git@host  (git-upload-pack 'group/repo.git')
 → authorized_keys 强制命令 → ssh-gateway/shell/main
 → GET /apiv1/internal/authSSH  body:{userID, repo, action}
 ← data:{uid, path}
 → PGYER_UID=<u_key>|<r_key>; exec git-upload-pack <path>
 → push 时 <repo>/hooks/post-receive → misc/hooks/main
 → POST /apiv1/internal/hooks  body:{uid, args, stdin} + Token 头
 ← code==0 → exit 0；code!=0 → 打印 data.remote 并 exit 255
```

**跨进程硬契约**：`PGYER_UID` 的 `"<u_key>|<r_key>"` 两段格式（`internal.php:145-146` 直接 `explode('|')[0]/[1]`）贯穿 authHTTP/authSSH → CGI env / os.Setenv → hooks 脚本 → 后端。任何一段改了格式，push 后端就找不到用户与仓库。

---

## 4. 架构问题诊断

### 4.1 分层与耦合

| 问题 | 证据 | 后果 |
|---|---|---|
| **模型层同时是 git 进程执行器** | `repository_model.php:619-709` `execCommand()` 直接 `exec()` shell | 业务逻辑与系统调用混杂，无法独立测试；所有 git 操作都带命令注入风险 |
| **隐式跨模型依赖** | `notification_model.php:197,233,238` 用 `$this->repositoryModel`，但该模型**从不加载** Repository_model；靠 `CI_Model::__get` 代理到 CI 超对象 | **通知拒收/恢复功能在 API 路径上实际已损坏**（对 NULL 调用方法 = 致命错误）。同类问题还有 `group_model.php:43` 依赖 User_model（靠 Repository_model 构造函数间接注册） |
| **服务层反向依赖框架** | `libraries/Service.php:27,39,48` 用 `$this->CI->load->model(...)` | 无法脱离 CI 测试 |
| **鉴权配置被模型直接引用** | `repository_model.php:9` `use service\AccessControl\UserAccessController`，角色常量用于数据写入 | 权限模型与存储层耦合 |
| **控制器直接触发 shell** | `api/repository.php:53-59` → model → `Command::batch()` → `exec()` | 分层形同虚设 |

### 4.2 数据一致性

| 问题 | 证据 | 后果 |
|---|---|---|
| **全仓零事务** | grep `trans_start\|trans_commit\|trans_begin` 于 `application/**` → 0 命中 | MR 创建（`repository_model.php:254-277` 用 `SELECT max(mr_id)+1` **手工分配 ID**）、通知双写（`notification_model.php:106,117`）、changeOwner 三步（`api/group.php:454-468`）均无原子性；并发建 MR **必然撞号** |
| **fork 补偿失效** | `repository_model.php:974` `execCommand` 返回值**被丢弃** | `git clone --bare` 失败会留下"有 DB 行、无磁盘仓库"的孤儿记录 |
| **缓存永不失效** | `CacheData/Store.php` 是**纯进程内数组**（不是 Redis/文件/APCu），且全仓**无任何 delete 调用** | `repository_model.php:979-988` / `group_model.php:256-261` / `user_model.php:273-284` 更新后**不更新缓存**，同请求内读到旧值 |
| **N+1 查询** | `repository_model.php:24-49` 每仓库 2 条 SQL；`:51-67` 每成员 2 条；`api/repository.php:157` 列表接口内对每仓库调 `getMembers()` | 仓库列表页 = O(仓库数 × 成员数) 条 SQL |
| **`batch()` 丢弃退出码** | `Command.php:33-35` `$return` 未使用 | "成功但无输出"与"失败"不可区分；`repository_model.php:929-935` 靠 git init 恰好有 stdout 才正确 |

### 4.3 安全

| 严重度 | 问题 | 证据 |
|---|---|---|
| 🔴 严重 | **密码 `md5(md5())` 无盐** | `user_model.php:66,79,145`；`user.php:168,228`；`internal.php:91` |
| 🔴 严重 | **命令注入**：`wrapArgument` 只反斜杠转义、不加引号，且不覆盖 `> < ( ) !` 等；`--grep=$keyword`（用户搜索词）、`deleteTag` 的 tag 名**完全未转义** | `Command.php:44-61`；`Git/Command.php:193-227, 80-83` |
| 🔴 严重 | **网关 token 纯字符串 `===` 比较**，无 HMAC/时间戳/nonce/常量时间 | `APIAuth.php:18` |
| 🟠 高 | **11 处越权读取缺口** | `commitDetail` / `lastCommitLog` / `refListContainSHA` / `mergeRequestCommits` / `targetRepository` / `getRepositoryWebhookLogs` / `getRepositoryWebhookLogData` 无资源级鉴权；`fileChanges` **传 `mergeRequest` 参数即完全跳过读权限校验**；`group/activities` 不校验；`blameInfo` 误用 `UAC_MR_READ` |
| 🟠 高 | **webhook 归属未校验** | `api/repository.php:2099-2107, 2138-2144` 只按 `rwKey` 改删，可操作他人仓库的 webhook |
| 🟠 高 | `cookie_secure=FALSE` + `csrf_protection=FALSE` + session 10 天有效 + 不绑 IP/UA | `config.php:271,296,253,254` |
| 🟠 高 | TOTP secret **明文**存 `u_2fa` | `user_model.php:88-100` |
| 🟡 中 | 仓库目录 `chmod -R 0777`；`ln -s ../../misc/hooks` 相对软链；`db_debug=TRUE`；`sql_log_enabled=true` 全量落盘写操作 SQL | `repository_model.php:654-655`；`database.php:59`；`config.php:365-366` |

### 4.4 正确性缺陷（会直接产生错误结果）

| 缺陷 | 证据 | 表现 |
|---|---|---|
| **建分支/建标签成功反而报错** | `api/repository.php:1442-1445`、`:1952-1955`：`if ($result) { Response::reject(0x040C); }`，而 model 的 `createBranch`/`createTag` **成功时返回 true** | 成功 → `0x040C Old Target Not Exists`；失败 → 返回成功空对象 |
| **query 分页被静默改写** | `Request.php:77-92` 用 `is_int($_GET['page'])` 判定，而 `$_GET` 值恒为字符串 | 走 query 的 23 个分页端点**永远只返回第 1 页**（page=1, pagesize=20） |
| **`paging` 字段永不出现** | `Response::setTotal()` 全仓无调用点 | 前端拿不到总数 |
| **零日期列在 MySQL 8 不可建表** | `db_structure.sql` 14 处 `timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'` | 见 §5.2，已实测 |
| **admin 关闭 MFA 列表永远为空** | `user_model.php:315` 用 `u_2fa IS NULL` 筛选，但关闭时写入的是**空字符串** | 筛选条件与写入值不匹配 |
| **参数缺失 → PHP 8 TypeError → 500** | `internal.php:144` `count(null)`；`admin.php:480` `array_merge($config, null)`；`repository.php:980` `$commitLog['parent']` | 截断的 JSON 响应（Content-Length 已设但 body 不全），前端解析失败 |
| **无全局异常处理器** | `www/index.php` 无 `set_exception_handler`；`application/hooks/`、`application/core/` **均为空壳** | 生产环境 `error_reporting(0)` 掩盖致命错误，异常不进 API 日志 |

### 4.5 可观测性

- 日志：`application/logs/` 按 scope 分文件追加（`Logger.php:17-32`），**无结构化、无请求 ID 串联**。
- 事件分发日志存在，但**未捕获异常不经过 `Response::_response()` → 不进 API 日志**。
- `admin/systemStatus` 手工执行 `vmstat` / `free -m` / `df` / `ps` 判断服务状态——**实测面板显示 `Crond: Stopped`、`SendMail: Stopped`**，而这两项确实未运行（邮件依赖本地 MTA）。
- `db_debug=TRUE` + `sql_log_only_modify=true`：生产环境把写操作 SQL 全量落盘（既是性能负担也是信息泄露）。

### 4.6 部署与运维

| 问题 | 证据 |
|---|---|
| Go 网关**每请求重读 `env.yaml`**，且写入包级变量 → 并发写竞态 | `http-gateway/Request/main.go:90,115` |
| Go 客户端**无超时、无重试**；gzip 解压失败直接 `panic` 终止整个网关进程 | `JsonHttpClient/main.go:25,45` |
| `misc/hooks/main` 的 stdin 是**非阻塞读取**，大 payload 有截断风险 | `misc/hooks/main:26` |
| ~~`env.yaml` / `config.yaml` 不挂载卷~~ → 已修复：当前使用 `config-data` 命名卷持久化 | `docker-compose.yml` |
| `path.git` 配置键**定义了但从未被读取**，代码硬编码 `/git-storage` | grep `YAML_PATH_GIT` → 0 命中 |
| 仓库名字符集**三层不一致**（历史缺陷，已修复） | 当前 nginx vhost、Go 网关/后端统一为 `[0-9a-z_.-]+`，带连字符的仓库名经 HTTP 可正常访问 |

---

## 5. 已实测的运行态事实

### 5.1 当前栈（验证于本次分析）

| 项 | 状态 |
|---|---|
| `codefever-app` | Up 10h（镜像 `codefever-community:local`），9080→80、2222→22 |
| `codefever-db` | Up 10h（healthy），mysql:5.7.31 |
| supervisor | nginx / php-fpm / sshd / crond / codefever-http-gateway / codefever-modify-authorized-keys 全部 RUNNING |
| Web UI | 200；SPA `/app/index.html` 200；`/static/js/main.e442a514.js` 200（568,642 bytes） |
| Git | HTTP 与 SSH 的 clone/commit/push 均成功，pre-receive·update·post-receive 钩子回调全部 Success |

### 5.2 MySQL 8.0.46 兼容性（**实测**）

在 `mysql:8.0.46` 容器上加载现有 schema，**三处硬阻塞**：

| # | 阻塞点 | 实测报错 | 根因 |
|---|---|---|---|
| 1 | `misc/docker/docker-entrypoint.sh:23` | `ERROR 1231 (42000): Variable 'sql_mode' can't be set to the value of 'NO_AUTO_CREATE_USER'` | 该模式 **MySQL 8.0.3+ 已删除** |
| 2 | `misc/db_structure.sql`（14 处）+ `misc/db_update_20220215.sql`（1 处） | `ERROR 1067 (42000): Invalid default value for 'a_updated'` | `timestamp NOT NULL DEFAULT '0000-00-00 00:00:00'` 撞上 MySQL 8 默认 `NO_ZERO_DATE`/`NO_ZERO_IN_DATE`；且 MySQL 8 默认 `explicit_defaults_for_timestamp=ON`，`TIMESTAMP NOT NULL DEFAULT NULL` 同样非法 → **0 张表建成** |
| 3 | `misc/db_data.sql`（5 处） | `ERROR 1292 (22007): Incorrect datetime value: '0000-00-00 00:00:00' for column 'u_updated'` | 种子数据里的零日期字面量 |

**后续深挖结论（实测）**：上述 2、3 两个"阻塞"其实**不需要改 DDL 或种子数据**。本项目的零日期是**有语义**的：

- PHP 侧 `strtotime('0000-00-00 00:00:00')` 返回 **-62169984000**（**不是 false**），该值经 `(int)` 转换后进入 API 响应（`repository_model.php:39`、`group_model.php:31`、`repository_model.php:2138`）。
- 因此把零日期改成 `NULL`（→ 0）或 `0000-01-01`（→ -62167219200）都会**改变前端可见的 `updated` 字段**。

正确做法是**放开 MySQL 8 默认新增的 `NO_ZERO_DATE` / `NO_ZERO_IN_DATE`**（而不是改写数据）：

**最终修复（仅 1 处，已应用并实测跑通）**：

```sh
# misc/docker/docker-entrypoint.sh:23
# 移除 NO_AUTO_CREATE_USER（8.0.3+ 已删除，否则 ERROR 1231）
# 移除 NO_ZERO_DATE / NO_ZERO_IN_DATE（保留零日期语义）
set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';
```

实测结果（**DDL 与种子数据零改动**）：
- **17 张表全部建成**；
- 种子数据 `3 用户 / 1 组 / 1 仓库 / 1 SSH key`，管理员 `root@codefever.cn`（`u_admin=1`），密码哈希 `14e1b600b1fd579f47433b88e8d85291` = `md5(md5('123456'))`；
- 零日期值原样保留（`0000-00-00 00:00:00`）；
- `datetime NOT NULL` 的隐式默认在 5.7 与 8.0 下**同为** `0001-01-01 00:00:00`，无需处理；
- 从线上 5.7 `mysqldump` 导入 8.0 后，计数一致（3 用户 / 2 组 / 3 仓库 / 1 SSH key）。

> 附带发现：`GROUPS` 在 MySQL 8.0.2+ 是**保留字**（窗口函数帧单位）。表名 `cc_groups` 带前缀不受影响，但重写后的 SQL 里任何裸 `groups` 别名都会语法错误——本次实测即被该保留字绊了一次。

### 5.3 UI 风格基线（浏览器取证）

内置浏览器实测 5 页，均正常渲染（登录由用户在浏览器中接管完成）：

| 页面 | URL | 基线要点 |
|---|---|---|
| 登录 | `/user/login` | **PHP 服务端渲染**（非 SPA！响应 8471 字节的 HTML，不含 `id="root"` 与 `static/js/main`）；白底居中卡片；`欢迎登录` / `CodeFever`；`btn-primary-_pgyer_codefever_` 主按钮；左侧留白 360px 输入框；引用 `/static/vendor/{bootstrap-4.4.1,fontawesome-free-5.13.0,intltel,toastr}` |
| 仓库列表 | `/repositories` | CODE FEVER COMMUNITY SVG logo；顶栏 仓库/仓库组 下拉 + 6 个图标 + 头像下拉；Tab 栏（所有/我参与的/我创建的）选中态 `fontWeight:600` + 主色下划线；右对齐主按钮「新建仓库」 |
| 仓库详情 | `/rootteam/demorepo` | 左侧 320px 侧栏（文件/提交/合并请求/分支/标签/成员/动态/设置）+ 主区（面包屑标题 + Fork/克隆按钮 + 统计条 + 分支选择器 + 文件表格 + README 预览） |
| SSH Keys | `/settings/sshkey` | 左侧设置侧栏（个人信息/多邮箱/SSH Keys/通知）；主区标题 `SSH Keys` + 文本域 + 名称 + 主按钮 + 折叠面板 |
| 管理后台 | `/admin/dashboard` | 左侧 概览/用户/仓库组/仓库/设置；主区彩色统计卡（蓝=用户/组/库，绿=CPU/RAM/磁盘，红=服务异常） |

**DOM 层面确认**：类名为 `MuiGrid-root` / `MuiAvatar-root` / `MuiTypography-root` / `jss65` → Material-UI v4 + JSS，**与 `@pgyer/mui-theme-preset` 注入的 `MuiThemeProvider` 一致**。这证明"SPA 部分的 UI 风格"由前端 bundle 完全决定，后端重写不影响视觉。

### 5.4 关键发现：存在**第二套 UI**——PHP 服务端渲染页面

"前端 UI 风格保持一致"这条约束，**不只覆盖 React SPA**。仓库里还有 9 个由 PHP 服务端渲染的页面，它们是**独立的视觉体系**（Bootstrap 4.4.1 + FontAwesome 5.13 + jQuery 3.5 + toastr），与 SPA 的 MUI 完全不是一套：

| 模板 | 行数 | 用途 |
|---|---|---|
| `application/views/newpage/user/login.php` | 153 | `/user/login` |
| `application/views/newpage/user/register.php` | 197 | `/user/register` |
| `application/views/newpage/user/reset_password.php` | 231 | `/user/resetPassword` |
| `application/views/newpage/user/verify.php` | 142 | `/user/verify`（2FA） |
| `application/views/newpage/user/countries_dropdown_menu.php` | 28 | 国家下拉（数据源 `config/countries_item.php` 1287 行） |
| `application/views/newpage/header_include.php` / `footer_include.php` | 29 / 22 | 公共头尾 |
| `application/views/newpage/login_activity.php` | 3 | 登录后跳转 |
| `application/views/doc/detail.php` | 130 | 文档详情 |
| `application/views/base/{error,success,disable}_message.php` | 108 | 通用提示页 |
| `application/views/nopage/index.php` | 24 | 未知路径的 404 落地页 |

**关键事实**：
- `/user/login` 返回 **8471 字节的 PHP 渲染 HTML**，**不含** `id="root"`、**不含** `static/js/main` → 确认它**不是** SPA。
- 这 9 个页面的响应信封也与 `/api/*` 不同：`{code, message, extra}`（`application/controllers/base.php:101-108` 的 `_ajax_message()`），**无 `data`、无 `request-id`**；登录成功返回 `{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}`，且响应头是 `Content-Type: text/html; charset=UTF-8`（不是 application/json）。
- 它们使用的静态资源在 `www/static/vendor/`（bootstrap-4.4.1 / fontawesome-free-5.13.0 / intltel / jquery-3.5.0 / jquery-validation-1.19.1 / poper-1.16.0 / toastr）与 `www/static/images/`，**由 nginx 直接托管**。

⇒ **重写必须把这些页面一并移植到 Go 模板**（逐字保持 DOM 与资源引用），否则"UI 风格一致"会在登录/注册环节破功。详见重写方案 §5.6 与 §9。

---

## 6. 可优化点清单

### P0 — 安全与正确性（重写时必须修掉）

| # | 问题 | 位置 | 建议 |
|---|---|---|---|
| P0-1 | md5(md5()) 无盐密码 | `user_model.php:66,79,145` | 改 **argon2id/bcrypt**；保留旧哈希校验分支，登录成功即透明升级（否则全量用户锁死） |
| P0-2 | git 命令注入 | `Command.php:44-61`、`Git/Command.php:193-227,80-83` | 改 **argv 数组调用**（`exec.Command`，不经 shell），彻底移除 `wrapArgument` |
| P0-3 | 11 处越权读取 + webhook 归属未校验 | `api/repository.php` 多处 | 统一在 handler 层做资源级鉴权（角色 → 权限位 → 资源实例） |
| P0-4 | 网关 token 非恒定时间、无防重放 | `APIAuth.php:18` | 改 `hmac.Equal` + 请求签名（时间戳 + nonce） |
| P0-5 | 建分支/建标签成功反而报错 | `api/repository.php:1442-1445,1952-1955` | 修逻辑反转（重写时由契约测试锁定） |
| P0-6 | 零事务导致的脏数据 | `repository_model.php:254-277` 等 | 引入显式事务边界；MR ID 改 DB 自增或 `INSERT ... ON DUPLICATE` |
| P0-7 | 无全局异常处理器 | `www/index.php` | 统一 panic recovery → 标准错误信封 + 结构化日志 |
| P0-8 | `cookie_secure=FALSE`、`csrf_protection=FALSE` | `config.php:271,296` | HTTPS 下开 secure；同源 + `SameSite=Lax` + CSRF token（前端已固定 `credentials:'same-origin'`，加 CSRF 头不影响 UI） |

### P1 — 架构与性能

| # | 问题 | 位置 | 建议 |
|---|---|---|---|
| P1-1 | 模型层兼任 git 进程执行器（1977 行） | `repository_model.php:619-709` | 拆分为 `domain/` + `git/` 两个包，git 层只暴露领域方法 |
| P1-2 | 隐式跨模型依赖已造成功能损坏 | `notification_model.php:197,233,238` | 显式依赖注入；该 bug 需专项回归测试 |
| P1-3 | N+1 查询 | `repository_model.php:24-49,51-67`、`api/repository.php:157` | 批量查询 + `IN` 聚合；仓库列表单查询完成 |
| P1-4 | 缓存是进程内数组且永不失效 | `CacheData/Store.php` | 换成带 TTL 的缓存（Redis 或内存 TTL map）；更新时主动失效 |
| P1-5 | 事件/通知同步内联阻塞请求 | `EventDispatcher.php:16-42` | 事件写入 DB 后由 worker 异步消费（邮件尤其） |
| P1-6 | 无事务、无幂等的 webhook 投递 | `backend/repository_webhook.php` | 加重试退避 + 死信 + 签名版本号 |
| P1-7 | `Command::batch()` 丢弃退出码 | `Command.php:33-35` | 以 `exec.ExitError` 判定成败 |
| P1-8 | Go 网关无超时/重试 + 包级变量竞态 + gzip panic | `JsonHttpClient/main.go:25,45`、`Request/main.go:90,115` | 设 `Timeout`、`hmac` 化、配置只读化、panic → 返回 5xx |
| P1-9 | `path.git` 配置键与实际路径脱节 | grep `YAML_PATH_GIT` → 0 | 统一从配置读取 |
| P1-10 | 仓库名字符集三层不一致 | nginx / Go / PHP 三处正则 | 统一为 `[0-9a-zA-Z_.-]+` 并让三端同步；PHP 的 `[\w\d_]+` 需放开 `-` |

### P2 — 可维护性与工程化

| # | 问题 | 建议 |
|---|---|---|
| P2-1 | **测试几乎为零**（控制器 0 覆盖、鉴权 0 覆盖、ErrorCode 0 覆盖、git shell 路径 0 覆盖） | 重写时**契约快照测试**作为第一交付物：对着现有 PHP 后端跑一遍 108 端点，固化 JSON fixture，新后端做 diff |
| P2-2 | `vendor/` 内嵌仓库 | 依赖清单化（go.mod / 未来 package.json） |
| P2-3 | `application/libraries/api_controller.php`（163 行）大半是死代码、`config/rest.php`（271 行）不可达分支 | 直接丢弃 |
| P2-4 | 硬编码 `'/tmp/workspace'`、`'/usr/local/php/bin/php'`、`http://127.0.0.1:27555` | 全部入配置 |
| P2-5 | `constants.php:24-44` 把测试数据写进生产代码 | 迁到测试 fixtures |
| P2-6 | `env.yaml`/`config.yaml` 不挂载卷 | 改为卷挂载 + 密钥文件只读挂载 |
| P2-7 | `package.json:152` jest `testRunner` 硬编码他人机器路径；`setupTests.js` 缺失 | 修复后可启用前端回归（但**前端本身不改动**） |
| P2-8 | 前端 0 测试 | 前端不动 → 用**浏览器视觉回归**替代（见重写方案 §11） |

---

## 7. 重写时必须保留的资产（业务规则 vs 框架胶水）

### 7.1 业务规则（原样搬运，不可丢）

| 资产 | 位置 | 说明 |
|---|---|---|
| **权限矩阵** | `application/config/access_controller.php:10-92` | 39 个 `UAC_*` 权限码 + 6 个 `ROLE_*` + `PERMISSION_SETTING` + `REPO_ACTION_MAPPING`。**最核心资产** |
| 角色继承链 | `repository_model.php:727-757`、`group_model.php:136-155`、`Service.php:25-72` | owner → repo member → group member → 无权限 |
| git 动作→权限映射 | `access_controller.php:88-92` | `''`/`git-upload-pack`→READ；`git-receive-pack`→PUSH |
| 保护分支规则匹配 | `repository_model.php:2039-2079` | 正则或精确匹配；push/merge 双最低角色；`role < ROLE_DEVELOPER` 一律受保护 |
| 分组类型语义 | `constants.php:124-125` | `GROUP_TYPE_NORMAL=1` / `GROUP_TYPE_USER=2` |
| 通知位掩码语义 | `constants.php:162-164` | `MR=1`、`EMAIL=2`；**位=1 表示关闭** |
| 通知拒收级联 | `notification_model.php:180-206` | 拒收组 → 自动拒收该组全部仓库；恢复仓库 → 同时恢复所属组 |
| 通知受众阈值 | `UserNotificationHandler.php:202-231` | MR 事件 `role > ROLE_REPORTER`；评审事件 `role > ROLE_DEVELOPER`；已指定 reviewer 时清空广播 |
| 通知模板（中英） | `UserNotificationHandler.php:22-138` | `{{$user@users:u_name}}` 占位 DSL |
| 占位符解析引擎 | `Utility/MessageGenerator.php` | `{tableName}:{primaryKeyName}` 映射 |
| MR 状态机 | `Constant/MergeRequestStatus.php` | OPEN / MERGED / CLOSED |
| 活动类型区间编码 | `repository_model.php:789-801`、`group_model.php:228-240` | `0x0401/0x0402/0x0501/0x0502`=commit；`0x07xx`=MR；`<0x06xx`=member |
| 错误码目录 + HTTP 映射 | `ErrorCode/ErrorCode.php:7-48` | 31 项；**前端 `ErrorCodeParser.js` 逐个依赖其数值语义** |
| 事件注册表 + 数据 schema | `Constant/EventType.php` | 39 事件 + `主事件:子事件` 命名 + `_D` 描述 DSL |
| git 输出 JSON 协议 | `Utility/Helper.php:25-29` + `Git/Command.php` 各 prettyPattern | 分隔符 `\ue000\ue001\ue000` + 字段集合 |
| slug / 显示名唯一性 | `api/repository.php:42-50,107-115`、`api/group.php` | 同组内 `r_name` / `r_display_name` 唯一（0x0401–0x0404） |
| 合并模式语义 | `repository_model.php:1017-1110` | `message=NULL`→FF；`message`+`squash=FALSE`→`--no-ff`；`message`+`squash=TRUE`→`--squash`+commit |
| 提交身份归属 | `repository_model.php:644-645,1049-1052` | `GIT_COMMITTER_NAME`=邮箱 `@` 前段；`GIT_COMMITTER_EMAIL`=邮箱 |
| Webhook 签名与载荷 | `backend/repository_webhook.php:8-16,123-126,67-76` | `md5(body . secret)` + `X-CodeFever-*` 头 |
| 主键生成规则 | `Utility/UUID.php:16-18` | `md5(time()+rand()+rand()+rand()+uniqid())` → 32 位 hex |
| 仓库磁盘布局 | `repository_model.php:641-642,918` | `<git-storage>/<32-hex>` 扁平单层；全部 bare；旧仓库为 `hooks → ../../misc/hooks`，新仓库为 `../../deploy/hooks`（镜像保留两者兼容软链） |
| 展示限制阈值 | `constants.php:108-111` | `MAX_DISPLAY_DIFF_FILE=100`、`MAX_DISPLAY_DIFF_LINE=500`、`MAX_DISPLAY_DIFF_LENGTH=0x2000`、`MAX_DISPLAY_FILE_SIZE=0x200000` |
| 分页默认值 | `constants.php:120-121` | `DEFAULT_PAGE_SIZE=20`、`DEFAULT_PAGE_SIZE_MAX=100` |
| SSH 公钥指纹算法 | `Utility/Helper.php:44-47` | `hash('md5', base64_decode($sshKey))` |
| 保留 URI 正则 | `constants.php:128-135` | `PRESERVE_URI` 6 条（feature/doc/community/api/user/group/repository/setting/admin…） |

### 7.2 框架胶水（重写时丢弃）

CI2 ActiveRecord、CI2 Cookie Session（`md5(sha1())+mcrypt`）、`service\Network\Request`/`Response` 静态单例、`APIAuth::$CI` 静态、`ErrorCode` 私有静态表、`MessageGenerator::$databaseConfig` 公有可写静态、`Command`（shell 拼接执行器）、`Workspace`、`EventDispatcher`+`autoLoader` 特判映射、`_validateEvent` 字符串 DSL、`$this->load->model(..., $alias)` 全局别名、各 model's `normalize*()` 数组整形方法、`_remap` 动态派发、`Logger` 文件追加、`api_controller.php`、`config/rest.php`、`application/{core,hooks,cache}/` 空壳、`application/language/`、`application/views/`（服务端渲染页）。

---

## 8. 附：本次分析的边界

- 未逐行阅读 `repository_model.php` 的 1123–1259、1560–2038 行区间；N+1 与 git 调用的结论基于抽样。
- `config/schedule.php`、`config/storage.php`、`config/lang.php` 未逐一展开（仅确认被引用）。
- `application/libraries/Captcha.php`、`Crypt/*`、`Avatars/*` 未细查。
- `searchPublicKey` 在仓库内无调用方，其消费方为外部 SSH 公钥查询服务，契约按 `internal.php:21-37` 推定。
- `git-http-backend` 的 `GIT_PROJECT_ROOT` + `PATH_INFO` 精确解析语义由 git 自身决定，未在仓库内验证；重写时**不得改变这两者的赋值方式**。
