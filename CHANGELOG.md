# 更新日志

本文件记录 CodeFever Next 的重要变更。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [Unreleased]

尚未发布的变更集中在此。

## [0.1.0] — 2026-10-23

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
- **数据库版本要求提高到 MySQL 8.0+（或 MariaDB 10.6+）**。
  事件 worker 与 webhook 投递器用 `FOR UPDATE SKIP LOCKED` 领取任务行，
  该语法自 MySQL 8.0.1 起才支持；在 5.7 上这两条查询直接报 SQL 语法错误，
  会让**活动流、站内信、邮件通知与 webhook 静默失效**
  （症状是日志里每 2 秒刷一次 `SKIP LOCKED` 报错）。
  `docker-compose.yml` 的默认数据库镜像已改为 `mysql:8.0`，
  并追加 `--default-authentication-plugin=mysql_native_password`
  ——MySQL 8 的默认插件 `caching_sha2_password` 不在应用容器内 MariaDB 客户端的插件目录里，
  不改这一项的话建库会直接失败。
  用 `CODEFEVER_MYSQL_IMAGE` 仍可覆盖镜像，但 **5.7 不再受支持**。
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

- Go 后端（`backend/`）：108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调，
  契约回归 **123/123** 符合预期。
- 43 条服务端渲染路由 + 5 个页面，与 PHP 版**逐字节一致（43/43）**。
- git 钩子 Go 化（`bin/codefever-hook`）：替代原 PHP 钩子脚本，
  stdout / 退出码 / 请求体与 PHP 版**逐字节一致**，并修复了 PHP 侧 stdin 非阻塞读可能截断的问题。
- SSH 公钥同步 Go 化：Go 后端直接写 `authorized_keys`，下线了原 PHP 守护进程与 FIFO 协议。
- 事件 / 站内信 / 邮件 / webhook 投递由独立的 Go worker 承担，替代原 PHP 定时任务调度器。
- 口令哈希支持 `argon2id`（通过 `CODEFEVER_PASSWORD_HASH` 切换，默认保持 `legacy` 以兼容存量账号）。

### 变更

- **镜像全部改为从本地源码构建**（`Dockerfile`），不再依赖任何上游预构建镜像。
- `docker-compose.local.yml` 合并进 `docker-compose.yml`，编排文件新增 `build` 段，
  `docker compose up -d` 即可完成构建与启动。
- 编排层配置统一走环境变量：`CODEFEVER_HTTP_PORT`、`CODEFEVER_SSH_PORT`、`CODEFEVER_DB_PASS`、
  `CODEFEVER_IMAGE`、`CODEFEVER_MYSQL_IMAGE`、`CODEFEVER_BASE_RUNTIME`、`CODEFEVER_BASE_GO`。
- 数据卷改用命名卷（`db-data` / `git-storage` / `file-storage`），
  保证 `git-storage/hooks/*` 的符号链接在容器重建后依然有效。
- 仓库侧不再保留 `git-storage/hooks/{main,pre-receive,post-receive,update}` 的脚本本体，
  只保留 `git-storage/hooks/.gitkeep` 作为目录骨架。这 4 个文件在 PHP 时代是钩子脚本的副本
  （PHP 下线后已无作用），且 Windows 检出时符号链接会退化成普通文本文件。
  镜像里的真实软链由 `Dockerfile` 构建期与容器 entrypoint 双重重建，
  不依赖仓库中的任何源文件；裸仓库的钩子链走 `misc/hooks/`，与本目录无关。
- **修复启动时的数据库迁移竞态**。`codefever-api` 与 `codefever-worker` 都会在启动时
  执行 `db.Migrate()`，并发时双方会同时查到「未迁移」并同时写入迁移记录，
  后提交的一方撞上 `Duplicate entry` 而退出（日志里表现为 worker 启动失败、
  由 supervisor 重启一次才恢复）。现在整个迁移过程在 MySQL 命名锁
  （`GET_LOCK`）下串行化，后到的进程拿到锁后重查即可正确跳过。
- 构建配方修复了若干上游阻塞点：
  - Debian buster 已 EOL，apt 源改指 `mirrors.aliyun.com/debian-archive`；
  - Go 网关改为多阶段构建 + module 模式（上游的 GOPATH 模式在 Go ≥ 1.22 已不可用）；
  - 构建期不再 `git clone`，直接 COPY 本地源码，可离线复现；
  - Windows 检出时恢复 git 符号链接；
  - 构建期统一行尾规范化，避免 `sh: $'\r': command not found`。
- 仓库卫生：移除未使用的第三方前端静态资源（约 23 MB → 3.6 MB），
  移除根目录 `package.json` 中与实际不符的元数据，补齐 `.gitattributes` / `.editorconfig`
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

QPS 在 ~970 处饱和，瓶颈是单库 MySQL 5.7 而非 Go 后端；64 并发下 p99 仍只有 178 ms，
没有出现排队塌陷。1 核 CPU / 1 GB 内存即可运行。

### 已知问题

- 404 页面引用了上游从未提供的图片 `/static/assets/images/404_pic.png`，会显示一个破图。
- 仓库名经 HTTP 访问时不允许包含连字符（SSH 方式不受限）。
- SSH 公钥仅支持 RSA。

详见 [SECURITY.md](SECURITY.md)。

[Unreleased]: https://github.com/AidenX0704/codefever-next/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AidenX0704/codefever-next/releases/tag/v0.1.0
