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
| 前端开发 | Node.js **20+** / npm（见 `web/package.json`） |
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

前端源码在 `web/`（React 19 + TypeScript + Vite，自研设计令牌与组件库，零 UI 框架依赖），构建产物 `web/build` 随仓库提交。

```shell
cd web
npm ci
npm run verify   # typecheck + lint + check:ui + build（输出到 web/build）
```

## 目录结构

```
backend/           单一 Go module（codefever-next/backend）
  cmd/             api / worker / hook / http-gateway / ssh-gateway / 冒烟与压测工具
  internal/        内部实现，按领域划分
  deploy/          数据库脚本、nginx/supervisor 模板、容器入口、服务脚本
web/               前端源码（src/）与预构建产物（build/ 必须随仓库提交）
doc/               用户文档（zh-cn / en-us；运行时会进镜像）
docs/              设计规范（design-v5/DESIGN.md）
scripts/           本地构建、运行与质量门禁脚本
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

- 2 空格缩进，遵循 `web/eslint.config.js` 与 TypeScript strict 模式（`npm run verify`）。
- 样式一律使用 `web/src/theme/theme.css` 中的语义/尺寸令牌，禁止硬编码颜色、间距、圆角。
- 图标一律从 `web/src/config/icons.ts` 导入。

### 通用

- 行尾统一 LF（由 `.gitattributes` 保证，不要在编辑器里提交 CRLF）。
- **`web/build` 是例外，必须随仓库提交**；除此之外不要提交构建产物、依赖目录、
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

### 维护者质量门禁

```powershell
pwsh -File scripts/verify.ps1
```

当前脚本执行：gofmt → `go build ./...` → Linux 交叉编译（`GOOS=linux`）→
`go vet ./...` → `go test ./internal/...` → `docker compose config`；只有加
`-BuildImage` 才会额外 `docker build`。涉及前端改动时另行执行 `cd web && npm run verify`。

CI（`.github/workflows/ci.yml`）在 Go 1.23 上运行 gofmt / build / Linux 交叉编译 /
vet / test，以及 `docker compose config`。提交 PR 前建议在本地跑同一组命令。

## 提交 Pull Request

1. **先开 Issue**：修复缺陷请先开 Issue 并在 PR 中引用；新特性请在 PR 中详细描述。
2. **建分支**：`git checkout -b feature/xxx` 或 `fix/xxx`。
3. **同步主干**：`git rebase origin/main`（本仓库不接受 merge 提交来同步主干）。
4. **自测**：至少跑通 `cd backend && go build ./... && go vet ./... && go test ./internal/...`；
   涉及前端变更的，运行 `cd web && npm run verify` 并提交重建后的 `web/build`。
5. **补文档**：改了配置项、API 或部署方式，必须同步改 `doc/`、`README*.md`、
   `LOCAL_DEPLOY.md` 等相关文档；`.github/` 模板也属于文档。
6. **PR 描述**：说明改了什么、为什么、怎么验证的，关联 Issue。

### PR 标题格式

```
feat(repo): 支持按 tag 过滤提交列表
fix(git): 修复 tag 名包含空格时被截断的问题
```

### 评审

- 至少需要一位维护者 approve。
- 本地质量门禁（gofmt / build / vet / test / compose config；前端 `npm run verify`）必须全绿。
- 有未解决的讨论评论请勿合并。

## 行为兼容性约定

- 改动 API 响应结构、错误码属于**破坏性变更**，需要在 PR 中显式标注，并同步更新 `doc/` 与 `CHANGELOG.md`。
- 内部实现（函数签名、数据库访问方式、包结构）不受此约束。

## 许可证

提交即表示你同意你的贡献以 MIT 协议授权，与本仓库保持一致。
