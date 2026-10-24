# Docker 镜像安装

> 本项目的**应用镜像**从本地源码构建（`Dockerfile`）；构建期仍需要拉取
> `webdevops/php-nginx:7.4` 与 `golang:1.25-bookworm` 两个基础镜像。PHP 运行时在
> 构建末段被删除，最终镜像里不存在任何 PHP 可执行文件。

## 安装步骤

### 镜像说明

`CodeFever Next` 的镜像以 `webdevops/php-nginx:7.4` 为基底，只借用其中的
`nginx` / `supervisor` / `sshd` / 容器入口集成。应用逻辑由**单 Go module**编译出的
5 个二进制提供：

| 产物 | 来源 | 作用 | 监听 |
| --- | --- | --- | --- |
| `bin/codefever-api` | `backend/cmd/codefever-api` | API / 服务端渲染页面 / 文件 | `127.0.0.1:9001` |
| `bin/codefever-worker` | `backend/cmd/codefever-worker` | 事件、站内信、邮件、webhook 投递 | — |
| `bin/codefever-hook` | `backend/cmd/codefever-hook` | git 钩子 | — |
| `bin/codefever-http-gateway` | `backend/cmd/codefever-http-gateway` | Git over HTTP 网关 | `127.0.0.1:27555` |
| `bin/codefever-ssh-gateway` | `backend/cmd/codefever-ssh-gateway` | Git over SSH forced command | — |

镜像内还会包含 `doc/`（应用内文档中心）与 `LICENSE`；前端产物 `www/view/build`
随仓库提交，因此构建期不需要 Node.js，也不需要访问 npm/GitHub。

### 构建镜像

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker build -f Dockerfile -t codefever-next:local .
```

构建期需要访问 Docker Hub（两个基础镜像）、Debian buster 归档源和 Go module 代理。
国内网络可覆盖：

```shell
docker build -f Dockerfile \
    --build-arg BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4 \
    --build-arg BASE_GO=docker.1ms.run/library/golang:1.25-bookworm \
    --build-arg APT_MIRROR=https://mirrors.aliyun.com/debian-archive \
    --build-arg GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
    -t codefever-next:local .
```

`--build-arg REGISTRY=docker.1ms.run` 也等价于同时改两个基础镜像前缀。Windows/PowerShell
还可以用：

```powershell
pwsh -File scripts/build-local.ps1
```

### 运行

**推荐直接使用 [docker-compose 安装](install_via_docker_compose.md)**：

```shell
docker compose up -d --build
```

Compose 会自动启动 MySQL 8.0、创建数据库、按 9080/2222 端口启动应用。
如果你已经有一个外部 MySQL 8.0.x，并且希望用 `docker run` 启动应用容器，必须显式传入
数据库地址，并把配置/仓库/附件挂载到 `codefever-data/`；否则不要使用单容器方式：

```shell
mkdir -p codefever-data/config codefever-data/git-storage codefever-data/file-storage

docker container run -d --name codefever \
  -p 9080:80 -p 2222:22 \
  -v "${PWD}/codefever-data/config:/data/www/codefever-community/config" \
  -v "${PWD}/codefever-data/git-storage:/data/www/codefever-community/git-storage" \
  -v "${PWD}/codefever-data/file-storage:/data/www/codefever-community/file-storage" \
  -e CODEFEVER_MYSQL_HOST='<mysql-host>' \
  -e CODEFEVER_MYSQL_PORT=3306 \
  -e CODEFEVER_MYSQL_USER=root \
  -e CODEFEVER_MYSQL_PASSWORD='<口令>' \
  -e CODEFEVER_MYSQL_DB=codefever_community \
  -e CODEFEVER_HTTP_PORT=9080 \
  -e CODEFEVER_SSH_PORT=2222 \
  codefever-next:local
```

> 单容器示例**不包含 MySQL 容器**。没有可用的外部 MySQL 8.0.x 时，请使用 Compose；
> 不要照抄旧文档里“只启动应用容器”的命令，那会因 `DB_HOST=db` 无法解析而失败。

启动与验证：

1. 首次启动会初始化数据库，通常需要 1-2 分钟；`docker logs -f codefever` 里出现
   `=== 初始化完成 ===` 表示完成。
2. 访问 <http://localhost:9080> 登录。
3. 默认管理员：`root@codefever.cn` / `123456`。**登录后立即修改密码、绑定 MFA，
   并在设置中关闭开放注册。**
4. 邮件通知需要配置 `CODEFEVER_SMTP_*`；镜像不安装 MTA。

### 离线分发

```shell
docker save -o codefever-next.tar codefever-next:local
```

目标机器还需要 MySQL 8.0.x 镜像（若用 Compose）：

```shell
docker load -i codefever-next.tar
docker load -i mysql-8.0.tar
docker compose up -d
```

### 服务维护

见 [管理员设置/概览和系统服务](../admin/dashboard.md#系统服务状态及维护)。
容器内的常驻进程由 supervisor 管理：

```shell
docker exec -it codefever-next-app sh
supervisorctl status
```

## 此文档适用条件

如果你的情况符合以下条件，请改用 [从零开始安装](install_from_scratch.md)：

- 需要直接修改/调试 Go 源码或提交 PR；
- 需要使用非 Docker 支持的操作系统/硬件架构；
- 需要对 nginx/systemd/数据库做深度定制。

参见：[从零开始安装](install_from_scratch.md)。
