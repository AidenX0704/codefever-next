# Install From Scratch

## When this document applies

> Use the `从零开始安装` (install from scratch) route if any of the following applies,
> otherwise use [Install via Docker Image](install_via_docker.md).

- Learning and technical exchange
- You need custom modifications
- `Docker 镜像安装` cannot satisfy your `Bug` fixing or `PR` submitting needs
- The Docker image cannot run on your `operating system` or `hardware architecture`

> **Version note**: the backend of this project has been rewritten from PHP in Go.
> **PHP, php-fpm, Composer and crond are no longer required.**
> The steps below describe the Go backend; the former PHP installation steps were removed.

## Installation

### 0. Before you start

These steps apply to most `Linux` distributions. The whole procedure has to be performed
as the `root` operating system user.

> Steps `1` - `3` only install the software environment. Skip them if your machine already
> provides it.

### 1. Prepare the operating system

Pick a suitable `Linux` distribution — `Debian 12` / `Ubuntu 22.04` or `CentOS 7.x` and newer
are recommended.

```shell
# Debian / Ubuntu
apt-get update
apt-get install -y nginx git mysql-server mariadb-client sendmail

# CentOS / RHEL
yum install -y nginx git mariadb-server mariadb sendmail
```

### 2. Install Go

Download the `1.25`+ binary release (recommended: `1.25`) from
[https://golang.google.cn/dl/](https://golang.google.cn/dl/), unpack it into `/usr/local/go`.

```shell
cp -R go /usr/local
ln -s /usr/local/go/bin/go /usr/local/bin/go
ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt
```

### 3. Install NodeJS (only when you need to rebuild the front-end)

The front-end bundle `www/view/build` is committed to the repository, so Node.js is **not**
required for a normal installation. You only need it when you change front-end sources:

```shell
curl -fsSL https://nodejs.org/dist/v20.x.x/node-v20.x.x-linux-x64.tar.xz | tar -xJ -C /usr/local
ln -s /usr/local/node-v20.x.x-linux-x64/bin/node /usr/local/bin/node
ln -s /usr/local/node-v20.x.x-linux-x64/bin/npm  /usr/local/bin/npm
```

Then run `yarn install && yarn build` inside `www/view`.

### 4. Install MySQL / MariaDB

MySQL does not have to be compiled — install it with the package manager of your distribution.

> ⚠️ **`MySQL 8.0+` (or `MariaDB 10.6+`) is required.** The event worker and the webhook
> deliverer claim their work with `FOR UPDATE SKIP LOCKED`, which only exists since
> MySQL 8.0.1 / MariaDB 10.6. On MySQL 5.7 those two queries fail with a syntax error,
> which silently kills the activity feed, in-app notifications, e-mail notifications
> and webhooks.

Start it with `service mariadb start` or `service mysql start`, then initialise it with
`mysql_secure_installation` or `mariadb-secure-installation`.

* When you set the `root` password to `123456`, you do not have to change the database
  settings in `env.yaml` in the next step.

The default `SQL MODE` of `MySQL 8.0` already satisfies the requirement. If you are on
`MySQL 5.7`, change the SQL MODE variable manually, otherwise creating the database fails:

```sql
set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION';
```

### 5. Get the source code

```shell
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next
```

### 6. Prepare the configuration files

```shell
cd /data/www/codefever-community
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0777 config.yaml env.yaml
```

`session_key` and `totp_salt` in `env.yaml` are placeholders — replace them with random values:

```shell
KEY="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
```

Adjust the database connection (`host` / `port` / `username` / `password` / `db`) in `env.yaml`
if needed.

### 7. Build the Go services

```shell
cd /data/www/codefever-community/backend

export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod

go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api    ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook   ./cmd/codefever-hook
```

Build the two Git gateways. `http-gateway/` and `ssh-gateway/` are upstream GOPATH-style code
(relative imports + `GO111MODULE=off`); modern Go no longer supports that mode, so the imports
are converted to module mode at build time:

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

### 8. Restore the git hook symlinks

The git hooks are provided by `bin/codefever-hook`; the `hooks/` directory of every bare
repository symlinks to it:

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

The front-end entry point `www/app` has to point at `www/view/build` as well:

```shell
rm -f www/app
ln -s view/build www/app
```

### 9. Initialise the database

```shell
cd /data/www/codefever-community/misc
sh ./create_db.sh
```

If you do not plan to modify CodeFever or to run the unit tests, the test data can be removed:

```shell
cd /data/www/codefever-community/misc
sh ./remove_test_data.sh
```

### 10. Configure nginx

Generate the site configuration from `misc/docker/vhost-go.conf-template` and make sure its
`root` points at the application directory on your machine:

```shell
mkdir -p /etc/nginx/conf.d
cp /data/www/codefever-community/misc/docker/vhost-go.conf-template \
   /etc/nginx/conf.d/codefever.conf

nginx -t && systemctl reload nginx
```

The configuration contains three parts:

- Application paths (`/api/*`, `/apiv1/*`, `/user/*`, `/doc/*`, …) → proxied to
  `http://127.0.0.1:9001` (the Go backend)
- `/*.git` → proxied to `http://127.0.0.1:27555` (the Git over HTTP gateway)
- `/static/*` and everything else → served directly by nginx from the front-end bundle

### 11. Configure passwordless SSH access

Git over SSH runs `ssh-gateway/shell/main` as a forced command:

```
# /etc/ssh/sshd_config
Match User git
    ForceCommand /data/www/codefever-community/ssh-gateway/shell/main
    AllowTcpForwarding no
    PermitTunnel no
```

### 12. Supervise the three Go services with systemd

`/etc/systemd/system/codefever-api.service`:

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

`/etc/systemd/system/codefever-worker.service`: same file with

```
ExecStart=/data/www/codefever-community/bin/codefever-worker
```

`/etc/systemd/system/codefever-http-gateway.service`: same file with

```
ExecStart=/data/www/codefever-community/http-gateway/main
```

Enable and start them:

```shell
systemctl daemon-reload
systemctl enable --now codefever-api codefever-worker codefever-http-gateway
```

### 13. Enable the services on boot

```shell
systemctl enable nginx mysqld mariadb sendmail
```

### 14. Verify

Open `http://127.0.0.1` or `http://<server ip>` and log in.

Default administrator: `root@codefever.cn`, password: `123456`.
**Change the password and bind an MFA device right after the first login.**

### 15. Service maintenance

See the `系统服务状态及维护` section of [Admin / Dashboard](../admin/dashboard.md).
