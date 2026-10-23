# Install with docker-compose

## Installation

### About the orchestration file

The `docker-compose.yml` at the repository root is the **only** orchestration file of this
project. It defines two services:

| Service | Description |
| --- | --- |
| `codefever` | The application itself; the image is **built from local source** (`Dockerfile`), no upstream pre-built image is used |
| `db` | MySQL 8.0, kept as a separate service so that you can back up the database on its own or point the app at an external database |

All data lives in **named volumes** (`db-data` / `git-storage` / `file-storage`).
Named volumes are used instead of bind mounts on purpose: a named volume is initialised from
the image content on first use, which is what keeps the `git-storage/hooks/*` symlinks intact.
A bind mount would shadow that directory with the host's copy and the git hooks would stop working.

### Get the code and start

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

`--build` rebuilds the image from the current source. If the image already exists,
a plain `docker compose up -d` reuses it.

1. Once the service is up, open `http://127.0.0.1` or `http://<server ip>` and log in.
2. The default ports are `9080` (HTTP) and `2222` (Git SSH). 80 / 22 / 8080 are commonly
   taken, hence the different defaults. Override them with environment variables:

   ```shell
   export CODEFEVER_HTTP_PORT=18080
   export CODEFEVER_SSH_PORT=18022
   docker compose up -d
   ```

3. If something goes wrong, enter the container shell for manual maintenance or run
   `docker compose restart codefever`.
4. Default administrator: `root@codefever.cn`, password: `123456`.
   **Change the password and bind an MFA device right after the first login.**

### Configurable environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | Host port for HTTP |
| `CODEFEVER_SSH_PORT` | `2222` | Host port for Git SSH |
| `CODEFEVER_DB_PASS` | `123456` | Database root password (shared by `MYSQL_ROOT_PASSWORD` and `DB_PASS` so they always match) |
| `CODEFEVER_IMAGE` | `codefever-next:local` | Application image name |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | Database image (**8.0+ is required**) |
| `CODEFEVER_BASE_RUNTIME` | `docker.1ms.run/webdevops/php-nginx:7.4` | Runtime base image used at build time |
| `CODEFEVER_BASE_GO` | `docker.1ms.run/library/golang:1.25-bookworm` | Go base image used at build time |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | Password hash format for new accounts: `legacy` (`md5(md5(x))`) or `argon2id` |

> **About `CODEFEVER_PASSWORD_HASH`**: existing accounts still carry the historical `legacy`
> hashes, so `legacy` is the default — **existing accounts keep working without any password
> reset**. Switch it to `argon2id` (the memory-hard function recommended by OWASP) once every
> account has had its password reset.

### Using an external database

If the database lives outside the container (a managed cloud database for instance),
comment out the `db` service and adjust the database environment variables of `codefever`:

```yaml
  codefever:
    environment:
      DB_HOST: "your-db-host"
      DB_PORT: "3306"
      DB_USER: "root"
      DB_PASS: "your-password"
      DB_NAME: "codefever_community"
```

### Note: the database must be MySQL 8.0+

The event worker and the webhook deliverer claim their work with `FOR UPDATE SKIP LOCKED`,
which only exists since **MySQL 8.0.1**. On MySQL 5.7 those two queries fail with a
syntax error, which silently kills the activity feed, in-app notifications, e-mail
notifications and webhooks — the symptom is a `SKIP LOCKED` error in the logs every two
seconds. The compose file already defaults to `mysql:8.0`.

The compose file also passes `--default-authentication-plugin=mysql_native_password`:
MySQL 8's default plugin `caching_sha2_password` is not part of the MariaDB client
bundled in the application container, and without this flag `misc/create_db.sh` fails to
create the database.

The default `SQL MODE` of `MySQL 8.0` already satisfies the requirement. If you insist on
`MySQL 5.7`, change the SQL MODE variable manually, otherwise creating the database fails:

```sql
set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION';
```

### Build your own image

The orchestration file already carries a `build` section pointing at `Dockerfile`
in the repository root:

```shell
docker compose build codefever
```

To push it to your own registry:

```shell
docker tag codefever-next:local you-name/codefever-next:latest
docker push you-name/codefever-next:latest
```

Then point `CODEFEVER_IMAGE` at your image name.

### Service maintenance

```shell
docker compose ps                # status
docker compose logs -f codefever # follow logs
docker compose restart codefever # restart the app
docker compose down              # stop (keep the volumes)
docker compose down -v           # stop and delete the volumes (all data is lost)
```

See the `系统服务状态及维护` section of [Admin / Dashboard](../admin/dashboard.md).

## When this document applies

Use [Install From Scratch](install_from_scratch.md) instead if any of the following applies:

- Learning and technical exchange
- You need custom modifications
- `Docker 镜像安装` cannot satisfy your `Bug` fixing or `PR` submitting needs
- The Docker image cannot run on your `operating system` or `hardware architecture`
