# Docker 镜像安装

> 本项目的镜像**全部从本地源码构建**（`Dockerfile`），不再依赖任何上游预构建镜像。

## 安装步骤

### 镜像说明

`CodeFever Next` 的镜像以 `webdevops/php-nginx:7.4` 为基底 —— 只借用其中的
`nginx` / `supervisor` / `sshd` / 容器入口集成，**PHP 运行时（解释器、php-fpm、PHP 配置、
PHP 入口脚本、crond）在构建期被整体删除**，最终镜像里不存在任何 PHP 可执行文件。

应用逻辑由两段 Go 多阶段构建产出：

| 产物 | 来源 | 作用 | 监听 |
| --- | --- | --- | --- |
| `bin/codefever-api` | `backend/cmd/codefever-api` | 应用后端（API / 服务端渲染页面 / 文件） | `127.0.0.1:9001` |
| `bin/codefever-worker` | `backend/cmd/codefever-worker` | 事件、站内信、邮件、webhook 投递 | — |
| `bin/codefever-hook` | `backend/cmd/codefever-hook` | git 钩子（替代原 PHP 钩子脚本） | — |
| `http-gateway/main` | `http-gateway/` | Git over HTTP 网关 | `127.0.0.1:27555` |
| `ssh-gateway/shell/main` | `ssh-gateway/shell/` | Git over SSH forced command | — |

前端产物 `www/view/build` 随仓库提交，因此构建期**不需要** Node.js，
也不需要访问 npm / GitHub。

### 构建镜像

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker build -f Dockerfile -t codefever-next:local .
```

构建期需要拉取 3 个基础镜像。若所在网络无法直连 Docker Hub，可以走镜像加速：

```shell
docker build -f Dockerfile \
    --build-arg REGISTRY=docker.1ms.run \
    -t codefever-next:local .
```

或者先用 `scripts/build-local.ps1`（Windows / PowerShell）一键完成
「拉取基础镜像 → 构建 → 导出 tar」：

```powershell
pwsh -File scripts/build-local.ps1
```

### 运行

**推荐**使用 [docker-compose 安装](install_via_docker_compose.md)：

```shell
docker compose up -d --build
```

也可以直接用 `docker run`：

```shell
docker container run \
  -d --name codefever \
  -p 80:80 -p 22:22 \
  -v codefever-next-db:/var/lib/mysql \
  -v codefever-git-storage:/data/www/codefever-community/git-storage \
  -v codefever-file-storage:/data/www/codefever-community/file-storage \
  -e DB_HOST=db -e DB_PORT=3306 -e DB_USER=root -e DB_PASS=123456 \
  -e DB_NAME=codefever_community \
  -e CONTAINER_UID=git \
  -it codefever-next:local
```

> `DB_HOST` 请填数据库的实际地址。若数据库不在本机（例如云数据库），把 `DB_HOST` 改成对应地址、
> `DB_PORT` 改成对应端口即可。

1. 首次启动需要初始化数据库，可能需要 1-2 分钟才能访问。
   查看初始化进度：`docker container logs codefever -f`，日志出现 `=== IMPORTANT NOTICE ===` 表示初始化完成。
2. 初始化完成后访问 `http://127.0.0.1` 或 `http://<server ip>` 登录。
3. 如果希望用宿主机 `22` 端口作为 Git 的 SSH 协议端口，请先把宿主机的 SSH 服务端口改成其他端口。
4. 如果服务异常，可以进入 Shell 人工维护，也可以直接重启容器。
5. 默认管理员用户：`root@codefever.cn`，密码：`123456`。**登录后请立即修改密码并绑定 MFA 设备。**

### 离线分发

```shell
docker save -o codefever-next.tar codefever-next:local
```

在目标机器上导入并启动：

```shell
docker load -i codefever-next.tar
docker compose up -d
```

### 服务维护

服务维护请参见 [管理员设置/概览和系统服务](../admin/dashboard.md) 中的 `系统服务状态及维护` 一节

## 此文档适用条件

如果你的情况符合以下条件，你需要使用 [从零开始安装](install_from_scratch.md) 的方式安装 `CodeFever`，否则请继续使用 `Docker 镜像安装` 方式安装。

- 学习和技术交流
- 需要做定制化修改
- `Docker 镜像安装` 不能满足处理 `Bug` 和提交 `PR` 的需求
- Docker 镜像不能在当前 `操作系统` 或 `硬件架构` 上使用

参见: [从零开始安装](install_from_scratch.md)
