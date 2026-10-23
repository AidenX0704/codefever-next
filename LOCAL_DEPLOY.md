# CodeFever Next —— 本地镜像构建与部署

本目录提供一套**完全基于本地源码 + 国内镜像源**的构建 / 打包 / 运行方案，
不依赖任何上游预构建镜像，也不需要在构建期访问 GitHub。

| 文件 | 说明 |
| --- | --- |
| `Dockerfile` | 构建配方（多阶段：Go 网关 + Go 后端 + nginx/sshd 运行时） |
| `docker-compose.yml` | 编排：codefever + mysql 8.0（镜像从本地源码构建） |
| `scripts/build-local.ps1` | 一键：校验基础镜像 → 构建 → 导出 tar |
| `scripts/run-local.ps1` | 启动 / 停止 / 状态 / 日志 |
| `scripts/verify.ps1` | 一键复验：编译 + 单测 + 契约比对 + 截图比对 + 打包 |
| `dist/` | `docker save` 出来的镜像归档（离线分发用） |

---

## 0. 前置检查

```powershell
# Docker 引擎 + Compose v2（需要 20.10+ / v2）
docker version --format '{{.Server.Version}}'
docker compose version

# 磁盘余量：镜像 1.7GB + 数据库 + 仓库数据，建议留 10GB 以上
docker system df
```

端口占用检查（默认 `9080` / `2222`，被占用时见 §2.2）：

```powershell
Get-NetTCPConnection -LocalPort 9080,2222 -ErrorAction SilentlyContinue
```

---

## 1. 一条龙：构建 + 启动

在仓库根目录执行（PowerShell）：

```powershell
cd E:\WorkSpace\codefever

# 1) 确保基础镜像就位 + 构建 + 打包成 tar
pwsh -File scripts/build-local.ps1

# 2) 启动（--build 会用当前源码强制重建镜像）
docker compose -f docker-compose.yml up -d --build

# 3) 看状态
docker compose -f docker-compose.yml ps
```

`build-local.ps1` 会先确认三个基础镜像在本地（缺失时经 `docker.1ms.run` 拉取并打上本地
tag），随后用**本地已存在的镜像**构建，因此构建期不直连 Docker Hub：

| 基础镜像 | 用途 | 国内源 |
| --- | --- | --- |
| `webdevops/php-nginx:7.4` | 运行时基底（nginx / sshd / supervisor，PHP 部分在构建期被删干净） | `docker.1ms.run/webdevops/php-nginx:7.4` |
| `golang:1.25-bookworm` | 编译 Go 后端与两个网关 | `docker.1ms.run/library/golang:1.25-bookworm` |
| `mysql:8.0` | 数据库 | `docker.1ms.run/library/mysql:8.0` |

构建期所有外部依赖均走国内镜像：

- apt → `mirrors.aliyun.com/debian-archive`（Debian buster 已 EOL，官方源 404）
- Go modules → `mirrors.aliyun.com/goproxy`，并强制 `GODEBUG=http2client=0`
  （国内 goproxy 的 HTTP/2 会无限挂起且不超时）

> PHP 已整体下线，因此**不再需要** Composer 与 PECL（yaml 扩展）——
> `env.yaml` 的解析与 git 钩子现在都由 Go 后端负责。

> ⚠️ **数据库必须是 MySQL 8.0+**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED` 领取任务行，该语法自 MySQL 8.0.1 起才支持。
> 在 5.7 上这两条查询直接报语法错误，活动流、站内信、邮件通知与 webhook
> 会**静默失效**（日志里每 2 秒刷一次 `SKIP LOCKED` 报错）。
> compose 已默认 `mysql:8.0`，并加了
> `--default-authentication-plugin=mysql_native_password`
> ——MySQL 8 的默认插件 `caching_sha2_password` 不在容器内 MariaDB 客户端的插件目录里，
> 不改这一项的话 `misc/create_db.sh` 建库就会失败。

---

## 2. 手工分步（想看清每一步时用）

### 2.1 构建镜像

```powershell
cd E:\WorkSpace\codefever

# 直接用 Dockerfile 构建（基础镜像需已在本地）
docker build -f Dockerfile `
  --build-arg BASE_RUNTIME=webdevops/php-nginx:7.4 `
  --build-arg BASE_GO=golang:1.25-bookworm `
  -t codefever-next:local .
```

构建完成后镜像内**不应该**存在任何 PHP 产物，可自查：

```powershell
docker run --rm codefever-next:local sh -c 'which php php-fpm || echo "no php"; ls -A /usr/src || echo "/usr/src empty"'
```

### 2.2 启动

```powershell
cd E:\WorkSpace\codefever

# 默认端口 9080 / 2222
docker compose -f docker-compose.yml up -d --build

# 端口被占用时改用环境变量
$env:CODEFEVER_HTTP_PORT = "18080"
$env:CODEFEVER_SSH_PORT  = "18022"
docker compose -f docker-compose.yml up -d --build
```

compose 项目名固定为 `codefever-next`，容器名固定为：

| 名称 | 角色 |
| --- | --- |
| `codefever-next-app` | 应用本体（Go 后端 + nginx + sshd） |
| `codefever-next-db` | MySQL 8.0 |

### 2.3 运行期命令

```powershell
docker compose -f docker-compose.yml ps                              # 状态
docker compose -f docker-compose.yml logs -f codefever-next-app      # 跟踪日志
docker compose -f docker-compose.yml restart codefever-next-app      # 重启应用
docker compose -f docker-compose.yml down                            # 停止（保留数据卷）
docker compose -f docker-compose.yml down -v --remove-orphans        # 停止并删除数据卷（数据全丢）
```

也可以用封装好的脚本：

```powershell
pwsh -File scripts/run-local.ps1 up        # 启动
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

# 4) 三个常驻进程都在（api / worker / http-gateway）
docker exec codefever-next-app sh -c 'ps -ef | grep -E "codefever-(api|worker)|http-gateway/main" | grep -v grep'

# 5) 数据库健康
docker exec codefever-next-db mysqladmin ping -h 127.0.0.1 -uroot -p123456
```

访问入口：

| 项 | 值 |
| --- | --- |
| Web UI | <http://localhost:9080> |
| 默认账号 | `root@codefever.cn` |
| 默认密码 | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh -p 2222 git@localhost` |

> **首次登录后请立即修改密码并绑定 MFA 设备。**

> **仓库名不能含连字符**（HTTP 访问时）—— nginx vhost 的规则是
> `location ~* ^\/[0-9a-z_]+\/[0-9a-z_]+\.git`，不含 `-`。
> 建仓库时避开 `-` 即可（如 `demorepo`），SSH 方式不受此限制。

---

## 4. 数据在哪里

数据全部放在**命名卷**里（不是宿主机绑定挂载）：

| 命名卷 | 内容 |
| --- | --- |
| `codefever-next_db-data` | MySQL 数据文件 |
| `codefever-next_git-storage` | 裸仓库 + `hooks/` 符号链接 |
| `codefever-next_file-storage` | 上传文件与附件 |

> 用命名卷而不是绑定挂载，是因为命名卷首次创建时会从镜像内容初始化，
> `git-storage/hooks/*` 的符号链接才能得以保留 —— 绑定挂载会把宿主机目录整个盖上去，
> git push 的钩子会失效。

查看与备份：

```powershell
docker volume ls --filter name=codefever-next

# 备份数据库
docker exec codefever-next-db mysqldump -uroot -p123456 --all-databases > backup.sql

# 备份裸仓库
docker run --rm -v codefever-next_git-storage:/data -v "${PWD}:/backup" alpine `
  tar czf /backup/git-storage.tar.gz -C /data .
```

---

## 5. 离线分发

```powershell
cd E:\WorkSpace\codefever
pwsh -File scripts/build-local.ps1          # 产物在 dist/*.tar
```

在目标机器上：

```powershell
docker load -i dist/codefever-next-local-<时间戳>.tar
docker compose up -d
```

`docker-compose.yml` 里的 `image: codefever-next:local` 与 `docker save` 出来的标签一致，
`docker load` 后可直接启动。

---

## 6. 清理

```powershell
# 停服务并删掉本项目的卷（数据全丢）
docker compose -f docker-compose.yml down -v --remove-orphans

# 只删镜像
docker rmi codefever-next:local

# 看整体占用
docker system df
```

---

## 7. 常见问题

| 现象 | 原因与处理 |
| --- | --- |
| 构建卡在 Go modules 阶段不动 | goproxy 走了 HTTP/2 会无限挂起。确认 `Dockerfile` 里的 `GODEBUG=http2client=0` 生效，或改用 `--network host` 之外的网络策略重试 |
| `docker compose up` 卡在 `db` 不 healthy | MySQL 首次初始化要 30s 左右；`docker compose logs codefever-next-db` 看真实报错 |
| 页面能开但静态资源 404 | 命名卷 `git-storage` / `file-storage` 被删过或被旧卷占用。`docker compose down -v` 后重新 `up -d --build` |
| Git push 不触发钩子 | `git-storage/hooks/*` 符号链接丢失。删掉卷重建，不要用绑定挂载 |
| 端口被占用 | 设 `CODEFEVER_HTTP_PORT` / `CODEFEVER_SSH_PORT` 环境变量后重启 |
| Windows 上 `go test` 报 `Access is denied` | 本机策略拦截 `config.test.exe` 文件名，与代码无关。改用 Linux 容器跑测试，命令见 [贡献指南 · 测试与验证](CONTRIBUTING.md#单元测试) |

---

## 与上游构建配方的差异

> 上游的 `Dockerfile` 与 `codefever.build`（PHP 发布用的两个配方）**已随 PHP 运行时
> 一并删除** —— 它们的作用就是构建 PHP 镜像。现在唯一的构建配方是 `Dockerfile`。

上游配方在当前环境（2024 年之后的国内网络 + 新版 Go）下本来就**无法直接构建**，
`Dockerfile` 修掉了这些阻塞点，并进一步把 PHP 整个摘掉：

1. **Debian buster 已 EOL**（2024-06）——`deb.debian.org` 全部 404，
   `nginx.org` 源的 GPG key 也过期。改为 `mirrors.aliyun.com/debian-archive`。
2. **Go 网关无法用现代 Go 构建**——上游用 GOPATH 模式
   （`GO111MODULE=off` + 相对导入 `./Request`），Go ≥ 1.22 已移除该模式，
   且 GOPATH 导入路径不允许连字符（`http-gateway`）。
   改为多阶段构建 + module 模式，在构建期转换导入路径，**不改动仓库源码**。
3. **构建期不再 `git clone`**——直接 COPY 本地源码，可离线复现。
4. **行尾规范化**——Windows 检出时 `core.autocrlf=true` 会把脚本转成 CRLF，
   容器内 `sh` 报 `$'\r': command not found`；`misc/create_db.sh` 用
   `grep|awk` 解析 `env.yaml` 时还会把 `\r` 带进库名，导致建出
   `codefever_community\r` 这个畸形库。
5. **恢复 git 符号链接**——Windows 上 symlink 会退化成普通文本文件，
   `www/app -> view/build` 缺失会导致前端与 `/static` 资源全部 404，
   `*/hooks/* -> main` 缺失会导致 git push 钩子不生效。
   > 注意：规范化脚本**不能**用 `-name 'main'` 通配——Go 网关二进制文件名就叫
   > `main`，被 `sed` 处理后会损坏（运行期 SIGILL）。
6. **移除 PHP 运行时**——上游配方装 PHP 7.4 + yaml 扩展 + composer + php-fpm，
   并把 `application/`、`system/`、`www/index.php` 打进镜像。
   现在这些**全部不进镜像**，基础镜像自带的 PHP 解释器、php-fpm、PHP 入口脚本
   与 supervisor program 在构建期被显式删除 —— 镜像内不存在任何 PHP 可执行文件。
7. **服务以 `git` 用户运行**（`CONTAINER_UID=git`）——webdevops 镜像默认用
   `application`。应用进程需要能读写 `/home/git/.ssh/authorized_keys`。
   > 注：PHP 时代的 `modify_authorized_keys` 守护进程已下线，authorized_keys
   > 现在由 Go 后端（容器内以 root 运行）直接写。
8. **随机化 session key**——`env.template.yaml` 里是 `<session_key>` 占位符。

## 已知上游限制

- **仓库名不能含连字符**（经 HTTP 访问时）。nginx vhost 的规则是
  `location ~* ^\/[0-9a-z_]+\/[0-9a-z_]+\.git`，不含 `-`；而 Go 网关自身的
  正则 `^([a-zA-z0-9\/\.-]+\.git)(.*)$` 是允许的。建仓库时避开 `-` 即可
  （如 `demorepo`），SSH 方式不受此限制。
- **SSH 公钥只支持 RSA**。`Helper::getSSHKeyType()` 用
  `base64_decode(整行公钥)` 再正则匹配类型，对 ed25519 会误判成 `ssh-rsa`，
  写出畸形 `authorized_keys`。用 `ssh-keygen -t rsa` 生成密钥。
- **添加 SSH 公钥时 `key` 字段只传 base64 主体**（不含 `ssh-rsa` 前缀和注释），
  与前端 `UserSettingSSHKey.js` 的 `parsedKey[2]` 一致。
