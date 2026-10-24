<div align="center">
  <br />
  <img src="www/static/images/logo-community.png" alt="CodeFever" height="42" />
  <br /><br />
  <p>
    <b>
      CodeFever Next —— 完全开源的 Git 代码托管平台 · 后端已用 Go 重写
    </b>
  </p>
  <p>

[![Contributions welcome](https://img.shields.io/badge/Contributions-welcome-brightgreen?logo=github)](CONTRIBUTING.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)](backend/)
[![PHP](https://img.shields.io/badge/PHP-removed-8892B0?logo=php&logoColor=white)](docs/PHP_REMOVAL.md)
[![Docker](https://img.shields.io/badge/Docker-supported-2496ED?logo=docker&logoColor=white)](doc/zh-cn/installation/install_via_docker_compose.md)

  </p>
  <br />
</div>

[English](README_EN.md) · [部署与构建说明](LOCAL_DEPLOY.md) · [文档中心](doc/zh-cn/index.md) ·
[贡献指南](CONTRIBUTING.md) · [安全策略](SECURITY.md) · [更新日志](CHANGELOG.md)

---

## ⚠️ 项目声明

> **CodeFever Next 是基于 [PGYER/codefever](https://github.com/PGYER/codefever) 进行二次开发的 Git 代码托管平台。**
>
> **本项目为非官方衍生版本，由 [GiserXiao](https://github.com/AidenX0704) 独立维护，与蒲公英官方团队无隶属关系。**
>
> **原始 CodeFever 项目版权归蒲公英开发者服务平台所有，基于 MIT 协议开源。本仓库保留原始 LICENSE 及版权声明。**

上游项目：<https://github.com/PGYER/codefever> ｜ 原始官网：<https://codefever.cn>

---

## 这是什么

`CodeFever Next` 是一套轻量、自托管的 Git 代码托管服务：仓库、分支、标签、
合并请求、分支保护、Webhook、多人协作与权限管理，中英双语界面。

**相对上游最大的变化：后端已从 PHP 完整重写为 Go**，运行镜像里不再存在任何
PHP 进程、PHP 可执行文件与 PHP 配置；`http-gateway` 与 `ssh-gateway` 也已并入
同一个 Go module（`backend/`）。

| 维度 | 状态 |
| --- | --- |
| 后端 | Go（`backend/`），替换原 CodeIgniter 2.1.1 + PHP 7.4 实现 |
| API 契约 | 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调；**123/123 为历史验收记录**，复跑方法见 [实施记录](docs/IMPLEMENTATION_STATUS.md) |
| Web 路由 | 43 条服务端渲染路由 + 5 个页面；**43/43 为历史对照记录**（品牌文案属有意变更） |
| 前端 | React 17 + MUI v4；产物 `www/view/build` 随仓库提交，构建期不需要 Node.js |
| 运行时 | 镜像内 **无 PHP、无 php-fpm、无 crond** |
| 镜像 | 应用镜像从本地源码构建；仍需拉取两个基础镜像（见“快速开始”） |
| 前端体积 | `www/static/vendor` 约 3.7 MB / 27 个文件（历史对照约 23.2 MB / 2072 个文件） |

> 迁移的完整依赖面清单、稳定性门禁与历史验证数据见 [`docs/PHP_REMOVAL.md`](docs/PHP_REMOVAL.md)。
> 镜像内包含 `doc/`（应用内文档中心）与 `LICENSE`；`docs/` 是重写记录，不进镜像。
> 契约数字是当次验收结果，不是当前环境的自动保证；复跑需要可用的实例与数据库。

## 功能特性

- 🆓 **永久免费:** 无任何付费墙与功能阉割
- ❤️ **完整开源:** MIT 协议，无编译或加密代码
- 🥬 **轻量安装:** `docker compose up -d --build` 即可完成部署
- ⚡️ **性能高效:** 1 核 CPU / 1G 内存即可运行；历史压测 64 并发 p99 ≈ 178 ms
- ♾️ **无限仓库:** 无仓库数量与容量限制
- ⌨️ **代码对比:** 支持多版本高亮差异对比
- 🛤️ **分支管理:** 完整支持 Git 原生分支与 Tag
- 🙋‍ **合并请求（Merge Request）:** 支持多人协作评审
- 🙅 **分支保护:** 保护分支让提交安全可控
- 👥 **多人协作:** 角色与权限管理
- 🔌 **Webhook:** 可与 Jenkins / Jira 等外部系统集成
- 🖥️ **管理后台:** 超级管理员可管理所有项目与用户
- 💬 **中英双语界面:** 界面支持中文与英文（用户文档的英文翻译仍在补全）
- 🔑 **SSH 推送:** 支持 SSH 公钥认证的 clone / pull / push

## 快速开始

环境要求：

- Docker 20.10+ 与 Docker Compose v2；
- **首次构建需要网络**：拉取 `webdevops/php-nginx:7.4`、`golang:1.25-bookworm`、`mysql:8.0`
  三个镜像，并访问 Debian buster 归档源与 Go module 代理；
- 建议至少 10 GB 可用磁盘空间（镜像、数据库与仓库数据）。

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

> **首次构建卡在 `load metadata ... docker.io` / `auth.docker.io` 超时？**
> 说明基础镜像尚未拉到本机，Docker 正在访问 Docker Hub。国内/企业网络先执行
> `pwsh -File scripts/build-local.ps1 -SkipPackage`（通过镜像源拉取并打本地 tag），
> 或按下方“国内网络 / 镜像源”设置 `CODEFEVER_BASE_*` 后重试。镜像已在本机时，
> Compose 会直接复用，不再访问 Docker Hub。

启动完成后：

| 项 | 值 |
| --- | --- |
| Web UI | <http://localhost:9080> |
| 默认账号 | `root@codefever.cn` |
| 默认密码 | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh://git@localhost:2222/<group>/<repo>.git` |

> **首次登录后请立即修改密码、绑定 MFA 设备，并在管理后台关闭开放注册。**
> 默认 `allowRegister: true`，公网实例务必关闭。

常用命令（注意服务名是 `codefever`，容器名才是 `codefever-next-app`）：

```shell
docker compose ps                          # 查看状态
docker compose logs -f codefever           # 跟踪应用日志
docker compose restart codefever           # 重启应用
docker compose down                        # 停止（保留 codefever-data）
docker compose down -v                     # 停止并删除容器/网络（不删 codefever-data）
docker exec -it codefever-next-app sh     # 进入应用容器排查
```

### 端口与站点地址

默认将容器内 `80/22` 映射到宿主机 `9080/2222`：

```shell
CODEFEVER_HTTP_PORT=18080 CODEFEVER_SSH_PORT=18022 docker compose up -d
```

容器入口会按这两个端口自动写入 `config.yaml` 的 `host`/`ssh`，前端 clone 面板
会直接显示正确地址。若部署在反向代理、域名或非默认端口之后，显式覆盖：

```shell
CODEFEVER_SITE_HOST=https://git.example.com \
CODEFEVER_SITE_SSH=ssh://git@git.example.com:2222/ \
docker compose up -d
```

> `ssh` 使用 `ssh://git@host:port/` 形式才能携带端口；scp 风格的
> `git@host:path` 不能表达非 22 端口。管理后台“设置 → 主机”也可以修改这两项，
> 修改会持久化到仓库根目录的 `codefever-data/config/config.yaml`。

### 国内网络 / 镜像源

Compose 默认使用 Docker Hub、`proxy.golang.org` 与 Debian 官方归档源。国内网络可覆盖：

```shell
CODEFEVER_BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4 \
CODEFEVER_BASE_GO=docker.1ms.run/library/golang:1.25-bookworm \
CODEFEVER_MYSQL_IMAGE=docker.1ms.run/library/mysql:8.0 \
CODEFEVER_GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
CODEFEVER_APT_MIRROR=https://mirrors.aliyun.com/debian-archive \
docker compose up -d --build
```

> 首次构建必须能访问上述镜像仓库/代理；完全离线环境请先在联网机器上
> `docker save`，再在目标机器 `docker load`（见 [LOCAL_DEPLOY.md](LOCAL_DEPLOY.md)）。

## 配置说明

### 编排层环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | 宿主 HTTP 端口 |
| `CODEFEVER_SSH_PORT` | `2222` | 宿主 Git SSH 端口 |
| `CODEFEVER_SITE_HOST` | 按 HTTP 端口生成 | clone 面板显示的主机 URL，如 `https://git.example.com` |
| `CODEFEVER_SITE_SSH` | 按 SSH 端口生成 | clone 面板显示的 SSH URL，如 `ssh://git@host:2222/` |
| `CODEFEVER_DB_PASS` | `123456` | 数据库 root 口令；支持含 `/`、`&`、`\` 的强口令 |
| `CODEFEVER_DATA_DIR` | `./codefever-data` | 数据目录（MySQL / 裸仓库 / 附件 / 配置） |
| `CODEFEVER_IMAGE` | `codefever-next:local` | 应用镜像名 |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | 数据库镜像（**8.0.x**；8.4+ 已移除 compose 里的旧认证参数） |
| `CODEFEVER_BASE_RUNTIME` | `webdevops/php-nginx:7.4` | 构建期运行时基础镜像 |
| `CODEFEVER_BASE_GO` | `golang:1.25-bookworm` | 构建期 Go 基础镜像 |
| `CODEFEVER_GOPROXY` | `https://proxy.golang.org,direct` | 构建期 Go module 代理 |
| `CODEFEVER_APT_MIRROR` | `https://archive.debian.org` | Debian buster 归档源 |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | 新口令哈希格式：`legacy`（`md5(md5(x))`）或 `argon2id` |

> 存量账号的口令哈希是历史遗留的 `legacy` 格式，默认保持 `legacy`，
> **存量账号无需重置口令即可登录**。等全部账号重置过口令后，再切成 `argon2id`。

> ⚠️ **数据库使用 MySQL 8.0.x**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED` 领取任务行（MySQL 8.0.1+）；compose 里的
> `--default-authentication-plugin` 在 MySQL 8.4+ 已移除，因此不要使用 8.4/9.x。

### 邮件（SMTP）

镜像默认**不安装 MTA**，不配置 SMTP 时系统 `mail` 命令不可用。请设置：

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<口令>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls     # 465 端口自动按 implicit TLS 处理
```

SMTP 未配置时，邮件通知会回退到系统 `mail`（镜像内通常不存在），只会在 worker
日志中记录失败，不会影响 Git 操作。

### 应用配置与持久化

默认所有持久化数据都在仓库根目录的 **`codefever-data/`**（已被 `.gitignore` 忽略）：

```text
codefever-data/
├── mysql/          # MySQL 数据文件
├── git-storage/    # 裸仓库 + hooks 软链
├── file-storage/   # 上传文件与附件
└── config/         # config.yaml + env.yaml（host/ssh、SMTP、token 等）
```

`env.yaml` 与 `config.yaml` 在容器内是 `<base>/config/` 目录的软链，因此管理后台的
修改直接落在 `codefever-data/config/`，容器重建/镜像升级后仍保留。
需要换位置时设置 `CODEFEVER_DATA_DIR=/绝对路径`。

备份（建议先停服务，确保 MySQL 数据一致）：

```shell
docker compose stop
tar czf codefever-data-$(date +%Y%m%d).tar.gz codefever-data
docker compose start
```

> Windows PowerShell：`Compress-Archive -Path codefever-data -DestinationPath codefever-data.zip`。
> **从旧命名卷升级到目录持久化时**，如果 MySQL 数据来自 Linux 命名卷、而当前是
> Windows 绑定挂载，需要先 `mysqldump` 再从新库导入（Windows 文件系统大小写不敏感，
> MySQL 的 `lower_case_table_names` 会从 0 变为 2，不能直接复用旧数据目录）。

- 镜像构建时会随机生成 `session.key`、`totp.salt`、`gateway.token`；
- 源码安装时需要手工填写 `gateway.token`（随机字符串），否则内部网关回调无法鉴权；
- `gateway.token` 不要提交到仓库，也不要在多个实例间复用。

## 不使用 Docker（从源码安装）

需要 `nginx`、`git`、`Go 1.23+`、`MySQL 8.0.x`（`backend/go.mod` 声明 `go 1.23.0`；
Docker 构建使用 Go 1.25 基础镜像，与源码安装不冲突）。完整步骤见
[从零开始安装](doc/zh-cn/installation/install_from_scratch.md)，核心流程如下：

```shell
# 0. 目录约定：源码放在 /data/www/codefever-next
#    容器内路径是 /data/www/codefever-community，两者不要混用。
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

# 1. 配置：源码安装直接使用仓库根目录的 config.yaml / env.yaml
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0660 config.yaml env.yaml

# 2. 填写随机的 session key / totp salt / gateway token
KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" env.yaml

# 2.1 修改 config.yaml 的 host / ssh（源码安装默认 80/22）
#     host: http://<服务器IP>
#     ssh:  git@<服务器IP>:        # 非 22 端口时用 ssh://git@<服务器IP>:<端口>/

# 3. 编译 5 个二进制
mkdir -p bin file-storage git-storage
cd backend
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api          ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker       ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook         ./cmd/codefever-hook
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-http-gateway ./cmd/codefever-http-gateway
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-ssh-gateway  ./cmd/codefever-ssh-gateway
cd ..

# 4. 初始化数据库（读取仓库根目录的 env.yaml）
cd backend/deploy/sql
sh ./create_db.sh
```

源码安装还需要：

- 启动服务前创建 `git` 系统用户与家目录：
  ```shell
  useradd -rm -d /home/git -s /bin/bash git
  mkdir -p /home/git/.ssh && chown -R git:git /home/git/.ssh
  chown root:git config.yaml env.yaml
  chmod 0660 config.yaml env.yaml
  ```
- 创建运行时钩子软链（Dockerfile 会自动做；源码安装需要手工执行一次）：
  ```shell
  mkdir -p deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell
  for d in deploy/hooks misc/hooks git-storage/hooks; do
    ln -sfn ../../bin/codefever-hook "$d/main"
    ln -sfn main "$d/pre-receive"; ln -sfn main "$d/post-receive"; ln -sfn main "$d/update"
  done
  ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main
  ln -sfn view/build www/app
  ```
- nginx 使用 `backend/deploy/docker/vhost-go.conf-template`，并把其中的
  `/data/www/codefever-community` 改成源码目录 `/data/www/codefever-next`；
- SSH 的 `ForceCommand` 使用 `<源码目录>/bin/codefever-ssh-gateway`；
- systemd 单元使用 `bin/codefever-api`、`bin/codefever-worker`、
  `bin/codefever-http-gateway`，工作目录为源码根目录。

## 开发与测试

```shell
cd backend

go build ./...          # 编译全部包（含 5 个命令）
go vet ./...            # 静态检查
gofmt -l .              # 格式检查
go test ./internal/...  # 单元测试
```

> **CI 已恢复**：`.github/workflows/ci.yml` 在 Go 1.23 上执行 gofmt / build /
> Linux 交叉编译 / vet / test，以及 `docker compose config`。
>
> ⚠️ **Windows 上建议在 Linux 容器里跑测试** —— 部分环境会拦截 `config.test.exe`
> 这类文件名，报 `Access is denied`（本机策略问题，与代码无关）。
> 容器化的完整命令见 [贡献指南 · 测试与验证](CONTRIBUTING.md#单元测试)。

维护者本地质量门禁（gofmt → build → Linux 交叉编译 → vet → 单测 → compose config；
加 `-BuildImage` 才构建镜像）：

```powershell
pwsh -File scripts/verify.ps1
```

> 该门禁不启动数据库/应用，也不做契约、截图或写操作冒烟；这些深度验证有额外前置，
> 见 [CONTRIBUTING.md](CONTRIBUTING.md#测试与验证)。普通用户部署只需要 Compose。

前端产物重建（**不要**在正常部署时重装依赖；`@pgyer/mui-theme-preset`
声明为 `latest`，重装会漂移）：

```shell
cd www/view && NODE_OPTIONS=--openssl-legacy-provider yarn build
```

契约基线在 `backend/testdata/contract/`，历史采集与比对命令见
[贡献指南](CONTRIBUTING.md#契约回归)。这些是历史验收/回归工具，不是运行前提。

> 行尾统一 LF，由 `.gitattributes` 保证；编辑器行为由 `.editorconfig` 约定。

## 从 PHP 版本迁移

如果你在跑旧的上游 PHP 版本：

- **数据不需要迁移** —— 数据库结构没有变化；会话需要重新登录一次
  （旧会话在客户端 Cookie 里，新会话在服务端 `cc_sessions` 表，两者互不认得）；
- **口令哈希默认仍是 `legacy`** —— 存量账号免重置口令即可登录；
- `admin.systemStatus` 接口少了 `php` / `crond` 两个键（PHP 下线后没有意义）；
- `repository.fileContent` 从 PHP 时代的恒定 500 修正为 200；
- 旧版的 `misc/hooks`、`ssh-gateway/shell/main` 路径在镜像里保留兼容软链；
  新建仓库的 hooks 指向 `deploy/hooks`。

## 已知限制

| 限制 | 影响 | 规避方式 |
| --- | --- | --- |
| SSH 公钥类型识别沿用上游正则 | 部分非 RSA 公钥（如某些 ed25519 变体）可能识别不稳定 | 优先使用 `ssh-keygen -t rsa` 生成密钥 |
| 添加 SSH 公钥时 `key` 字段只传 base64 主体 | 不含 `ssh-rsa` 前缀与注释 | 与前端 `UserSettingSSHKey.js` 的解析保持一致 |
| 404 页面引用了不存在的图片 | `/static/assets/images/404_pic.png` 会显示破图 | 可自行补充图片；不影响功能 |
| 英文用户文档尚未全部翻译 | `doc/en-us/` 中部分页面仍为中文 | 见 [README_EN.md](README_EN.md) 的说明；欢迎参与翻译 |

> 仓库名**可以**包含连字符：nginx vhost 与 Go 后端使用 `[0-9a-z_.-]+` 规则。
> 更完整的构建差异见 [LOCAL_DEPLOY.md](LOCAL_DEPLOY.md)。

## 使用说明

- [仓库](doc/zh-cn/repo) · [仓库组](doc/zh-cn/repo_group) · [管理员设置](doc/zh-cn/admin) · [Git](doc/zh-cn/git) · [常见问题](doc/zh-cn/common)

## 问题反馈

- 使用中遇到非预期结果，欢迎提交 `Issue`。参照：[提交问题](doc/zh-cn/contribute/bug_fix_issue.md)
- 希望新增特性，也欢迎提交 `Issue`。参照：[提交特性请求](doc/zh-cn/contribute/request_feature_issue.md)
- 发现安全漏洞请勿公开反馈，参见 [安全策略](SECURITY.md)

## 贡献代码

欢迎提交 `PR`，请确定修复 `Bug` 或新增 `Feature` 后已做适当测试。

- `PR` 内容是问题修复：请先提交 `Issue` 并在 PR 中引用。参照：[提交问题修复](doc/zh-cn/contribute/bug_fix_pr.md)
- `PR` 内容是新特性：请在 PR 中详细描述特性内容；若针对某个特性请求，请引用该 `Issue`。参照：[提交新特性](doc/zh-cn/contribute/new_feature_pr.md)

开发环境、目录结构、代码与提交规范、测试与验证流程详见 **[贡献指南](CONTRIBUTING.md)**。

## 贡献文档

欢迎指正文档错误或参与翻译。提交 `PR` 前请确定文档表述准确，参照：[提交文档](doc/zh-cn/contribute/doc_pr.md)

## 开源治理

| 文件 | 内容 |
| --- | --- |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 开发环境、代码与提交规范、测试与验证、PR 流程 |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | 社区行为准则 |
| [SECURITY.md](SECURITY.md) | 漏洞私下报告流程、部署前的默认配置加固、已知限制 |
| [CHANGELOG.md](CHANGELOG.md) | 版本变更记录 |
| [.github/](.github/) | Issue 模板、PR 模板 |

## 关于

`CodeFever` 项目起初由 `蒲公英开发者服务平台` 开发和维护，2020 年 6 月上线，
2022 年 2 月以 MIT 协议开源，开放版本称为 `CodeFever Community`。

`CodeFever Next` 是在该开源版本基础上的**二次开源版本**：后端已用 Go 完整重写，
运行镜像中不再包含 PHP，应用镜像全部改为从本地源码构建。
`CodeFever Next` 保留了上游的大部分功能，并针对单机部署做了优化。

### 维护者

| | |
| --- | --- |
| 维护者 | [GiserXiao](https://github.com/AidenX0704) |
| 仓库地址 | <https://github.com/AidenX0704/codefever-next> |
| 问题反馈 | <https://github.com/AidenX0704/codefever-next/issues> |
| 安全漏洞 | 不公开反馈，流程见 [SECURITY.md](SECURITY.md) |

本仓库为**非官方衍生版本**，由上表维护者独立开发，与蒲公英官方团队无隶属关系；
上游的商标、Logo 与其他知识产权仍归原权利人所有，本仓库沿用其视觉资产仅作版本标识。

## License

[MIT](LICENSE) © CodeFever authors

本仓库保留原始 CodeFever 项目的 LICENSE 与版权声明（Copyright © 2022 CodeFever authors）。
二次开发部分的著作权归本仓库维护者所有，同样以 MIT 协议发布。

## 致谢

- [CodeFever](https://codefever.cn) —— 原始项目与产品设计
- [CodeIgniter](https://codeigniter.com/) —— 历史 PHP 实现所用的框架
- 所有为 Go 后端重写、契约回归与文档完善提交过代码的贡献者
