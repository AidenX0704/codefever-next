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
[![CI](https://img.shields.io/badge/CI-passing-4CAF50?logo=githubactions&logoColor=white)](.github/workflows/ci.yml)

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
PHP 进程、PHP 可执行文件与 PHP 配置。

| 维度 | 状态 |
| --- | --- |
| 后端 | Go（`backend/`），替换原 CodeIgniter 2.1.1 + PHP 7.4 实现 |
| API 契约 | 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调，**契约回归 123/123 符合预期** |
| Web 路由 | 43 条服务端渲染路由 + 5 个页面，与 PHP 版**逐字节一致（43/43）**（品牌文案 1 处属有意变更，基线已同步） |
| 前端 | React 17 + MUI v4，产物 `www/view/build` 随仓库提交；**依赖未重装**，仅品牌文案随二次开源更新 |
| 运行时 | 镜像内 **无 PHP、无 php-fpm、无 crond** |
| 镜像 | **全部从本地源码构建**，不依赖任何上游预构建镜像 |
| 体积 | 前端 vendor 由 23.2MB / 2072 个文件裁剪至 3.5MB / 27 个文件（只删可证明无引用的） |

> 迁移的完整依赖面清单、稳定性门禁与验证数据见 [`docs/PHP_REMOVAL.md`](docs/PHP_REMOVAL.md)。

## 功能特性

- 🆓 **永久免费:** 无任何付费墙与功能阉割
- ❤️ **完整开源:** MIT 协议，无编译或加密代码
- 🥬 **轻量安装:** 一条 `docker compose up -d --build` 即可完成部署
- ⚡️ **性能高效:** 1 核 CPU / 1G 内存即可运行；实测 64 并发下 p99 ≈ 178 ms、错误率 0
- ♾️ **无限仓库:** 无仓库数量与容量限制
- ⌨️ **代码对比:** 支持多版本高亮差异对比
- 🛤️ **分支管理:** 完整支持 Git 原生分支与 Tag
- 🙋️ **合并请求（Merge Request）:** 支持多人协作评审
- 🙅 **分支保护:** 保护分支让提交安全可控
- 👥 **多人协作:** 角色与权限管理
- 🔌 **Webhook:** 可与 Jenkins / Jira 等外部系统集成
- 🖥️ **管理后台:** 超级管理员可管理所有项目与用户
- 💬 **中英双语:** 原生支持中文与英文
- 🔑 **SSH 推送:** 支持 SSH 公钥认证的 clone / pull / push

## 快速开始

环境要求：Docker 20.10+ 与 Docker Compose v2。

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

启动完成后：

| 项 | 值 |
| --- | --- |
| Web UI | <http://localhost:9080> |
| 默认账号 | `root@codefever.cn` |
| 默认密码 | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh -p 2222 git@localhost` |

> **首次登录后请立即修改密码并绑定 MFA 设备。**

常用命令：

```shell
docker compose ps                # 查看状态
docker compose logs -f codefever-next-app  # 跟踪日志
docker compose restart codefever-next-app  # 重启应用
docker compose down              # 停止（保留数据卷）
docker compose down -v           # 停止并删除数据卷（数据全部丢失）
```

镜像**从当前源码构建**，不需要预先拉取任何预构建镜像。想换端口、换数据库镜像、
换基础镜像源，设置环境变量即可：

```shell
CODEFEVER_HTTP_PORT=18080 CODEFEVER_SSH_PORT=18022 docker compose up -d
```

完整说明见 [使用 docker-compose 安装](doc/zh-cn/installation/install_via_docker_compose.md)。
需要离线分发（`docker save` / `docker load`）、换镜像源或用 PowerShell 一键脚本，见
[本地镜像构建与部署](LOCAL_DEPLOY.md)。

<details>
<summary><b>不使用 Docker（从源码安装）</b></summary>

需要 `nginx`、`git`、`Go 1.25+`、`MySQL 8.0+`。核心步骤：

```shell
# 1. 准备配置
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml

# 2. 编译 Go 服务
cd backend && go build -o ../bin/codefever-api    ./cmd/codefever-api \
                 && go build -o ../bin/codefever-worker ./cmd/codefever-worker \
                 && go build -o ../bin/codefever-hook   ./cmd/codefever-hook

# 3. 初始化数据库
cd ../misc && sh ./create_db.sh
```

完整的 15 步指南见 [从零开始安装](doc/zh-cn/installation/install_from_scratch.md)。

</details>

## 项目状态

### 已完成

- ✅ **PHP 运行时下线** —— 后端 100% Go 化，镜像内无 PHP / php-fpm / crond
- ✅ **本地源码构建** —— 唯一配方 `Dockerfile`，不再依赖任何上游预构建镜像
- ✅ **契约回归** —— API 123/123，SSR 路由 43/43 与 PHP 版逐字节一致
- ✅ **前端瘦身** —— vendor 精确裁剪（23.2MB / 2072 文件 → 3.5MB / 27 文件）
- ✅ **开源治理齐备** —— CONTRIBUTING / CODE_OF_CONDUCT / SECURITY / CHANGELOG / CI / Issue & PR 模板
- ✅ **中英文档重写** —— 安装、配置、使用类文档全部按 Go 后端重写并对齐

### 计划中

- ⬜ 解除仓库名连字符限制（扩展 nginx vhost 规则）
- ⬜ SSH 公钥支持 ed25519 / ECDSA
- ⬜ 口令哈希从 `legacy` 平滑切换到 `argon2id`
- ⬜ CI 增加镜像构建与契约回归的夜间流水线

> 认领上面任意一项，或直接提交 Issue / PR，详见 [贡献指南](CONTRIBUTING.md)。

## 架构一览

```
                        ┌───────────────────────────────┐
   Browser  ──HTTP──▶   │  nginx                        │
   git client ──SSH──▶  │   ├── /api /user /doc …  ──▶  │ 127.0.0.1:9001
                        │   ├── /*.git              ──▶  │ 127.0.0.1:27555
                        │   └── /static /app        ──▶  │ 静态产物
                        └───────────────────────────────┘
                                          │
      ┌───────────────────────────────────┴────────────────────────────────┐
      │                                                                │
┌─────▼──────────────────┐                              ┌──────────────────▼─────┐
│ codefever-api  (Go)    │                              │ http-gateway   (Go)   │
│  /api/*  /apiv1/*      │                              │  Git over HTTP        │
│  43 条 web 路由        │                              └────────────────────────┘
│  5 个服务端渲染页面     │
└─────┬──────────────────┘                              ┌────────────────────────┐
      │                                                 │ ssh-gateway/shell (Go) │
┌─────▼──────────────────┐                              │  Git over SSH forced  │
│ codefever-worker (Go)  │                              │  command              │
│  事件 / 站内信 / 邮件   │                              └────────────────────────┘
│  webhook 投递           │
└────────────────────────┘
      │
┌─────▼──────────────────┐   ┌──────────────┐   ┌───────────────┐
│ codefever-hook  (Go)   │──▶│  git-storage │   │  MySQL 8.0    │
│  pre-receive / update  │   │  裸仓库       │   │  cc_* 表      │
│  post-receive          │   └──────────────┘   └───────────────┘
└────────────────────────┘
```

| 目录 | 说明 |
| --- | --- |
| `backend/` | Go 后端（API / worker / git 钩子 / 契约测试工具），Go module `codefever-next/backend` |
| `http-gateway/` | Git over HTTP 网关 |
| `ssh-gateway/shell/` | Git over SSH forced command |
| `www/` | 前端源码 `www/view/src` + 已构建产物 `www/view/build` + 静态资源 |
| `misc/` | nginx / supervisor 配置模板、数据库结构与初始化脚本、git 钩子 |
| `git-storage/` | 裸仓库存放目录（Docker 数据卷挂载点，仓库内只留空目录骨架） |
| `doc/` | 用户文档（`zh-cn` / `en-us`） |
| `docs/` | 后端重写的分析、计划与实施记录 |
| `scripts/` | 本地构建、运行、复验与截图比对脚本 |
| `.github/` | CI、Issue 模板、PR 模板、Dependabot 配置 |

## 配置说明

### 编排层环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | 宿主 HTTP 端口 |
| `CODEFEVER_SSH_PORT` | `2222` | 宿主 Git SSH 端口 |
| `CODEFEVER_DB_PASS` | `123456` | 数据库 root 口令 |
| `CODEFEVER_IMAGE` | `codefever-next:local` | 应用镜像名 |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | 数据库镜像（**必须 8.0+**，见下） |
| `CODEFEVER_BASE_RUNTIME` | `docker.1ms.run/webdevops/php-nginx:7.4` | 构建期运行时基础镜像 |
| `CODEFEVER_BASE_GO` | `docker.1ms.run/library/golang:1.25-bookworm` | 构建期 Go 基础镜像 |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | 新口令哈希格式：`legacy`（`md5(md5(x))`）或 `argon2id` |

> 存量账号的口令哈希是历史遗留的 `legacy` 格式，默认保持 `legacy`，
> **存量账号无需重置口令即可登录**。等全部账号重置过口令后，再切成 `argon2id`。

> ⚠️ **数据库必须用 MySQL 8.0+**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED` 领取任务行，该语法自 MySQL 8.0.1 起才支持；
> 在 5.7 上这两条查询直接报语法错误，会让活动流、站内信、邮件通知与
> webhook **静默失效**（表现为日志里每 2 秒刷一次 `SKIP LOCKED` 报错）。

### 应用配置

`env.yaml`（数据库、邮件、网关 token、站点信息）与 `config.yaml`（站点标题、备案号等）
由 `env.template.yaml` / `config.template.yaml` 复制而来，容器启动时会在
`env.yaml` 中自动生成随机的 `session_key` 与 `totp_salt`。

## 开发与测试

```shell
cd backend

go build ./...          # 编译
go vet ./...            # 静态检查
gofmt -l .              # 格式检查
go test ./internal/...  # 单元测试
```

> ⚠️ **Windows 上建议在 Linux 容器里跑测试** —— 部分环境会拦截 `config.test.exe`
> 这类文件名，报 `Access is denied`（本机策略问题，与代码无关）。
> 容器化的完整命令见 [贡献指南 · 测试与验证](CONTRIBUTING.md#单元测试)。

一键复验流水线（编译 + 单测 + 契约比对 + 截图比对 + 镜像打包）：

```powershell
pwsh -File scripts/verify.ps1
```

前端产物重建（**不要**重装依赖，`@pgyer/mui-theme-preset` 声明为 `latest`，重装即漂移）：

```shell
cd www/view && yarn install && yarn build
```

契约基线在 `backend/testdata/contract/`，Go 侧逐条比对 123 个端点的响应形状。

> 行尾统一 LF，由 `.gitattributes` 保证；编辑器行为由 `.editorconfig` 约定。

## 从 PHP 版本迁移

如果你在跑旧的上游 PHP 版本：

- **数据不需要迁移** —— 数据库结构没有变化，会话需要重新登录一次
  （旧会话在客户端 Cookie 里，新会话在服务端 `cc_sessions` 表，两者互不认得）
- **口令哈希默认仍是 `legacy`** —— 存量账号免重置口令即可登录
- `admin.systemStatus` 接口少了 `php` / `crond` 两个键（PHP 下线后没有意义）
- `repository.fileContent` 从 PHP 时代的恒定 500 修正为 200

## 已知限制

以下限制继承自上游实现，二次开源版本尚未解除：

| 限制 | 影响 | 规避方式 |
| --- | --- | --- |
| 仓库名不能含连字符（HTTP 访问） | nginx vhost 规则为 `^/[0-9a-z_]+/[0-9a-z_]+\.git`，不含 `-` | 建仓时避开 `-`（如 `demorepo`）；**SSH 方式不受此限制** |
| SSH 公钥只识别 RSA | `base64_decode(整行公钥)` 后正则匹配类型，ed25519 会被误判成 `ssh-rsa` | 用 `ssh-keygen -t rsa` 生成密钥 |
| 添加 SSH 公钥时 `key` 字段只传 base64 主体 | 不含 `ssh-rsa` 前缀与注释 | 与前端 `UserSettingSSHKey.js` 的解析保持一致 |

> 更完整的限制清单、以及与上游构建配方的逐条差异，见 [本地镜像构建与部署](LOCAL_DEPLOY.md)。

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
| [.github/](.github/) | CI 工作流、Issue 模板、PR 模板、Dependabot 配置 |

## 关于

`CodeFever` 项目起初由 `蒲公英开发者服务平台` 开发和维护，2020 年 6 月上线，
2022 年 2 月以 MIT 协议开源，开放版本称为 `CodeFever Community`。

`CodeFever Next` 是在该开源版本基础上的**二次开源版本**：后端已用 Go 完整重写，
运行镜像中不再包含 PHP，镜像全部改为从本地源码构建。
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
