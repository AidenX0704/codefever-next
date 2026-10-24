# 从零开始安装

## 此文档适用条件

> 如果你的情况符合以下条件，可以使用 `从零开始安装`；否则推荐
> [使用 docker-compose 安装](install_via_docker_compose.md)（部署路径更短，升级也更简单）。

- 学习和技术交流
- 需要做定制化修改
- 需要直接编译/调试 Go 源码，或提交 PR
- Docker 镜像不能在当前操作系统或硬件架构上使用

> **版本说明**：后端已从 PHP 重写为 Go，`http-gateway` / `ssh-gateway` 也已并入
> `backend/`。**不再需要 PHP、php-fpm、Composer、crond**。

## 安装步骤

### 0. 安装前的说明

- 以下命令以 `root` 执行，适用于大多数 Linux 发行版。
- **源码目录**示例为 `/data/www/codefever-next`；容器内路径是
  `/data/www/codefever-community`，两者不同，不要混用。
- 源码安装的 HTTP/SSH 端口默认是 **80/22**（nginx/sshd），与 Compose 的
  9080/2222 不同。

### 1. 准备操作系统环境

推荐 `Debian 12` / `Ubuntu 22.04` 或更新版本。

```shell
# Debian / Ubuntu
apt-get update
apt-get install -y nginx git mysql-server mariadb-client ca-certificates curl

# 如需在服务器上用系统 mail 发通知，再额外安装 MTA（推荐改用 SMTP，见第 6 步）
# apt-get install -y mailutils
```

> `sendmail`/`mailutils` 只在你想使用系统 `mail` 回退时需要；CodeFever 更推荐
> 配置 SMTP（`CODEFEVER_SMTP_*`）。

### 2. 安装 Go

从 <https://golang.google.cn/dl/> 下载 `1.23` 或更新版本的 Linux 二进制包
（`backend/go.mod` 声明 `go 1.23.0`；Docker 构建使用 1.25 基础镜像，不影响源码安装）：

```shell
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
go version
```

### 3. 安装 Node.js（仅在需要重新构建前端时使用）

前端产物 `www/view/build` 已随仓库提交，正常安装**不需要** Node.js：

```shell
curl -fsSL https://nodejs.org/dist/v20.19.0/node-v20.19.0-linux-x64.tar.xz | tar -xJ -C /usr/local
ln -sf /usr/local/node-v20.19.0-linux-x64/bin/node /usr/local/bin/node
ln -sf /usr/local/node-v20.19.0-linux-x64/bin/npm  /usr/local/bin/npm
```

然后在 `www/view` 目录执行 `yarn install && yarn build`；Node 17+ 需要
`NODE_OPTIONS=--openssl-legacy-provider`。

### 4. 安装 MySQL / MariaDB

> ⚠️ **使用 MySQL 8.0.x（或 MariaDB 10.6+）**。事件 worker 与 webhook 投递器用
> `FOR UPDATE SKIP LOCKED`（MySQL 8.0.1 / MariaDB 10.6 起）。
> MySQL 5.7 会报语法错误；MySQL 8.4+ 的认证参数与 compose 里的旧参数不兼容，暂不推荐。

```shell
# Debian / Ubuntu，二选一
service mysql start
# 或
service mariadb start

mysql_secure_installation
# 或 mariadb-secure-installation
```

如果 root 口令设为 `123456`，可以不修改后面的 `env.yaml`；否则在第 6 步填写。

### 5. 下载源码

```shell
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next
```

以下所有命令都在 `/data/www/codefever-next` 下执行。

### 6. 准备配置文件

源码安装直接使用仓库根目录的 `config.yaml` / `env.yaml`（不是 `config/` 目录）：

```shell
cd /data/www/codefever-next
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0660 config.yaml env.yaml
```

`env.yaml` 里的 `session_key`、`totp_salt`、`gateway.token` 都是占位符，必须替换：

```shell
KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" env.yaml
```

- `gateway.token` 是内部网关回调令牌；**源码安装必须手填**，否则 `/apiv1/internal/*` 鉴权失败。
- 按需修改 `env.yaml` 的 `mysql` 段（`host` / `port` / `username` / `password` / `db`）。
- `config.yaml` 的 `host` / `ssh` 用于前端 clone 面板。源码安装默认 80/22 时可写
  `host: http://<服务器IP>`、`ssh: git@<服务器IP>:`；如果 SSH 不是 22 端口，
  必须写成 `ssh://git@<服务器IP>:<端口>/`。
- 公网部署前把 `allowRegister` 改为 `false`。

### 7. 编译 Go 服务

`backend/` 是单一 Go module，包含 API、worker、git 钩子、HTTP 网关、SSH 网关：

```shell
cd /data/www/codefever-next/backend

export GOPROXY=https://mirrors.aliyun.com/goproxy,direct   # 国内可选
export GOFLAGS=-mod=mod

go build ./...                                             # 编译检查（含 5 个命令）

mkdir -p ../bin
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api          ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker       ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook         ./cmd/codefever-hook
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-http-gateway ./cmd/codefever-http-gateway
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-ssh-gateway  ./cmd/codefever-ssh-gateway
```

> 不再需要给 `http-gateway/`、`ssh-gateway/` 单独建 `go.mod` 或用 `sed` 改 import；
> 它们已经是 `backend/cmd/` 下的普通命令。

### 8. 创建 git 用户、目录与软链

```shell
cd /data/www/codefever-next

# SSH 登录用户与公钥目录（authkeys 会把 UI 里添加的公钥写到这里）
useradd -rm -d /home/git -s /bin/bash git || true
mkdir -p /home/git/.ssh
chown -R git:git /home/git/.ssh

# git 钩子以 git 用户运行，需要能读 env.yaml（gateway.token）
chown root:git config.yaml env.yaml
chmod 0660 config.yaml env.yaml

# 运行时目录
mkdir -p bin git-storage file-storage deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell

# 新裸仓库的 hooks 指向 deploy/hooks；misc/hooks 与 ssh-gateway/shell 是旧部署兼容路径
for d in deploy/hooks misc/hooks git-storage/hooks; do
  ln -sfn ../../bin/codefever-hook "$d/main"
  ln -sfn main "$d/pre-receive"
  ln -sfn main "$d/post-receive"
  ln -sfn main "$d/update"
done
ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main

# 前端入口
ln -sfn view/build www/app
```

### 9. 初始化数据库

脚本默认读取仓库根目录的 `env.yaml`，可重复执行（幂等）：

```shell
cd /data/www/codefever-next
sh backend/deploy/sql/create_db.sh
```

如果不需要测试数据（通常生产不需要）：

```shell
sh backend/deploy/sql/remove_test_data.sh
```

### 10. 配置 nginx

模板在 `backend/deploy/docker/vhost-go.conf-template`，其中 root 写的是容器路径
`/data/www/codefever-community`，源码安装需要替换为本机路径：

```shell
mkdir -p /etc/nginx/conf.d
sed 's#/data/www/codefever-community#/data/www/codefever-next#g' \
    /data/www/codefever-next/backend/deploy/docker/vhost-go.conf-template \
    > /etc/nginx/conf.d/codefever.conf

# 如有发行版自带的 default site 监听 80，先禁用再 reload
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

该配置包含：

- `/api/*`、`/apiv1/*`、`/user/*`、`/doc/*` 等 → `http://127.0.0.1:9001`（Go API）
- `/*.git` → `http://127.0.0.1:27555`（HTTP 网关）
- `/static/*` 与其余路径 → `www/view/build` 静态产物

### 11. 配置 SSH 免密登录

Git over SSH 由 `bin/codefever-ssh-gateway` 作为 forced command 执行：

```sshconfig
# /etc/ssh/sshd_config 末尾
Match User git
    ForceCommand /data/www/codefever-next/bin/codefever-ssh-gateway
    AllowTcpForwarding no
    PermitTunnel no
```

```shell
sshd -t && systemctl reload ssh
```

然后在 Web UI 的“SSH Key”页面添加公钥，再用
`git clone ssh://git@<服务器IP>:22/<group>/<repo>.git` 测试。

### 12. 以 systemd 托管三个常驻服务

`ssh-gateway` 不是常驻进程，由 sshd 每次连接拉起；常驻的是 API、worker、HTTP 网关。

`/etc/systemd/system/codefever-api.service`：

```ini
[Unit]
Description=CodeFever Next API (Go)
After=network.target mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/data/www/codefever-next
Environment=CODEFEVER_BASE_DIR=/data/www/codefever-next
# 邮件建议使用 SMTP（按需填写）
# Environment=CODEFEVER_SMTP_HOST=smtp.example.com
# Environment=CODEFEVER_SMTP_PORT=587
# Environment=CODEFEVER_SMTP_USER=git@example.com
# Environment=CODEFEVER_SMTP_PASSWORD='<口令>'
# Environment=CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
# Environment=CODEFEVER_SMTP_TLS=starttls
ExecStart=/data/www/codefever-next/bin/codefever-api -base /data/www/codefever-next
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

`/etc/systemd/system/codefever-worker.service` 与
`/etc/systemd/system/codefever-http-gateway.service` 复制上面的 `[Unit]/[Service]/[Install]`，
只把描述和 `ExecStart` 换成：

```ini
ExecStart=/data/www/codefever-next/bin/codefever-worker -base /data/www/codefever-next
ExecStart=/data/www/codefever-next/bin/codefever-http-gateway -base /data/www/codefever-next
```

启用并启动：

```shell
systemctl daemon-reload
systemctl enable --now codefever-api codefever-worker codefever-http-gateway
```

> 提供的 unit 以 `User=root` 运行，与容器行为一致；如改为 `git` 用户，需要确保
> `git-storage`、`file-storage`、`bin/`、`config.yaml`/`env.yaml` 的读写权限正确。

### 13. 开机自启

```shell
systemctl enable nginx
systemctl enable mysql     # 或 mariadb
```

### 14. 验证

访问 `http://<服务器IP>`（源码安装默认 80）登录。

默认管理员：`root@codefever.cn`，密码：`123456`。
**登录后请立即修改密码、绑定 MFA，并在设置中关闭开放注册。**

### 15. 服务维护

服务维护见 [管理员设置/概览和系统服务](../admin/dashboard.md)。
源码安装的日常操作：

```shell
systemctl status codefever-api codefever-worker codefever-http-gateway
journalctl -u codefever-api -f
nginx -t && systemctl reload nginx
```
