# 更新日志

本文件记录 CodeFever Next 的重要变更。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## Unreleased

尚未发布的变更集中在此。

### 变更

- **仓库结构收敛为单一 Go module**：`http-gateway/` 与 `ssh-gateway/` 并入
  `backend/cmd/codefever-http-gateway`、`backend/cmd/codefever-ssh-gateway`；
  `misc/` 收敛到 `backend/deploy/{sql,docker}`。构建统一为
  `cd backend && go build ./...`，不再需要 GOPATH 模式或构建期 `sed` 改写 import。
- **镜像改为 5 个二进制**：`codefever-api`、`codefever-worker`、`codefever-hook`、
  `codefever-http-gateway`、`codefever-ssh-gateway`；旧路径
  `misc/hooks`、`ssh-gateway/shell/main` 保留兼容软链。
- **配置持久化**：`config.yaml` / `env.yaml` 改为 `<base>/config/` 的软链，
  默认落在仓库根目录的 `codefever-data/config/`；管理后台修改的主机名、邮件设置等
  不再因容器重建丢失。
- **数据目录改为 `codefever-data/`**：MySQL、裸仓库、附件、配置全部绑定到仓库根目录
  （`codefever-data/{mysql,git-storage,file-storage,config}`），可用
  `CODEFEVER_DATA_DIR` 覆盖；不再使用 Docker 命名卷，备份/迁移只需处理一个目录。
- **站点地址自动生成**：入口脚本按 `CODEFEVER_HTTP_PORT` / `CODEFEVER_SSH_PORT`
  写入 `host` / `ssh`，支持 `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH` 覆盖。
- **邮件 SMTP**：新增 `CODEFEVER_SMTP_*`；未配置时回退系统 `mail`（镜像不含 MTA）。
- **安全默认值**：镜像构建期随机生成 `session.key`、`totp.salt`、`gateway.token`；
  `config.yaml` / `env.yaml` 权限降为 `0660`，不再用 `sed` 拼接 DB 口令；
  数据库参数同时支持 `CODEFEVER_MYSQL_*` 与旧 `DB_*`。
- **迁移收尾修复**：
  - `post-receive` 现在写入 `HOOK_POST_RECEIVE` 事件，push 后活动流 / 通知 / webhook 恢复；
  - `group` 域事件总线在生产装配中正确地注入（此前 10 个 group 事件被静默丢弃）；
  - MR 详情返回评审人；fork MR 的跨仓库 `fileChanges` / `mergeRequestCommits` 已实现；
  - 禁用账号不能登录/继续使用旧会话，改密/管理员重置口令会吊销会话；
  - 事件 worker 增加租约回收与 panic 兜底，避免事件永久卡在 Processing；
  - MR 合并使用 MySQL 命名锁串行化，避免并发重复合并；
  - HTTP 网关删除危险的"内部模式"鉴权旁路，POST push 会按 `git-receive-pack`
    校验 PUSH 权限；SSH 公钥写入拒绝换行/NUL 注入并改为原子替换；
  - 请求体上限、git 命令兜底超时、数据库连接/读写超时、`/api/*` 尾斜杠兼容；
  - **默认分支为 main 的仓库 clone 修复**：`git init --bare` 的 HEAD 默认指向
    `master`，首次 push `main` 后 clone 会报 `remote HEAD refers to nonexistent ref`
    且工作区为空。现在 push / 修改默认分支 / 服务启动时会自动把 HEAD 指向真实分支。
- **依赖**：`backend/go.mod` 降到 `go 1.23.0`（Docker 仍用 Go 1.25 构建）。
- **文档进镜像**：Dockerfile `COPY doc/` 与 `COPY LICENSE`，应用内文档中心可用。
- **CI 已恢复**。`.github/workflows/ci.yml` 提供 Go 1.23 的 gofmt / build /
  Linux 交叉编译 / vet / test 与 `docker compose config`；本地对应门禁为
  `scripts/verify.ps1`（同样的最小集，可选 `-BuildImage`）。Dependabot 仍未配置。
- **README/安装/管理/安全文档按当前结构重写**：删除 `misc/`、`http-gateway/`、
  `ssh-gateway/`、`Dockerfile.local`、`docker-compose.local.yml`、`CODEFEVER_BACKEND`
  等旧路径/旧开关；英文安装文档与 Webhook 页面同步更新。

### 新增

- **Web 性能指标上报**：前端接入 `web-vitals`（CLS / FID / FCP / LCP / TTFB），
  通过 `sendBeacon` 发到新增的 `POST /api/web-vitals`，只写日志不入库。
  之所以要它：此前所有前端性能结论只能靠人工复测，无法在发布前后做同口径对比。
- **追加表保留期清理**：新增每日任务，清理已处理事件（30 天）、死信事件（60 天）、
  已读站内信及其孤儿内容行（90 天）、webhook 投递日志（90 天），全部按批 `LIMIT`，
  天数可用 `CODEFEVER_RETENTION_*` 覆盖。**待处理与处理中事件永不删除**，
  `cc_activities`（用户可见的仓库动态）默认不清理。
  此前除 `cc_sessions` 外没有任何清理，这五张表会无限增长。

### 性能

- **首屏 JavaScript 减半**：`1,699,347 B → 871,055 B`（-48.7%），gzip
  `454,176 B → 263,310 B`（-42%），前端产物目录 `7.9 MB → 1.8 MB`。三项来源：
  1. 路由级代码分割：34 个页面全部改为 `React.lazy`，按业务域切成 7 个 chunk
     （此前是 2 个文件、登录页也要下载后台与 Markdown 全家桶）；
  2. 依赖裁剪：`@pgyer/icons` 引入方式收敛到项目内的 `src/config/icons.js`
     （该包无 `sideEffects`，barrel 会把 154 个图标全部打进包，实际只用 59 个；
     该注册表按路径引入后统一包装成 FontAwesome 图标定义，是**唯一**允许的
     图标入口）；Prism 语言 56 → 36 种，
     并补齐了原本静默回落 markdown 高亮的 8 种语言；
  3. `react-markdown`（约 172 KB）改为按需加载，`github-markdown-css` 本地内置
     （不再从第三方 CDN 取样式）。
- **静态资源开启 gzip 与分级缓存**：此前全站无压缩、无 `Cache-Control`。
  现在 gzip 使 vendor chunk 传输 `719,449 B → 227,452 B`；静态资源按是否带内容
  哈希分档缓存（哈希产物一年 immutable，其余 7 天 + `stale-while-revalidate`，
  SPA 壳 `no-cache`）。二次访问不再重复下载。
- **SPA 脚本改为 `defer`**：此前两个 `<script>` 阻塞 HTML 解析直到 1.7 MB
  下载并执行完毕。
- **登录页关键路径瘦身**：`marked.min.js`（47.6 KB）改为仅文档页加载；
  登录页 LCP 大图由 147.9 KB 的 PNG 改为 WebP（30.5 KB，-79%）。
- **后端查询与连接池**：
  - 列表接口 `pagesize` 增加全站上限（经 `domain.ClampPaging`，上限 100）。
    此前只设下限，`?pagesize=20000` 会拼出无上限 `LIMIT`，而合并请求列表对每行
    都要跑一次 `git log` 子进程 —— 单个请求可拉起两万个进程；
  - 管理端用户列表由 `2 + 20×5 = 102` 条查询降为 **3** 条；
  - 通知 worker 的拒收过滤由「每收件人 2 次 `COUNT(*)`」降为 **1** 条批量查询
    （200 人群组事件：400 次 → 1 次）；
  - 鉴权不再重复查库：每个已登录请求对 `cc_sessions` / `cc_users` 的查询
    由各 2 次降为各 1 次；
  - 6 个迁移文件补齐 12 项索引（覆盖 9 张表），其中 `cc_users.u_tel` / `u_status`
    此前**完全没有索引**，手机号注册与按状态统计走全表扫描；
  - `git clone` 移出数据库事务，事务内跨连接查询清零。此前 25 路并发 fork 即可
    抽干连接池（`MaxOpenConns=25`），随后全站阻塞在等连接上。
- **MySQL 慢查询可观测**：开启 `slow_query_log`、`long_query_time` 10s → 0.2s。
  此前生产环境没有任何慢查询数据。

### 修复

- **admin 端点访问日志双写**：17 个管理端点此前被注册为 page route 并二次分发，
  每个请求走两层中间件，同一 `request_id` 出现两条完全相同的日志
  （请求体限制与 panic 恢复也各套两层）。根因（`contract.Parse` 先读完请求体
  导致 multipart 字段丢失）早已修复，现直接删除该补丁。
- **确认框队列「移除」从未生效**：`confirmation.remove` 分支用
  `typeof id === 'number'` 判断，而取到的是对象，条件恒为 false，是死代码；
  同时原实现用 `delete` 删除数组元素会留下空洞，后续 `shift()` 可能返回
  `undefined` 并在读取 `.data.title` 时抛异常。现改为按 key 过滤的不可变更新。
- **仓库头像更新事件里的 `from` 取值错误**：原先在事务内跨连接重读仓库行，
  读不到未提交的 `UPDATE`，导致事件中 `from ≠ to`，与既有语义（同一连接
  update 后重读，`from == to`）不符。
- **检索关键字未转义**：活动流与合并请求搜索的两处 `LIKE` 未转义通配符，
  `?keyword=%` 会退化为全表扫描。
- **管理端分页排序不稳定**：`sort` 未命中白名单时不拼 `ORDER BY`，导致
  `LIMIT/OFFSET` 无序扫描、翻页可能重复或漏行；现回落到创建时间并追加主键兜底。
- **前端若干渲染期问题**：三个 Redux reducer 对无关 action 也返回新对象，
  使每次 API 响应都触发全部 connected 组件重渲染；`Main.js` 在
  `shouldComponentUpdate` 里 dispatch 且无 `groupName` 的路由每次渲染都触发；
  `CodeViewer` 每次渲染都重新高亮整个文件；管理端列表的搜索防抖定时器
  在卸载时未清理；滚动监听未声明 `passive`。

## 0.1.0 — 2026-10-23

CodeFever Next 的首个公开版本。基于
[PGYER/codefever](https://github.com/PGYER/codefever) 二次开发，
**后端已从 PHP 完整重写为 Go**。

### 破坏性变更

- **后端运行时下线 PHP**。镜像内不再存在 PHP 解释器、php-fpm、PHP 配置、PHP 入口脚本
  与 crond；`application/`、`system/`、`www/index.php` 等 PHP 源码已从仓库中删除。
  - `GET /api/admin/systemStatus` 的 `service` 段不再返回 `php` / `crond` 两个键
    （管理端仪表盘同步去掉了 PHP-FPM 与 Crond 两张卡片）。
  - `GET /api/repository/fileContent` 由 PHP 时代的**恒定 500** 修正为 **200**
    （PHP 侧方法名拼写错误导致的缺陷）。
- **会话存储位置变更**。PHP 会话在客户端 Cookie（CI 加密），Go 会话在服务端 `cc_sessions` 表，
  两者互不认得。**从旧版本切过来后需要重新登录一次**（不影响任何业务数据）。
- **数据库结构未变**，无需数据迁移。
- **数据库版本要求提高到 MySQL 8.0.x（或 MariaDB 10.6+）**。
  事件 worker 与 webhook 投递器用 `FOR UPDATE SKIP LOCKED` 领取任务行，
  该语法自 MySQL 8.0.1 起才支持；在 5.7 上这两条查询直接报 SQL 语法错误，
  会让**活动流、站内信、邮件通知与 webhook 静默失效**
  （症状是日志里每 2 秒刷一次 `SKIP LOCKED` 报错）。
  `docker-compose.yml` 的默认数据库镜像为 `mysql:8.0`，并追加
  `--default-authentication-plugin=mysql_native_password`
  ——MySQL 8 的默认插件 `caching_sha2_password` 不在应用容器内 MariaDB 客户端的插件目录里，
  不改这一项的话建库会直接失败。该参数在 MySQL 8.4+ 已移除，因此不支持 8.4/9.x。
- **站点品牌名变更**。站点名由 `CodeFever Community` 改为 `CodeFever Next`（仓库名 `codefever-next`）。
  Logo 与其余视觉资产保持不变。受影响的位置：
  - `config.yaml` 的 `email.name` 默认值（页面标题、邮件发件人名）——
    `/api/admin/config` 的 `data.email.name` 随之变为 `CodeFever Next`，
    生效契约基线 `backend/testdata/contract/admin.config.json` 已同步。
  - 服务端渲染页面的 `<meta name="description">`。
  - 通知邮件标题、保护分支拒绝提示、git 钩子提示语等用户可见文案。
  - 前端「关于 / 贡献代码」菜单项与 PWA `manifest.json` 的 `name`
    （`www/view/build` 产物已重建）。
  - 其余位置仍与 PHP 基线逐字节一致，详见 [`backend/testdata/README.md`](backend/testdata/README.md)。

### 新增

- Go 后端（`backend/`）：108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调；
  历史契约回归记录为 **123/123**，复跑方法见
  [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md)。
- 43 条服务端渲染路由 + 5 个页面；历史对照记录为 **43/43**（品牌文案属有意变更）。
- git 钩子 Go 化（`bin/codefever-hook`）：替代原 PHP 钩子脚本，
  stdout / 退出码 / 请求体与 PHP 版**逐字节一致**，并修复了 PHP 侧 stdin 非阻塞读可能截断的问题。
- SSH 公钥同步 Go 化：Go 后端直接写 `authorized_keys`，下线了原 PHP 守护进程与 FIFO 协议。
- 事件 / 站内信 / 邮件 / webhook 投递由独立的 Go worker 承担，替代原 PHP 定时任务调度器。
- 口令哈希支持 `argon2id`（通过 `CODEFEVER_PASSWORD_HASH` 切换，默认保持 `legacy` 以兼容存量账号）。

### 变更

- **应用镜像从本地源码构建**（`Dockerfile`）；构建期仍需要两个基础镜像、Debian 归档源与
  Go module 代理，不是“完全离线”。
- `docker-compose.local.yml` 合并进 `docker-compose.yml`，编排文件新增 `build` 段，
  `docker compose up -d` 即可完成构建与启动。
- 编排层配置统一走环境变量：`CODEFEVER_HTTP_PORT`、`CODEFEVER_SSH_PORT`、`CODEFEVER_DB_PASS`、
  `CODEFEVER_IMAGE`、`CODEFEVER_MYSQL_IMAGE`、`CODEFEVER_BASE_RUNTIME`、`CODEFEVER_BASE_GO`。
- 数据卷改用命名卷（`db-data` / `git-storage` / `file-storage`），
  保证 hooks 软链在容器重建后依然有效；后续版本新增 `config-data`。
  **当前版本已改为仓库根目录的 `codefever-data/` 绑定目录**，上述命名卷仅适用于
  0.1.0 及更早版本。
- 0.1.0 时仓库侧不再保留 `git-storage/hooks/{main,pre-receive,post-receive,update}`
  的脚本本体；当前版本进一步把运行时目录全部交给 Dockerfile/entrypoint 创建，
  新裸仓库的钩子链走 `deploy/hooks`，旧 `misc/hooks` 保留兼容软链。
- **修复启动时的数据库迁移竞态**。`codefever-api` 与 `codefever-worker` 都会在启动时
  执行 `db.Migrate()`，并发时双方会同时查到「未迁移」并同时写入迁移记录，
  后提交的一方撞上 `Duplicate entry` 而退出（日志里表现为 worker 启动失败、
  由 supervisor 重启一次才恢复）。现在整个迁移过程在 MySQL 命名锁
  （`GET_LOCK`）下串行化，后到的进程拿到锁后重查即可正确跳过。
- 构建配方修复了若干上游阻塞点：
  - Debian buster 已 EOL，apt 源改指归档源（默认 `archive.debian.org`，可用
    `CODEFEVER_APT_MIRROR` / `--build-arg APT_MIRROR` 覆盖）；
  - Go 网关改为多阶段构建 + module 模式（上游的 GOPATH 模式在 Go ≥ 1.22 已不可用）；
  - 构建期不再 `git clone`，直接 COPY 本地源码，不需要 GitHub；
  - Windows 检出时恢复 git 符号链接；
  - 构建期统一行尾规范化，避免 `sh: $'\r': command not found`。
- 仓库卫生：移除未使用的第三方前端静态资源（约 23 MB → 3.7 MB），
  修正根目录 `package.json` 元数据，补齐 `.gitattributes` / `.editorconfig`
  等开源项目标准文件。
- **构建配方更名为 `Dockerfile`**（原 `Dockerfile.local`）。`Dockerfile.local` 这个名字
  是"上游有 `Dockerfile`、本地另建一份"的产物；本仓库已没有上游配方，直接用
  `Dockerfile` 更符合惯例，`docker build .` 即可。
- Go module 路径由 `codefever-community/backend` 改为 `codefever-next/backend`。
- 仓库地址统一指向 <https://github.com/AidenX0704/codefever-next>。

### 修复

- `ssh-gateway` 打不开日志文件时 panic —— forced command 以 `git` 用户运行、工作目录是
  `/home/git`，一旦日志文件属主不对或家目录不可写，**所有 SSH push / clone 全部失败**，
  且客户端只能看到 "Permission denied"。现改为打不开就退回 stderr，绝不 panic。
- `.dockerignore` 未排除 `www/view/node_modules/`，本地安装的前端依赖（数百 MB）
  会被整个打进镜像。
- 文档读取限制在语言根目录内，阻断 `../` 穿越与符号链接逃逸。
- 创建 tag 时对参数做转义，避免命令注入。

### 性能

| 并发 | QPS | 错误率 | p95 | p99 |
| --- | --- | --- | --- | --- |
| 8 | 615 | 0 | 23.8 ms | 36.1 ms |
| 16 | 866 | 0 | 33.6 ms | 40.4 ms |
| 32 | 965 | 0 | 65.4 ms | 82.7 ms |
| 64 | 978 | 0 | 141.6 ms | 177.7 ms |

QPS 在 ~970 处饱和。当次压测环境使用 MySQL 5.7，瓶颈是单库而非 Go 后端；
64 并发下 p99 仍只有 178 ms，没有出现排队塌陷。1 核 CPU / 1 GB 内存即可运行。

### 已知问题

- 404 页面引用了上游从未提供的图片 `/static/assets/images/404_pic.png`，会显示一个破图。
- SSH 公钥类型识别沿用上游正则，部分非 RSA 公钥可能识别不稳定；建议使用 RSA。
- 英文用户文档仍在逐步翻译，部分 `doc/en-us/` 页面暂时保留中文。

详见 [SECURITY.md](SECURITY.md)。
