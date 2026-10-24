# 使用 docker-compose 安装

## 安装步骤

### 编排文件说明

仓库根目录的 `docker-compose.yml` 是本项目**唯一**的编排文件，包含两个服务：

| 服务 | 容器名 | 说明 |
| --- | --- | --- |
| `codefever` | `codefever-next-app` | 应用本体，镜像从当前源码构建（`Dockerfile`） |
| `db` | `codefever-next-db` | MySQL 8.0.x |

数据默认持久化在仓库根目录的 **`codefever-data/`** 下（绑定挂载，已被 `.gitignore` 忽略）：

| 宿主机目录 | 内容 |
| --- | --- |
| `codefever-data/mysql/` | MySQL 数据 |
| `codefever-data/git-storage/` | 裸仓库与 hooks 软链 |
| `codefever-data/file-storage/` | 上传文件/附件 |
| `codefever-data/config/` | `config.yaml` + `env.yaml`（管理后台修改的主机名、SMTP、token） |

首次启动时 Docker 会创建目录，容器入口脚本会创建 `git-storage/hooks`、`deploy/hooks`
等兼容软链，不需要手工准备。可用 `CODEFEVER_DATA_DIR=/绝对路径` 换位置。

### 拉取代码并启动

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

首次构建需要能访问 Docker Hub（`webdevops/php-nginx:7.4`、`golang:1.25-bookworm`、
`mysql:8.0`）、Debian buster 归档源和 Go module 代理。国内网络见下方“镜像源”。

> **卡在 `load metadata ... docker.io` / `auth.docker.io` 超时**：基础镜像还没拉到本机。
> 先在仓库根目录执行 `pwsh -File scripts/build-local.ps1 -SkipPackage`（通过镜像源拉取并
> 打本地 tag），或按“镜像源”设置 `CODEFEVER_BASE_*` 后重试。本地已有镜像时，
> Compose 会直接复用，不再访问 Docker Hub。

`--build` 会强制用当前源码重新构建镜像；镜像已存在时直接 `docker compose up -d` 即可复用。

1. 服务启动后访问 **<http://localhost:9080>**（或 `http://<server ip>:9080`）登录。
2. 默认端口是 `9080`（HTTP）与 `2222`（Git SSH）。80/22/8080 常被占用，因此默认改了端口。
   需要改端口时设置环境变量：

   ```shell
   export CODEFEVER_HTTP_PORT=18080
   export CODEFEVER_SSH_PORT=18022
   docker compose up -d
   ```

   容器入口会按实际端口自动写 `config.yaml` 的 `host`/`ssh`，clone 面板会显示
   `http://localhost:18080/...` 与 `ssh://git@localhost:18022/...`。
   反向代理/域名/外部端口不一致时，显式覆盖：

   ```shell
   export CODEFEVER_SITE_HOST=https://git.example.com
   export CODEFEVER_SITE_SSH=ssh://git@git.example.com:2222/
   docker compose up -d
   ```

3. 如果服务异常，可以进入 Shell 人工维护，或 `docker compose restart codefever`。
4. 默认管理员用户：`root@codefever.cn`，密码：`123456`。**登录后请立即修改密码、
   绑定 MFA 设备，并在设置中关闭开放注册（`allowRegister` 默认为 true）。**

### 可配置的环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | 宿主 HTTP 端口 |
| `CODEFEVER_SSH_PORT` | `2222` | 宿主 Git SSH 端口 |
| `CODEFEVER_SITE_HOST` | 按 HTTP 端口生成 | clone 面板的 HTTP 地址 |
| `CODEFEVER_SITE_SSH` | 按 SSH 端口生成 | clone 面板的 SSH 地址，必须用 `ssh://git@host:port/` 形式 |
| `CODEFEVER_DB_PASS` | `123456` | 数据库 root 口令；支持特殊字符 |
| `CODEFEVER_IMAGE` | `codefever-next:local` | 应用镜像名 |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | 数据库镜像（使用 **8.0.x**） |
| `CODEFEVER_BASE_RUNTIME` | `webdevops/php-nginx:7.4` | 构建期运行时基础镜像 |
| `CODEFEVER_BASE_GO` | `golang:1.25-bookworm` | 构建期 Go 基础镜像 |
| `CODEFEVER_GOPROXY` | `https://proxy.golang.org,direct` | 构建期 Go module 代理 |
| `CODEFEVER_APT_MIRROR` | `https://archive.debian.org` | Debian buster 归档源 |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | 新口令哈希格式，`legacy` 或 `argon2id` |

数据库连接同时接受 `CODEFEVER_MYSQL_HOST/PORT/USER/PASSWORD/DB`（优先）和旧版
`DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME`（兼容）。Compose 已同时传入两套。

### 邮件（SMTP）

镜像不安装 MTA，必须配置 SMTP 才能发通知邮件：

```shell
export CODEFEVER_SMTP_HOST=smtp.example.com
export CODEFEVER_SMTP_PORT=587
export CODEFEVER_SMTP_USER=git@example.com
export CODEFEVER_SMTP_PASSWORD='<口令>'
export CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
export CODEFEVER_SMTP_TLS=starttls     # 465 端口自动按 implicit TLS
docker compose up -d
```

> 未配置 SMTP 时，邮件发送会回退到系统 `mail`；镜像内通常没有 `mail`，因此通知
> 只会记录失败日志，不影响 Git 操作。

### 国内镜像源

```shell
export CODEFEVER_BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4
export CODEFEVER_BASE_GO=docker.1ms.run/library/golang:1.25-bookworm
export CODEFEVER_MYSQL_IMAGE=docker.1ms.run/library/mysql:8.0
export CODEFEVER_GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export CODEFEVER_APT_MIRROR=https://mirrors.aliyun.com/debian-archive
docker compose up -d --build
```

### 接入外部数据库

如果数据库独立于容器之外（例如云数据库），把 `db` 服务注释掉，**同时删除
`codefever` 的 `depends_on` 段**（否则 compose 会报依赖不存在），再调整数据库变量：

```yaml
  codefever:
    # depends_on:
    #   db:
    #     condition: service_healthy
    environment:
      CODEFEVER_MYSQL_HOST: "your-db-host"
      CODEFEVER_MYSQL_PORT: "3306"
      CODEFEVER_MYSQL_USER: "root"
      CODEFEVER_MYSQL_PASSWORD: "your-password"
      CODEFEVER_MYSQL_DB: "codefever_community"
```

外部数据库必须是 MySQL 8.0.x（或 MariaDB 10.6+），并允许应用容器访问。

### 注意：数据库版本

事件 worker 与 webhook 投递器用 `FOR UPDATE SKIP LOCKED` 领取任务行，该语法自
**MySQL 8.0.1** 起才支持。MySQL 5.7 上这两条查询直接报语法错误，活动流、站内信、
邮件通知与 webhook 会**静默失效**；日志里每 2 秒刷一次 `SKIP LOCKED` 报错。

compose 还加了 `--default-authentication-plugin=mysql_native_password`：MySQL 8.0 默认的
`caching_sha2_password` 不在应用容器内 MariaDB 客户端的插件目录里，建库会失败。
该参数在 **MySQL 8.4+ 已移除**，所以请使用 `mysql:8.0` / 8.0.x，不要用 8.4/9.x。

### 构建自己的镜像

编排文件已经内置 `build` 段，指向仓库根目录的 `Dockerfile`：

```shell
docker compose build codefever
```

推到自己的镜像仓库：

```shell
docker tag codefever-next:local you-name/codefever-next:latest
docker push you-name/codefever-next:latest
```

然后把 `CODEFEVER_IMAGE` 设成你自己的镜像名。

### 服务维护

```shell
docker compose ps                # 状态（服务名 codefever / db）
docker compose logs -f codefever # 跟踪应用日志
docker compose restart codefever # 重启应用
docker compose down              # 停止（保留 codefever-data 数据目录）
docker compose down -v           # 停止并删除容器/网络（codefever-data 不受影响）
docker exec -it codefever-next-app sh
```

容器内常驻进程由 supervisor 管理；管理端概览页可查看 nginx / codefever / sendmail
（SMTP）状态。服务维护详见 [管理员设置/概览和系统服务](../admin/dashboard.md)。

## 此文档适用条件

如果你的情况符合以下条件，请改用 [从零开始安装](install_from_scratch.md)：

- 学习和技术交流
- 需要做定制化修改
- `Docker 镜像安装` 不能满足处理 `Bug` 和提交 `PR` 的需求
- Docker 镜像不能在当前操作系统或硬件架构上使用

参见：[从零开始安装](install_from_scratch.md)。
