# Install via Docker Image

> All images of this project are **built from local source** (`Dockerfile`).
> No upstream pre-built image is used any more.

## Installation

### About the image

The `CodeFever Next` image is based on `webdevops/php-nginx:7.4` — it only borrows the
`nginx` / `supervisor` / `sshd` / container-entrypoint integration from it. The **PHP runtime**
(interpreter, php-fpm, PHP configuration, PHP entrypoint scripts, crond) is **completely removed
at build time**, so the final image contains no PHP executable at all.

The application logic is produced by two Go multi-stage builds:

| Artifact | Source | Purpose | Listens on |
| --- | --- | --- | --- |
| `bin/codefever-api` | `backend/cmd/codefever-api` | Application backend (API / server-rendered pages / files) | `127.0.0.1:9001` |
| `bin/codefever-worker` | `backend/cmd/codefever-worker` | Events, notifications, e-mail, webhook delivery | — |
| `bin/codefever-hook` | `backend/cmd/codefever-hook` | git hooks (replaces the former PHP hook script) | — |
| `http-gateway/main` | `http-gateway/` | Git over HTTP gateway | `127.0.0.1:27555` |
| `ssh-gateway/shell/main` | `ssh-gateway/shell/` | Git over SSH forced command | — |

The front-end bundle `www/view/build` is committed to the repository, so the build needs
**neither Node.js nor access to npm / GitHub**.

### Build the image

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker build -f Dockerfile -t codefever-next:local .
```

Three base images are pulled during the build. If Docker Hub is not directly reachable,
use a registry mirror:

```shell
docker build -f Dockerfile \
    --build-arg REGISTRY=docker.1ms.run \
    -t codefever-next:local .
```

On Windows / PowerShell, `scripts/build-local.ps1` does "pull base images → build → export tar"
in one step:

```powershell
pwsh -File scripts/build-local.ps1
```

### Run

**Recommended**: use [docker-compose](install_via_docker_compose.md)

```shell
docker compose up -d --build
```

Or run the container directly:

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

> Set `DB_HOST` to the real address of your database. For an external database
> (e.g. a managed cloud instance) change `DB_HOST` and `DB_PORT` accordingly.

1. The first start initializes the database and may take 1-2 minutes.
   Watch the progress with `docker container logs codefever -f`; the initialization is done
   when `=== IMPORTANT NOTICE ===` shows up in the log.
2. Afterwards open `http://127.0.0.1` or `http://<server ip>` and log in.
3. If you want to use the host's port `22` for the Git SSH protocol, move the host SSH
   service to a different port first.
4. If something goes wrong you can enter the container shell for manual maintenance,
   or simply restart the container.
5. Default administrator: `root@codefever.cn`, password: `123456`.
   **Change the password and bind an MFA device right after the first login.**

### Offline distribution

```shell
docker save -o codefever-next.tar codefever-next:local
```

On the target machine:

```shell
docker load -i codefever-next.tar
docker compose up -d
```

### Service maintenance

See the `系统服务状态及维护` section of [Admin / Dashboard](../admin/dashboard.md).

## When this document applies

Use [Install From Scratch](install_from_scratch.md) instead if any of the following applies:

- Learning and technical exchange
- You need custom modifications
- `Docker 镜像安装` cannot satisfy your `Bug` fixing or `PR` submitting needs
- The Docker image cannot run on your `operating system` or `hardware architecture`
