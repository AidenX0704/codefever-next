# Install with docker-compose

## Installation

### About the orchestration file

The `docker-compose.yml` at the repository root is the **only** orchestration file. It
defines two services:

| Service | Container | Description |
| --- | --- | --- |
| `codefever` | `codefever-next-app` | The application; the image is built from the current source (`Dockerfile`) |
| `db` | `codefever-next-db` | MySQL 8.0.x |

All data is persisted under **`codefever-data/`** in the repository root (bind mounts,
ignored by `.gitignore`):

| Host directory | Content |
| --- | --- |
| `codefever-data/mysql/` | MySQL data |
| `codefever-data/git-storage/` | bare repositories and hook symlinks |
| `codefever-data/file-storage/` | uploaded files/attachments |
| `codefever-data/config/` | `config.yaml` + `env.yaml` (host names, SMTP, gateway token) |

Docker creates the directories on first start; the entrypoint recreates the
`git-storage/hooks` and `deploy/hooks` symlinks, so no manual preparation is required.
Set `CODEFEVER_DATA_DIR=/absolute/path` to move the data directory.

### Get the code and start

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

The first build needs network access to Docker Hub (`webdevops/php-nginx:7.4`,
`golang:1.25-bookworm`, `mysql:8.0`), the Debian buster archive and a Go module proxy.
See “China / mirror setup” below.

> **Stuck on `load metadata ... docker.io` / `auth.docker.io`?** The base images are
> not local yet. Run `pwsh -File scripts/build-local.ps1 -SkipPackage` from the repo
> root (pulls through a mirror and tags locally), or set `CODEFEVER_BASE_*` as shown
> below. Local images are reused without contacting Docker Hub.

`--build` rebuilds from the current source; a plain `docker compose up -d` reuses an
existing image.

1. Open **<http://localhost:9080>** (or `http://<server ip>:9080`) and log in.
2. Default ports are `9080` (HTTP) and `2222` (Git SSH). Override them with:

   ```shell
   export CODEFEVER_HTTP_PORT=18080
   export CODEFEVER_SSH_PORT=18022
   docker compose up -d
   ```

   The entrypoint writes `host`/`ssh` from these ports, so the clone panel shows
   `http://localhost:18080/...` and `ssh://git@localhost:18022/...`. Behind a reverse proxy
   or on a domain, override explicitly:

   ```shell
   export CODEFEVER_SITE_HOST=https://git.example.com
   export CODEFEVER_SITE_SSH=ssh://git@git.example.com:2222/
   docker compose up -d
   ```

3. If something goes wrong, enter the container or run `docker compose restart codefever`.
4. Default administrator: `root@codefever.cn` / `123456`. **Change the password, bind MFA
   and disable open registration (`allowRegister` defaults to true) after the first login.**

### Configurable environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | Host port for HTTP |
| `CODEFEVER_SSH_PORT` | `2222` | Host port for Git SSH |
| `CODEFEVER_SITE_HOST` | derived from HTTP port | HTTP URL shown in the clone panel |
| `CODEFEVER_SITE_SSH` | derived from SSH port | SSH URL; must use `ssh://git@host:port/` |
| `CODEFEVER_DB_PASS` | `123456` | Database root password; special characters supported |
| `CODEFEVER_IMAGE` | `codefever-next:local` | Application image name |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | Database image (use **8.0.x**) |
| `CODEFEVER_BASE_RUNTIME` | `webdevops/php-nginx:7.4` | Runtime base image |
| `CODEFEVER_BASE_GO` | `golang:1.25-bookworm` | Go base image |
| `CODEFEVER_GOPROXY` | `https://proxy.golang.org,direct` | Go module proxy |
| `CODEFEVER_APT_MIRROR` | `https://archive.debian.org` | Debian buster archive |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | New password hash format: `legacy` or `argon2id` |

Database settings accept `CODEFEVER_MYSQL_HOST/PORT/USER/PASSWORD/DB` (preferred) and the
legacy `DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME` (compatibility). Compose passes both.

### E-mail (SMTP)

The image does not install an MTA; SMTP must be configured for notifications:

```shell
export CODEFEVER_SMTP_HOST=smtp.example.com
export CODEFEVER_SMTP_PORT=587
export CODEFEVER_SMTP_USER=git@example.com
export CODEFEVER_SMTP_PASSWORD='<password>'
export CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
export CODEFEVER_SMTP_TLS=starttls     # port 465 is implicit TLS
docker compose up -d
```

> Without SMTP, e-mail falls back to the system `mail` command, which is normally absent;
> failures are only logged by the worker and do not affect Git.

### China / mirror setup

```shell
export CODEFEVER_BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4
export CODEFEVER_BASE_GO=docker.1ms.run/library/golang:1.25-bookworm
export CODEFEVER_MYSQL_IMAGE=docker.1ms.run/library/mysql:8.0
export CODEFEVER_GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export CODEFEVER_APT_MIRROR=https://mirrors.aliyun.com/debian-archive
docker compose up -d --build
```

### Using an external database

If the database lives outside the container, comment out the `db` service **and remove the
`depends_on` block from `codefever`** (otherwise Compose reports a dependency on an
undefined service), then adjust the database variables:

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

The external database must be MySQL 8.0.x (or MariaDB 10.6+) and reachable from the app
container.

### Database version

The event worker and webhook deliverer use `FOR UPDATE SKIP LOCKED`, which only exists since
**MySQL 8.0.1**. On MySQL 5.7 the queries fail with a syntax error and the activity feed,
in-app notifications, e-mail notifications and webhooks fail silently (`SKIP LOCKED` errors
every two seconds in the logs).

Compose passes `--default-authentication-plugin=mysql_native_password` because MySQL 8.0's
default `caching_sha2_password` is not available in the bundled MariaDB client. That flag
was **removed in MySQL 8.4**, so use `mysql:8.0` / 8.0.x, not 8.4/9.x.

### Build your own image

```shell
docker compose build codefever
docker tag codefever-next:local you-name/codefever-next:latest
docker push you-name/codefever-next:latest
```

Then point `CODEFEVER_IMAGE` at your image.

### Service maintenance

```shell
docker compose ps                # status (services: codefever / db)
docker compose logs -f codefever # follow application logs
docker compose restart codefever # restart the app
docker compose down              # stop (keep the codefever-data directory)
docker compose down -v           # stop and remove containers/networks (codefever-data is untouched)
docker exec -it codefever-next-app sh
```

Resident processes are managed by supervisor; the admin overview shows nginx / codefever /
sendmail (SMTP). See [Admin / Overview and system services](../admin/dashboard.md).

## When this document applies

Use [Install From Scratch](install_from_scratch.md) instead if:

- you need to modify/debug Go sources or submit a PR;
- Docker is unavailable on your OS/architecture;
- you need deep nginx/systemd/database customization.
