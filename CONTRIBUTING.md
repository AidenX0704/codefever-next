# 贡献指南

感谢你愿意为 CodeFever Next 付出时间。这份文档说明如何把代码跑起来、代码写成什么风格、
以及一个 PR 从提交到合入要走哪些步骤。

---

## 目录

- [环境要求](#环境要求)
- [本地开发](#本地开发)
- [目录结构](#目录结构)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [测试与验证](#测试与验证)
- [提交 Pull Request](#提交-pull-request)

---

## 环境要求

| 用途 | 要求 |
| --- | --- |
| 后端开发 | Go **1.23+**（`backend/go.mod` 声明 `go 1.23.0`；Docker 构建使用 1.25 基础镜像） |
| 前端开发 | Node.js 16.10+ / Yarn |
| 完整环境 | Docker 20.10+ 与 Docker Compose v2 |
| 数据库 | MySQL **8.0.x** 或 MariaDB **10.6+**（事件 worker 用 `FOR UPDATE SKIP LOCKED`，5.7 / 10.3 不支持；MySQL 8.4+ 的认证参数变化需另行适配） |

> 后端是**单一 Go module**：`http-gateway` 与 `ssh-gateway` 已并入
> `backend/cmd/codefever-http-gateway` / `backend/cmd/codefever-ssh-gateway`，
> 不再需要 GOPATH 模式或构建期 `sed`。直接 `cd backend && go build ./...` 即可。

## 本地开发

### 方式一：Docker（推荐，最接近生产拓扑）

```shell
# 构建并启动（镜像从当前源码构建；首次需要网络拉基础镜像）
docker compose up -d --build

# 观察日志（服务名是 codefever）
docker compose logs -f codefever

# 停止
docker compose down
```

访问 <http://localhost:9080>，默认账号 `root@codefever.cn` / `123456`。
改端口用 `CODEFEVER_HTTP_PORT` / `CODEFEVER_SSH_PORT`；域名/反代用
`CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH`。

### 方式二：只跑 Go 后端（快速迭代）

**必须先准备仓库根目录的配置文件**（`-base` 指向仓库根，不是 `backend/`）：

```shell
cd /path/to/codefever
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
# 至少把 env.yaml 的 session key / totp salt / gateway token 换成随机值；
# 只是为了本地跑通也可以先留占位符，但日志会提示“gateway.token 仍是模板占位符”。
```

从 `backend/` 启动（`-base ..` 指回仓库根）：

```shell
cd backend
go run ./cmd/codefever-api -base .. -addr 127.0.0.1:9001
```

数据库连接可通过环境变量覆盖（entrypoint 与后端都识别 `CODEFEVER_MYSQL_*`；
旧版 `DB_*` 仍兼容）：

```shell
cd backend
CODEFEVER_MYSQL_HOST=127.0.0.1 CODEFEVER_MYSQL_PORT=3306 \
CODEFEVER_MYSQL_USER=root CODEFEVER_MYSQL_PASSWORD=123456 \
CODEFEVER_MYSQL_DB=codefever_community \
go run ./cmd/codefever-api -base .. -addr 127.0.0.1:9001
```

> 从仓库根直接 `go run ./backend/cmd/codefever-api` **不可用**：仓库根没有
> `go.mod`，Go 会报 `cannot find main module`。要么 `cd backend` 后用 `-base ..`，
> 要么先在仓库根用 `GO111MODULE=on GOWORK=off` 建 workspace（不推荐）。

### 前端

前端源码在 `www/view/`，构建产物 `www/view/build` 随仓库提交。

```shell
cd www/view
yarn install
yarn build
```

> ⚠️ **不要重装前端依赖**。`www/view/package.json` 里 `@pgyer/mui-theme-preset` 声明为
> `latest`，`yarn install` 会拉取新版本导致 UI 漂移。`yarn.lock` 已把 `latest` 锁到 `0.0.3`，
> 改动依赖时必须同时提交 `yarn.lock`。

> ⚠️ **Node.js 17 及以上需要额外的 OpenSSL 兼容开关**。本项目用的是 webpack 4，
> 它用 MD4 计算内容哈希，而 OpenSSL 3（Node 17+ 默认）已把 MD4 移出默认 provider，
> 构建会失败并报 `error:0308010C:digital envelope routines::unsupported`。
> 两种解决方式：
>
> ```shell
> # 方式一：本次构建带上兼容开关（推荐）
> NODE_OPTIONS=--openssl-legacy-provider yarn build
>
> # 方式二：用 Node.js 16 构建（自带 OpenSSL 1.1.1，无需开关）
> ```
>
> 另外，构建脚本会**先清空** `www/view/build` 再输出。若构建中途失败，
> 用 `git checkout -- www/view/build` 即可恢复已提交的产物。

## 目录结构

```
backend/           单一 Go module（codefever-next/backend）
  cmd/             api / worker / hook / http-gateway / ssh-gateway / 契约工具
  internal/        内部实现，按领域划分
  deploy/          数据库脚本、nginx/supervisor 模板、容器入口、服务脚本
  contract/        API 契约 manifest
  testdata/        契约基线与页面快照
www/view/          前端源码与构建产物（build/ 必须提交）
www/static/        服务端渲染页面用的静态资源
doc/               用户文档（zh-cn / en-us；运行时会进镜像）
docs/              后端重写的分析、计划与实施记录
scripts/           本地构建、运行、复验与截图比对脚本
Dockerfile         镜像构建配方
docker-compose.yml 编排（codefever + mysql）
```

仓库根**没有** `misc/`、`http-gateway/`、`ssh-gateway/`、`git-storage/`；
这些内容分别收敛到 `backend/deploy/`、`backend/cmd/`，运行时目录由
Dockerfile/entrypoint 创建。

## 代码规范

### Go

```shell
cd backend
gofmt -l .        # 必须无输出
go vet ./...      # 必须无告警
go build ./...    # 必须通过（含 5 个命令）
```

- 遵循 [Effective Go](https://go.dev/doc/effective_go) 与
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)。
- 导出的标识符必须有以标识符名开头的注释。
- 错误必须包装（`fmt.Errorf("...: %w", err)`），不要用 `errors.New(fmt.Sprintf(...))`。
- 提交前跑 `go test ./internal/...`，全部通过。

### 前端

- 2 空格缩进，遵循 `www/view/package.json` 的 `eslintConfig` 与 `scripts`
  （`yarn lint` / `yarn fix`）；仓库没有独立的 `.eslintrc` 文件。
- 不引入新的全局样式污染；复用 `@pgyer/essential-component` 与
  `@pgyer/mui-theme-preset` 提供的组件与主题变量。

### 通用

- 行尾统一 LF（由 `.gitattributes` 保证，不要在编辑器里提交 CRLF）。
- **`www/view/build` 是例外，必须随仓库提交**；除此之外不要提交构建产物、依赖目录、
  运行时数据（`.gitignore`/`.dockerignore` 已覆盖）。
- 中文注释与英文标识符混用是本仓库的既有风格，保持一致即可。

## 提交规范

采用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>

<footer>
```

`type` 取值：

| type | 用途 |
| --- | --- |
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `docs` | 文档 |
| `refactor` | 重构（不含功能变化） |
| `perf` | 性能优化 |
| `test` | 测试 |
| `build` | 构建系统、依赖 |
| `chore` | 杂项 |

示例：

```
feat(repo): 支持按 tag 过滤提交列表
fix(git): 修复 tag 名包含空格时被截断的问题
docs(readme): 补充 CodeFever Next 的架构说明
```

> 仓库根目录的 `package.json` 已配置 `cz-conventional-changelog`，
> 执行 `npx cz` 可以交互式生成符合规范的提交信息（需要能访问 npm）。

## 测试与验证

### 单元测试

```shell
cd backend
go test ./internal/...
```

> ⚠️ **Windows 上请改用 Linux 容器跑测试**。部分 Windows 环境会拦截 `config.test.exe`
> 这类文件名，表现为 `open ...\config.test.exe: Access is denied` 或
> `fork/exec ...: Access is denied` —— 这是本机安全策略，与代码无关。
> 用容器跑可以完全绕开：
>
> ```shell
> docker volume create codefever-gomodcache
> docker run --rm \
>   -v "$PWD/backend:/app" \
>   -v "$PWD/doc:/doc" \
>   -v codefever-gomodcache:/go/pkg/mod \
>   -w /app \
>   -e GOFLAGS=-mod=mod \
>   -e GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
>   -e GOSUMDB=off \
>   golang:1.25-bookworm \
>   sh -c 'go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./internal/... -count=1'
> ```
>
> 两个容易踩的点：
>
> - **`doc` 目录必须挂进去** —— `backend/internal/handlers/web_doc_test.go` 用
>   `repoDocDir = "../../../doc"` 读真实文档树，缺了会直接失败；
> - 如果国内 goproxy 出现 HTTP/2 挂起，可再加 `-e GODEBUG=http2client=0`。

### 契约回归

后端保留了 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调的历史基线
（`backend/testdata/contract/`）。改动 API 后如需复跑，需要一个**已启动且可登录**的实例：

```shell
cd backend
go run ./cmd/contract-collect -base http://localhost:9080 -out testdata/contract-verify
go run ./cmd/contract-compare -baseline testdata/contract -actual testdata/contract-verify
```

比对结果有差异时，确认是有意的行为变更后，再更新基线并在 PR 描述里写明原因。
这些命令是验收/回归工具，不是部署或构建的前提。

### 维护者质量门禁

```powershell
pwsh -File scripts/verify.ps1
```

当前脚本执行：gofmt → `go build ./...` → Linux 交叉编译（`GOOS=linux`）→
`go vet ./...` → `go test ./internal/...` → `docker compose config`；只有加
`-BuildImage` 才会额外 `docker build`。它**不启动**数据库/应用，也不做契约比对、
截图比对或写操作冒烟；这些深度验证见
[docs/IMPLEMENTATION_STATUS.md](docs/IMPLEMENTATION_STATUS.md) 与 `backend/cmd/` 工具。

CI 已恢复（`.github/workflows/ci.yml`）：Go 1.23 的 gofmt / build / Linux 交叉编译 /
vet / test，以及 `docker compose config`。提交 PR 前建议在本地跑同一组命令。

## 提交 Pull Request

1. **先开 Issue**：修复缺陷请先开 Issue 并在 PR 中引用；新特性请在 PR 中详细描述。
2. **建分支**：`git checkout -b feature/xxx` 或 `fix/xxx`。
3. **同步主干**：`git rebase origin/main`（本仓库不接受 merge 提交来同步主干）。
4. **自测**：至少跑通 `cd backend && go build ./... && go vet ./... && go test ./internal/...`；
   涉及页面变更的，重建 `www/view/build` 并跑截图比对。
5. **补文档**：改了配置项、API 或部署方式，必须同步改 `doc/`、`README*.md`、
   `LOCAL_DEPLOY.md` 等下相关文档；`.github/` 模板也属于文档。
6. **PR 描述**：说明改了什么、为什么、怎么验证的，关联 Issue。

### PR 标题格式

```
feat(repo): 支持按 tag 过滤提交列表
fix(git): 修复 tag 名包含空格时被截断的问题
```

### 评审

- 至少需要一位维护者 approve。
- 本地质量门禁（gofmt / build / vet / test / compose config）必须全绿。
- 有未解决的讨论评论请勿合并。

## 行为兼容性约定

CodeFever Next 的后端在重写时承诺**逐字节保持**与原 PHP 实现的前端可见行为。
因此：

- 改动 API 响应结构、页面 HTML、错误码，都属于**破坏性变更**，需要在 PR 中显式标注。
- 确有必要做破坏性变更时，请先开 Issue 讨论，并同步更新
  `backend/testdata/contract/` 基线与 `doc/` 文档。
- 内部实现（函数签名、数据库访问方式、包结构）不受此约束。

## 许可证

提交即表示你同意你的贡献以 MIT 协议授权，与本仓库保持一致。
