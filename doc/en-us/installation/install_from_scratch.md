# Install From Scratch

## When this document applies

Use the source installation if:

- you want to study or customize the code;
- you need to build/debug Go sources directly or submit a PR;
- Docker is not available on your OS/architecture;
- you need deep control over nginx/systemd/database.

Otherwise, use [Install with docker-compose](install_via_docker_compose.md); it is shorter
and easier to upgrade.

> **Version note**: the backend has been rewritten in Go. `http-gateway` and `ssh-gateway`
> are now part of `backend/`. **PHP, php-fpm, Composer and crond are no longer required.**

## Installation

### 0. Before you start

- Run the commands below as `root`.
- The **source path** used here is `/data/www/codefever-next`; the in-container path is
  `/data/www/codefever-community`. They are different; do not mix them.
- A source install serves HTTP/SSH on **80/22** by default (nginx/sshd), unlike the Compose
  defaults of 9080/2222.

### 1. Prepare the OS

`Debian 12` / `Ubuntu 22.04` or newer is recommended.

```shell
# Debian / Ubuntu
apt-get update
apt-get install -y nginx git mysql-server mariadb-client ca-certificates curl

# Optional: only if you want the system-mail fallback instead of SMTP
# apt-get install -y mailutils
```

> CodeFever recommends SMTP (`CODEFEVER_SMTP_*`) over the system `mail` fallback.

### 2. Install Go

Download a `1.23`+ Linux binary from <https://golang.google.cn/dl/>
(`backend/go.mod` declares `go 1.23.0`; the Docker build uses a 1.25 base image, which does
not affect a source install):

```shell
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
go version
```

### 3. Install Node.js (only when rebuilding the front-end)

The front-end bundle `www/view/build` is committed, so Node.js is **not** needed for a
normal installation:

```shell
curl -fsSL https://nodejs.org/dist/v20.19.0/node-v20.19.0-linux-x64.tar.xz | tar -xJ -C /usr/local
ln -sf /usr/local/node-v20.19.0-linux-x64/bin/node /usr/local/bin/node
ln -sf /usr/local/node-v20.19.0-linux-x64/bin/npm  /usr/local/bin/npm
```

Then run `yarn install && yarn build` in `www/view`. Node 17+ needs
`NODE_OPTIONS=--openssl-legacy-provider`.

### 4. Install MySQL / MariaDB

> ⚠️ **Use MySQL 8.0.x (or MariaDB 10.6+).** The event worker and webhook deliverer use
> `FOR UPDATE SKIP LOCKED` (MySQL 8.0.1 / MariaDB 10.6+).
> MySQL 5.7 fails with a syntax error; MySQL 8.4+ changes authentication options and is not
> yet supported by the provided setup.

```shell
service mysql start     # or: service mariadb start
mysql_secure_installation
# or: mariadb-secure-installation
```

If the root password is `123456`, you can keep the `env.yaml` defaults; otherwise update
the `mysql` section in step 6.

### 5. Get the source code

```shell
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next
```

All following commands run in `/data/www/codefever-next`.

### 6. Prepare the configuration files

A source install uses the repository-root `config.yaml` / `env.yaml` (not `config/`):

```shell
cd /data/www/codefever-next
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0660 config.yaml env.yaml
```

`session_key`, `totp_salt` and `gateway.token` in `env.yaml` are placeholders and must be
replaced:

```shell
KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" env.yaml
```

- `gateway.token` authenticates the internal gateway callbacks; a source install **must**
  fill it in.
- Adjust the `mysql` section in `env.yaml` (`host` / `port` / `username` / `password` / `db`).
- `config.yaml` `host` / `ssh` controls the clone panel. For a default 80/22 source install
  use `host: http://<server-ip>` and `ssh: git@<server-ip>:`. If SSH is not on port 22, use
  `ssh://git@<server-ip>:<port>/`.
- Set `allowRegister: false` before exposing the instance publicly.

### 7. Build the Go services

`backend/` is a single Go module containing the API, worker, git hook, HTTP gateway and SSH
gateway:

```shell
cd /data/www/codefever-next/backend

export GOPROXY=https://mirrors.aliyun.com/goproxy,direct   # optional in China
export GOFLAGS=-mod=mod

go build ./...                                             # compile check (5 commands)

mkdir -p ../bin
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api          ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker       ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook         ./cmd/codefever-hook
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-http-gateway ./cmd/codefever-http-gateway
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-ssh-gateway  ./cmd/codefever-ssh-gateway
```

> You no longer need per-gateway `go.mod` files or `sed` on relative imports; they are
> ordinary commands under `backend/cmd/`.

### 8. Create the git user, directories and symlinks

```shell
cd /data/www/codefever-next

# SSH user and public-key directory (the UI writes authorized_keys here)
useradd -rm -d /home/git -s /bin/bash git || true
mkdir -p /home/git/.ssh
chown -R git:git /home/git/.ssh

# git hooks run as the git user and need to read env.yaml (gateway.token)
chown root:git config.yaml env.yaml
chmod 0660 config.yaml env.yaml

# runtime directories
mkdir -p bin git-storage file-storage deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell

# new bare repos use deploy/hooks; misc/hooks and ssh-gateway/shell keep old deployments working
for d in deploy/hooks misc/hooks git-storage/hooks; do
  ln -sfn ../../bin/codefever-hook "$d/main"
  ln -sfn main "$d/pre-receive"
  ln -sfn main "$d/post-receive"
  ln -sfn main "$d/update"
done
ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main

# front-end entry
ln -sfn view/build www/app
```

### 9. Initialise the database

The script reads the repository-root `env.yaml` and is idempotent:

```shell
cd /data/www/codefever-next
sh backend/deploy/sql/create_db.sh
```

If you do not want the test data (usual for production):

```shell
sh backend/deploy/sql/remove_test_data.sh
```

### 10. Configure nginx

The template is `backend/deploy/docker/vhost-go.conf-template`; its root uses the container
path `/data/www/codefever-community`, so replace it with your source path:

```shell
mkdir -p /etc/nginx/conf.d
sed 's#/data/www/codefever-community#/data/www/codefever-next#g' \
    /data/www/codefever-next/backend/deploy/docker/vhost-go.conf-template \
    > /etc/nginx/conf.d/codefever.conf

# disable the distro's default site on :80 if present
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

The config contains:

- `/api/*`, `/apiv1/*`, `/user/*`, `/doc/*` → `http://127.0.0.1:9001` (Go API)
- `/*.git` → `http://127.0.0.1:27555` (HTTP gateway)
- `/static/*` and everything else → `www/view/build`

### 11. Configure passwordless SSH access

Git over SSH runs `bin/codefever-ssh-gateway` as a forced command:

```sshconfig
# end of /etc/ssh/sshd_config
Match User git
    ForceCommand /data/www/codefever-next/bin/codefever-ssh-gateway
    AllowTcpForwarding no
    PermitTunnel no
```

```shell
sshd -t && systemctl reload ssh
```

Then add a public key in **SSH Key** in the UI and test with
`git clone ssh://git@<server-ip>:22/<group>/<repo>.git`.

### 12. Supervise the three resident services with systemd

`ssh-gateway` is not resident; sshd starts it per connection. The resident services are API,
worker and HTTP gateway.

`/etc/systemd/system/codefever-api.service`:

```ini
[Unit]
Description=CodeFever Next API (Go)
After=network.target mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/data/www/codefever-next
Environment=CODEFEVER_BASE_DIR=/data/www/codefever-next
# E-mail is recommended via SMTP:
# Environment=CODEFEVER_SMTP_HOST=smtp.example.com
# Environment=CODEFEVER_SMTP_PORT=587
# Environment=CODEFEVER_SMTP_USER=git@example.com
# Environment=CODEFEVER_SMTP_PASSWORD='<password>'
# Environment=CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
# Environment=CODEFEVER_SMTP_TLS=starttls
ExecStart=/data/www/codefever-next/bin/codefever-api -base /data/www/codefever-next
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

Create `codefever-worker.service` and `codefever-http-gateway.service` with the same
`[Unit]/[Service]/[Install]`, changing only the description and `ExecStart`:

```ini
ExecStart=/data/www/codefever-next/bin/codefever-worker -base /data/www/codefever-next
ExecStart=/data/www/codefever-next/bin/codefever-http-gateway -base /data/www/codefever-next
```

Enable and start:

```shell
systemctl daemon-reload
systemctl enable --now codefever-api codefever-worker codefever-http-gateway
```

> The provided units run as `root`, matching the container. If you switch to `git`, make
> sure `git-storage`, `file-storage`, `bin/`, `config.yaml` and `env.yaml` permissions are
> correct.

### 13. Enable services on boot

```shell
systemctl enable nginx
systemctl enable mysql     # or mariadb
```

### 14. Verify

Open `http://<server-ip>` (port 80 for a source install) and log in.

Default administrator: `root@codefever.cn`, password: `123456`.
**Change the password, bind MFA and disable open registration right after the first login.**

### 15. Service maintenance

See [Admin / Overview and system services](../admin/dashboard.md). Common commands:

```shell
systemctl status codefever-api codefever-worker codefever-http-gateway
journalctl -u codefever-api -f
nginx -t && systemctl reload nginx
```
