# CodeFever Next —— 本地镜像构建与部署

本文件说明如何从**当前源码**构建 / 打包 / 运行 CodeFever Next，以及离线分发、
镜像源切换和常见排查。构建期不需要访问 GitHub，但仍需要镜像仓库、Debian 归档源
与 Go module 代理。

| 文件 | 说明 |
| --- | --- |
| `Dockerfile` | 构建配方：单 Go module（`backend/`）编译 5 个二进制 + nginx/sshd 运行时 |
| `docker-compose.yml` | 编排：`codefever` + MySQL 8.0，应用镜像从当前源码构建 |
| `backend/deploy/` | 数据库脚本、nginx/supervisor 模板、容器入口、服务脚本 |
| `scripts/build-local.ps1` | 校验/拉取基础镜像 → 构建 → 导出 tar |
| `scripts/run-local.ps1` | 启动 / 停止 / 状态 / 日志 |
| `scripts/verify.ps1` | 维护者质量门禁：gofmt / build / vet / 单测 / compose 校验；加 `-BuildImage` 才构建镜像 |
| `dist/` | `docker save` 出来的镜像归档（离线分发用，已被 git 忽略） |

---

## 0. 前置检查

```powershell
# Docker 引擎 + Compose v2
docker version --format '{{.Server.Version}}'
docker compose version

# 磁盘余量：镜像、数据库与仓库数据，建议留 10GB 以上
docker system df
```

端口占用检查（默认 `9080` / `2222`，被占用时见 §2.2）：

```powershell
Get-NetTCPConnection -LocalPort 9080,2222 -ErrorAction SilentlyContinue
```

首次构建需要能访问：`docker.io`（或镜像加速）、Debian buster 归档源、Go module 代理。
国内网络见 §1.1。

---

## 1. 一条龙：构建 + 启动

在**仓库根目录**执行（PowerShell）：

```powershell
# 1) 确保基础镜像就位 + 构建 + 打包成 tar
pwsh -File scripts/build-local.ps1

# 2) 启动（--build 会用当前源码强制重建镜像）
docker compose -f docker-compose.yml up -d --build

# 3) 看状态
docker compose -f docker-compose.yml ps
```

`build-local.ps1` 会先确认三个基础镜像在本地；缺失时经 `-Registry`（默认
`docker.1ms.run`）拉取并打上本地 tag，然后用本地 tag 构建，构建期不直连 Docker Hub：

| 基础镜像 | 用途 | 默认镜像源（脚本） |
| --- | --- | --- |
| `webdevops/php-nginx:7.4` | 运行时基底（nginx / sshd / supervisor；PHP 在构建期删除） | `docker.1ms.run/webdevops/php-nginx:7.4` |
| `golang:1.25-bookworm` | 编译 Go 后端 + 5 个命令 | `docker.1ms.run/library/golang:1.25-bookworm` |
| `mysql:8.0` | 数据库 | `docker.1ms.run/library/mysql:8.0` |

### 1.1 镜像源

`docker-compose.yml` 默认使用 Docker Hub / `proxy.golang.org` / `archive.debian.org`；
可直接用环境变量切换（不依赖 build-local 脚本）：

```powershell
$env:CODEFEVER_BASE_RUNTIME = 'docker.1ms.run/webdevops/php-nginx:7.4'
$env:CODEFEVER_BASE_GO      = 'docker.1ms.run/library/golang:1.25-bookworm'
$env:CODEFEVER_MYSQL_IMAGE  = 'docker.1ms.run/library/mysql:8.0'
$env:CODEFEVER_GOPROXY      = 'https://mirrors.aliyun.com/goproxy,direct'
$env:CODEFEVER_APT_MIRROR   = 'https://mirrors.aliyun.com/debian-archive'
docker compose up -d --build
```

> 不要混用两套方式：`build-local.ps1` 走 `-Registry`；直接 `docker compose` 走
> `CODEFEVER_BASE_RUNTIME` / `CODEFEVER_BASE_GO` / `CODEFEVER_MYSQL_IMAGE`。

> ⚠️ **数据库使用 MySQL 8.0.x**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED`（MySQL 8.0.1+）。compose 里的
> `--default-authentication-plugin` 在 MySQL 8.4+ 已移除，所以不要用 `mysql:8.4`/`9.x`。

> PHP 已整体下线。`env.yaml` 解析、git 钩子、HTTP/SSH 网关全部由 Go 实现，
> 不需要 Composer、PECL 或 php-fpm。

---

## 2. 手工分步（想看清每一步时用）

### 2.1 构建镜像

```powershell
# 在仓库根目录
docker build -f Dockerfile `
  --build-arg BASE_RUNTIME=webdevops/php-nginx:7.4 `
  --build-arg BASE_GO=golang:1.25-bookworm `
  -t codefever-next:local .
```

构建完成后镜像内**不应该**存在任何 PHP 产物，可自查：

```powershell
docker run --rm --entrypoint sh codefever-next:local -c 'which php php-fpm || echo "no php"; ls -A /usr/src || true'
```

镜像内 `doc/` 会随构建复制进去，应用内 `/doc/zh-cn/`、`/doc/en-us/` 可以正常访问；
`LICENSE` 也在镜像根目录。

### 2.2 启动

```powershell
# 默认端口 9080 / 2222
docker compose -f docker-compose.yml up -d --build

# 端口被占用时改用环境变量
$env:CODEFEVER_HTTP_PORT = "18080"
$env:CODEFEVER_SSH_PORT  = "18022"
docker compose -f docker-compose.yml up -d --build
```

compose 项目名固定为 `codefever-next`。**服务名**与**容器名**不同：

| Compose 服务 | 容器名 | 角色 |
| --- | --- | --- |
| `codefever` | `codefever-next-app` | 应用本体（Go 后端 + nginx + sshd） |
| `db` | `codefever-next-db` | MySQL 8.0 |

换域名/反代/非默认端口时，显式设置 clone 面板地址：

```powershell
$env:CODEFEVER_SITE_HOST = "https://git.example.com"
$env:CODEFEVER_SITE_SSH  = "ssh://git@git.example.com:2222/"
docker compose up -d
```

### 2.3 运行期命令

```powershell
docker compose -f docker-compose.yml ps                              # 状态
docker compose -f docker-compose.yml logs -f codefever               # 跟踪应用日志
docker compose -f docker-compose.yml restart codefever               # 重启应用
docker compose -f docker-compose.yml down                            # 停止（保留 codefever-data）
docker compose -f docker-compose.yml down -v --remove-orphans        # 删除容器/网络（codefever-data 保留）
docker exec -it codefever-next-app sh                                # 进入容器排查
```

也可以用封装好的脚本：

```powershell
pwsh -File scripts/run-local.ps1 up        # 启动并轮询 /user/login
pwsh -File scripts/run-local.ps1 status    # 状态 + HTTP 健康检查
pwsh -File scripts/run-local.ps1 logs      # 跟踪日志
pwsh -File scripts/run-local.ps1 down      # 停止（保留数据卷）
```

---

## 3. 验证清单

启动后按顺序过一遍，全绿即表示本地部署成功：

```powershell
# 1) 首页可访问
Invoke-WebRequest http://localhost:9080 -UseBasicParsing | Select-Object StatusCode

# 2) 登录（默认账号 root@codefever.cn / 123456）
#    浏览器打开 http://localhost:9080 手工登录即可

# 3) 镜像里没有 PHP
docker exec codefever-next-app sh -c 'which php php-fpm || echo "no php"'

# 4) 常驻进程（api / worker / http-gateway / nginx / sshd）
docker exec codefever-next-app sh -c 'ps -ef | grep -E "codefever-(api|worker|http-gateway)|nginx|sshd" | grep -v grep'

# 5) 数据库健康（密码用你自己的 CODEFEVER_DB_PASS）
docker exec codefever-next-db mysqladmin ping -h 127.0.0.1 -uroot -p'<口令>'
```

访问入口：

| 项 | 值 |
| --- | --- |
| Web UI | <http://localhost:9080> |
| 默认账号 | `root@codefever.cn` |
| 默认密码 | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh://git@localhost:2222/<group>/<repo>.git` |

> **首次登录后请立即修改密码、绑定 MFA 设备，并在管理后台关闭开放注册。**

> 仓库名**可以**包含连字符：nginx vhost 与 Go 网关/后端统一使用
> `[0-9a-z_.-]+`。SSH 方式没有额外字符限制。

### 3.1 邮件（SMTP）

镜像不安装 MTA，不配置 SMTP 时系统 `mail` 通常不可用。compose 支持：

```powershell
$env:CODEFEVER_SMTP_HOST = 'smtp.example.com'
$env:CODEFEVER_SMTP_PORT = '587'
$env:CODEFEVER_SMTP_USER = 'git@example.com'
$env:CODEFEVER_SMTP_PASSWORD = '<口令>'
$env:CODEFEVER_SMTP_FROM = 'CodeFever <git@example.com>'
$env:CODEFEVER_SMTP_TLS = 'starttls'
docker compose up -d
```

未配置 SMTP 时，通知邮件只会在 worker 日志里记录失败，不影响 Git 操作。

---

## 4. 数据在哪里

默认全部持久化在仓库根目录的 **`codefever-data/`**（绑定挂载，不是 Docker 命名卷）：

| 宿主机目录 | 内容 |
| --- | --- |
| `codefever-data/mysql/` | MySQL 数据文件 |
| `codefever-data/git-storage/` | 裸仓库 + `deploy/hooks` / `misc/hooks` 软链 |
| `codefever-data/file-storage/` | 上传文件与附件 |
| `codefever-data/config/` | `config.yaml` / `env.yaml`（管理后台修改的主机名、SMTP、token 等） |

`codefever-data/` 已被 `.gitignore` 忽略；首次启动时 Docker 会自动创建这些子目录，
容器入口脚本会创建 `git-storage/hooks`、`deploy/hooks` 等兼容软链，不需要手工准备。
需要换位置时设置 `CODEFEVER_DATA_DIR=/绝对路径`。

查看与备份（先停服务，保证 MySQL 数据一致）：

```powershell
docker compose stop

# 整个数据目录（MySQL / 仓库 / 附件 / 配置）
tar czf codefever-data-$(date +%Y%m%d).tar.gz codefever-data

# Windows PowerShell 可用：
# Compress-Archive -Path codefever-data -DestinationPath codefever-data.zip

docker compose start
```

> **从旧命名卷迁移**：旧版本的 `codefever-next_db-data` 等命名卷不会被自动挂载。
> Linux 上可直接把卷内容复制到 `codefever-data/`；Windows 上因为绑定挂载是大小写
> 不敏感文件系统，MySQL 的 `lower_case_table_names` 会从 0 变成 2，必须
> `mysqldump` 旧库后在新库导入，不能直接复制 `mysql/` 数据目录。

---

## 5. 离线分发

```powershell
# 在联网/可访问镜像源的机器上，仓库根目录
pwsh -File scripts/build-local.ps1          # 产物在 dist/*.tar
```

`build-local.ps1` 的实际文件名是 `dist/codefever-next_local-<yyyyMMdd-HHmmss>.tar`
（镜像名里的 `:` 被替换为 `_`）。把该 tar 与仓库源码一起带到目标机器：

```powershell
docker load -i dist/codefever-next_local-<时间戳>.tar
docker compose up -d
```

`docker-compose.yml` 里的 `image: codefever-next:local` 与 `docker save` 的标签一致，
`docker load` 后可直接启动；**不需要**再改 compose 里的 image 名。

> 完全离线环境还需要目标机器上已有 `mysql:8.0` 镜像；可在联网机器
> `docker save mysql:8.0` 后一并 `docker load`。

---

## 6. 清理

```powershell
# 停服务并删除容器/网络（codefever-data 数据目录保留）
docker compose -f docker-compose.yml down -v --remove-orphans

# 确实要清空全部数据时，再手动删除：
# Remove-Item -Recurse -Force codefever-data

# 只删镜像
docker rmi codefever-next:local

# 看整体占用
docker system df
```

---

## 7. 常见问题

| 现象 | 原因与处理 |
| --- | --- |
| 首次构建卡在拉取镜像 / Go modules | 当前网络访问不了 Docker Hub、Debian 归档或 Go proxy。按 §1.1 设置镜像源；国内 goproxy 可用 `mirrors.aliyun.com/goproxy` |
| `docker compose up` 卡在 `db` 不 healthy | MySQL 首次初始化需要几十秒；`docker compose logs db` 看真实报错 |
| 页面能开但静态资源 404 | 检查 `www/app -> view/build` 软链与 `www/view/build` 是否在镜像内；与 `codefever-data` 数据目录无关 |
| Git push 不触发钩子 | `codefever-data/git-storage/hooks` 或 `deploy/hooks` 软链丢失；重启 `codefever` 容器会自动重建，不要手工删数据目录 |
| clone 面板地址不对 | 检查 `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH`；或在管理后台“设置 → 主机”修改。修改持久化在 `codefever-data/config/config.yaml` |
| 邮件发不出 | 配置 `CODEFEVER_SMTP_*`；镜像没有 MTA，未配置 SMTP 时 `mail` 回退不可用 |
| 端口被占用 | 设 `CODEFEVER_HTTP_PORT` / `CODEFEVER_SSH_PORT` 环境变量后重启 |
| Windows 上 `go test` 报 `Access is denied` | 本机策略拦截 `config.test.exe` 文件名，与代码无关。改用 Linux 容器跑测试，命令见 [贡献指南 · 测试与验证](CONTRIBUTING.md#单元测试) |

---

## 与上游构建配方的差异

> 上游的 `Dockerfile` 与 `codefever.build`（PHP 发布用的两个配方）**已随 PHP 运行时
> 一并删除**。当前唯一的构建配方是 `Dockerfile`。

当前配方还做了这些结构收敛：

1. **单 Go module**：`http-gateway` / `ssh-gateway` 已并入 `backend/`（`backend/cmd/
   codefever-http-gateway`、`codefever-ssh-gateway`），不再有 GOPATH 相对导入与
   构建期 `sed` 改写。
2. **构建期不 `git clone`**：直接 COPY 本地源码；不需要 GitHub，但仍需镜像仓库、
   Debian 归档和 Go module 代理。
3. **行尾规范化**：Windows 检出的 CRLF 会在构建期转成 LF，避免
   `sh: $'\r': command not found`。
4. **软链骨架**：构建期创建 `www/app -> view/build`、`deploy/hooks`、`misc/hooks`
   （旧裸仓库兼容）与 `git-storage/hooks`；入口脚本每次启动会重建。
5. **移除 PHP 运行时**：不 COPY PHP 源码，并删除基础镜像自带的 PHP / php-fpm /
   PHP 入口与 supervisor program，构建期自检。
6. **配置持久化**：`config.yaml` / `env.yaml` 是 `config/` 的软链，`config/` 位于
   `codefever-data/config/` 绑定目录；管理后台修改不会因容器重建丢失。
7. **随机会话与网关 token**：构建期生成 `session.key`、`totp.salt`、`gateway.token`；
   源码安装需手工填写。
8. **文档与许可证进镜像**：`COPY doc/` 与 `COPY LICENSE`，应用内文档中心可用。

## 已知限制

- **SSH 公钥类型识别沿用上游正则**。部分非 RSA 公钥（如某些 ed25519 变体）可能
  识别不稳定，优先用 `ssh-keygen -t rsa`。
- **添加 SSH 公钥时 `key` 字段只传 base64 主体**（不含前缀和注释），与前端
  `UserSettingSSHKey.js` 的解析一致。
- **基础镜像仍是 Debian buster（EOL）**，apt 走归档源，git 版本偏旧；这是后续
  迁移项，不影响当前功能。
