# 从零开始安装

## 此文档适用条件

> 如果你的情况符合以下条件, 你可以使用 `从零开始安装` 的方式安装 `CodeFever`，否则请使用 [Docker 镜像安装](install_via_docker.md) 方式安装。

- 学习和技术交流
- 需要做定制化修改
- `Docker 镜像安装` 不能满足处理 `Bug` 和提交 `PR` 的需求
- Docker 镜像不能在当前 `操作系统` 或 `硬件架构` 上使用

> **版本说明**：本项目的后端已从 PHP 重写为 Go，**不再需要 PHP、php-fpm、Composer 与 crond**。
> 下面这份步骤按 Go 后端编写；历史文档里的 PHP 安装步骤已全部移除。

## 安装步骤

### 0. 安装前的说明

此安装步骤是推荐的 `从头开始安装` 步骤，举例的脚本适用于大部分 `Linux` 操作系统。

整个安装过程需要使用 `root` 操作系统用户来完成。

> `步骤1` - `步骤3` 均为软件环境安装步骤。如果你的操作已经具备当前步骤的软件或环境，此步骤可以跳过。

### 1. 准备操作系统环境

选择合适的 `Linux` 发行版本，推荐使用 `Debian 12` / `Ubuntu 22.04` 或 `CentOS 7.x` 以上版本。

```shell
# Debian / Ubuntu
apt-get update
apt-get install -y nginx git mysql-server mariadb-client sendmail

# CentOS / RHEL
yum install -y nginx git mariadb-server mariadb sendmail
```

### 2. 安装 Go

访问 [https://golang.google.cn/dl/](https://golang.google.cn/dl/) 下载 `1.25` 版本以上
（推荐 `1.25`）的二进制安装包，解压后复制到 `/usr/local/go` 目录下。

```shell
cp -R go /usr/local
ln -s /usr/local/go/bin/go /usr/local/bin/go
ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt
```

### 3. 安装 NodeJS（仅在需要重新构建前端时使用）

前端产物 `www/view/build` 已经随仓库提交，正常安装**不需要** Node.js。
只有要改动前端源码时才需要：

```shell
curl -fsSL https://nodejs.org/dist/v20.x.x/node-v20.x.x-linux-x64.tar.xz | tar -xJ -C /usr/local
ln -s /usr/local/node-v20.x.x-linux-x64/bin/node /usr/local/bin/node
ln -s /usr/local/node-v20.x.x-linux-x64/bin/npm  /usr/local/bin/npm
```

然后在 `www/view` 目录下执行 `yarn install && yarn build`。

### 4. 安装 MySQL / MariaDB

MySQL 不需要单独编译，使用系统包管理器安装即可。

> ⚠️ **需要 `MySQL 8.0+`（或 `MariaDB 10.6+`）**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED` 领取任务行，该语法自 MySQL 8.0.1 / MariaDB 10.6 起才支持。
> 在 MySQL 5.7 上这两条查询直接报语法错误，活动流、站内信、邮件通知与 webhook
> 会**静默失效**。

运行 `service mariadb start` 或 `service mysql start` 启动服务后，
使用 `mysql_secure_installation` 或 `mariadb-secure-installation` 初始化数据库。

* 当设置 `root` 密码为 `123456` 时，不需要在下一步中修改 `env.yaml` 中的数据库设置。

`MySQL 8.0` 的默认 `SQL MODE` 已经满足要求，无需额外修改。
如果你用的是 `MySQL 5.7`，需要手动修改 `SQL MODE` 变量，否则创建数据库时会报错：

```sql
set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION';
```

### 5. 下载源码

```shell
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next
```

### 6. 准备配置文件

```shell
cd /data/www/codefever-community
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0777 config.yaml env.yaml
```

`env.yaml` 里的 `session_key` 与 `totp_salt` 是占位符，需要替换成随机值：

```shell
KEY="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
```

按需修改 `env.yaml` 里的数据库连接信息（`host` / `port` / `username` / `password` / `db`）。

### 7. 编译 Go 服务

```shell
cd /data/www/codefever-community/backend

export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod

go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api    ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook   ./cmd/codefever-hook
```

编译两个 Git 网关。`http-gateway/` 与 `ssh-gateway/` 是上游保留的 GOPATH 风格代码
（相对导入 + `GO111MODULE=off`），现代 Go 已不支持该模式，因此构建期需要转换成 module 模式：

```shell
cd /data/www/codefever-community
export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod

printf 'module codefever\n\ngo 1.21\n\nrequire gopkg.in/yaml.v2 v2.4.0\n' > go.mod
sed -i 's#"\./Request"#"codefever/http-gateway/Request"#' http-gateway/main.go
sed -i 's#"\.\./JsonHttpClient"#"codefever/http-gateway/JsonHttpClient"#' http-gateway/Request/main.go
sed -i 's#"\.\./JsonHttpClient"#"codefever/ssh-gateway/JsonHttpClient"#' ssh-gateway/shell/main.go

cd http-gateway && go build -trimpath -ldflags="-s -w" -o main .
cd ../ssh-gateway/shell && go build -trimpath -ldflags="-s -w" -o main .
```

### 8. 恢复 git 钩子符号链接

git 钩子由 `bin/codefever-hook` 提供，裸仓库的 `hooks/` 目录通过软链指向它：

```shell
cd /data/www/codefever-community
mkdir -p git-storage/hooks
rm -f misc/hooks/main misc/hooks/post-receive misc/hooks/pre-receive misc/hooks/update
rm -f git-storage/hooks/main git-storage/hooks/post-receive git-storage/hooks/pre-receive git-storage/hooks/update

ln -s ../../bin/codefever-hook misc/hooks/main
ln -s main misc/hooks/post-receive
ln -s main misc/hooks/pre-receive
ln -s main misc/hooks/update

ln -s ../../bin/codefever-hook git-storage/hooks/main
ln -s main git-storage/hooks/post-receive
ln -s main git-storage/hooks/pre-receive
ln -s main git-storage/hooks/update
```

同时前端入口 `www/app` 必须指向 `www/view/build`：

```shell
rm -f www/app
ln -s view/build www/app
```

### 9. 初始化数据库

```shell
cd /data/www/codefever-community/misc
sh ./create_db.sh
```

如果你不需要对 CodeFever 进行代码上的修改，或者将来不计划运行单元测试，可以删除测试数据：

```shell
cd /data/www/codefever-community/misc
sh ./remove_test_data.sh
```

### 10. 配置 nginx

以 `misc/docker/vhost-go.conf-template` 为模板生成站点配置，
并把其中的 `root` 指向本机的应用目录：

```shell
mkdir -p /etc/nginx/conf.d
sed 's#/data/www/codefever-community#/data/www/codefever-community#' \
    /data/www/codefever-community/misc/docker/vhost-go.conf-template \
    > /etc/nginx/conf.d/codefever.conf

nginx -t && systemctl reload nginx
```

该配置包含三段规则：

- `/api/*`、`/apiv1/*`、`/user/*`、`/doc/*` 等应用层路径 → 反向代理到 `http://127.0.0.1:9001`（Go 后端）
- `/*.git` → 反向代理到 `http://127.0.0.1:27555`（Git over HTTP 网关）
- `/static/*` 与其余路径 → 直接由 nginx 托管前端静态产物

### 11. 配置 SSH 免密登录

Git over SSH 通过 `ssh-gateway/shell/main` 作为 forced command 执行：

```
# /etc/ssh/sshd_config
Match User git
    ForceCommand /data/www/codefever-community/ssh-gateway/shell/main
    AllowTcpForwarding no
    PermitTunnel no
```

### 12. 以 systemd 托管三个 Go 服务

`/etc/systemd/system/codefever-api.service`：

```ini
[Unit]
Description=CodeFever Next API (Go)
After=network.target mysqld.service

[Service]
Type=simple
User=root
WorkingDirectory=/data/www/codefever-community
Environment=CODEFEVER_BASE_DIR=/data/www/codefever-community
ExecStart=/data/www/codefever-community/bin/codefever-api
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

`/etc/systemd/system/codefever-worker.service`：把 `ExecStart` 换成

```
ExecStart=/data/www/codefever-community/bin/codefever-worker
```

`/etc/systemd/system/codefever-http-gateway.service`：把 `ExecStart` 换成

```
ExecStart=/data/www/codefever-community/http-gateway/main
```

启用并启动：

```shell
systemctl daemon-reload
systemctl enable --now codefever-api codefever-worker codefever-http-gateway
```

### 13. 开机自启

```shell
systemctl enable nginx mysqld mariadb sendmail
```

### 14. 验证

访问 `http://127.0.0.1` 或 `http://<server ip>` 来登录。

默认管理员用户：`root@codefever.cn`，密码：`123456`。**登录后请立即修改密码并绑定 MFA 设备。**

### 15. 服务维护

服务维护请参见 [管理员设置/概览和系统服务](../admin/dashboard.md) 中的 `系统服务状态及维护` 一节
