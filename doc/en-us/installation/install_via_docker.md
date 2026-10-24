# Install via Docker Image

> The **application image** is built from local source (`Dockerfile`); the build still pulls
> two base images (`webdevops/php-nginx:7.4` and `golang:1.25-bookworm`). The PHP runtime is
> deleted at the end of the build, so the final image contains no PHP executable.

## Installation

### About the image

The image is based on `webdevops/php-nginx:7.4` and borrows only the
`nginx` / `supervisor` / `sshd` / container-entrypoint integration. The application is
provided by five binaries built from a **single Go module**:

| Artifact | Source | Purpose | Listens on |
| --- | --- | --- | --- |
| `bin/codefever-api` | `backend/cmd/codefever-api` | API / server-rendered pages / files | `127.0.0.1:9001` |
| `bin/codefever-worker` | `backend/cmd/codefever-worker` | events, inbox, e-mail, webhook delivery | — |
| `bin/codefever-hook` | `backend/cmd/codefever-hook` | git hooks | — |
| `bin/codefever-http-gateway` | `backend/cmd/codefever-http-gateway` | Git over HTTP gateway | `127.0.0.1:27555` |
| `bin/codefever-ssh-gateway` | `backend/cmd/codefever-ssh-gateway` | Git over SSH forced command | — |

The image also contains `doc/` (in-app documentation) and `LICENSE`. The front-end bundle
`www/view/build` is committed, so the build does not need Node.js, npm or GitHub.

### Build the image

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker build -f Dockerfile -t codefever-next:local .
```

The build needs Docker Hub (two base images), the Debian buster archive and a Go module
proxy. For China networks:

```shell
docker build -f Dockerfile \
    --build-arg BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4 \
    --build-arg BASE_GO=docker.1ms.run/library/golang:1.25-bookworm \
    --build-arg APT_MIRROR=https://mirrors.aliyun.com/debian-archive \
    --build-arg GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
    -t codefever-next:local .
```

`--build-arg REGISTRY=docker.1ms.run` changes both base-image prefixes. On Windows /
PowerShell:

```powershell
pwsh -File scripts/build-local.ps1
```

### Run

**Use [docker-compose](install_via_docker_compose.md)**:

```shell
docker compose up -d --build
```

Compose starts MySQL 8.0, creates the database, and exposes the app on 9080/2222.

A standalone `docker run` is only valid when you already have an external **MySQL 8.0.x**.
In that case, pass the DB address, mount `codefever-data/` directories, and publish the
ports; otherwise the container will fail to reach `db`:

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
  -e CODEFEVER_MYSQL_PASSWORD='<password>' \
  -e CODEFEVER_MYSQL_DB=codefever_community \
  -e CODEFEVER_HTTP_PORT=9080 \
  -e CODEFEVER_SSH_PORT=2222 \
  codefever-next:local
```

> This example does **not** start a MySQL container. Without an external MySQL 8.0.x, use
> Compose. Do not copy the old “app container only” command with `DB_HOST=db`; there is no
> `db` container in that setup.

1. First start initialises the database; `docker logs -f codefever` shows
   `=== 初始化完成 ===` when done.
2. Open <http://localhost:9080> and log in.
3. Default administrator: `root@codefever.cn` / `123456`. **Change the password, bind MFA
   and disable open registration right after the first login.**
4. E-mail requires `CODEFEVER_SMTP_*`; the image does not install an MTA.

### Offline distribution

```shell
docker save -o codefever-next.tar codefever-next:local
```

The target machine also needs a MySQL 8.0.x image when using Compose:

```shell
docker load -i codefever-next.tar
docker load -i mysql-8.0.tar
docker compose up -d
```

### Service maintenance

See [Admin / Overview and system services](../admin/dashboard.md#service-status-and-maintenance).
Resident processes are managed by supervisor:

```shell
docker exec -it codefever-next-app sh
supervisorctl status
```

## When this document applies

Use [Install From Scratch](install_from_scratch.md) instead if:

- you need to modify/debug Go sources or submit a PR;
- Docker is unavailable on your OS/architecture;
- you need deep nginx/systemd/database customization.
