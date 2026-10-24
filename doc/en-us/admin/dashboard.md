# Overview and system services

This page has no management actions; it shows `system resource usage` and `service status`.

### Open the overview page

You need to be a CodeFever `administrator`.

1. Click `Admin` in the top-right navigation bar.
2. Click `Overview` in the left sidebar.

### Usage statistics

Shows CodeFever usage.

> CodeFever does not limit the number of repositories.

### System resources

Shows `system resource` usage.

> Check this panel when investigating performance problems.

### Disk usage

Shows `disk` usage.

> Git repositories live under `git-storage/` in the application root:
> - Docker Compose: `codefever-data/git-storage/` under the repository root;
> - source install: `/data/www/codefever-next/git-storage`.
>
> If disk space is low, expand the disk or move this directory to a larger disk and mount
> it back.

### Service status and maintenance

The backend is fully Go-based: **there is no PHP-FPM / php-fpm / crond**. The running
processes are:

| Component | Purpose | Managed by |
| --- | --- | --- |
| `nginx` | HTTP reverse proxy, static assets, Git HTTP entry | base image / system service |
| `codefever-api` | API, server-rendered pages, file downloads | supervisor (container) / systemd (source) |
| `codefever-worker` | events, inbox, e-mail, webhook delivery | supervisor / systemd |
| `codefever-http-gateway` | Git over HTTP gateway (`127.0.0.1:27555`) | supervisor / systemd |
| `codefever-ssh-gateway` | SSH forced command, started per SSH connection | not resident |
| `sshd` | Git over SSH entry | base image / system service |

> The admin overview `service` object only contains `nginx`, `codefever` (detected via the
> `codefever-api` process) and `sendmail`; PHP/crond keys no longer exist. `sendmail` now
> means “system mail channel”; SMTP is recommended.

#### Container deployment (Docker Compose)

```shell
# all resident processes
docker exec -it codefever-next-app supervisorctl status

# individual services
docker exec -it codefever-next-app supervisorctl restart codefever-api
docker exec -it codefever-next-app supervisorctl restart codefever-worker
docker exec -it codefever-next-app supervisorctl restart codefever-http-gateway

# nginx
docker exec -it codefever-next-app nginx -t
docker exec -it codefever-next-app nginx -s reload

# logs
docker compose logs -f codefever
```

#### Source installation (systemd)

```shell
systemctl status codefever-api codefever-worker codefever-http-gateway
systemctl restart codefever-api codefever-worker codefever-http-gateway

# nginx
nginx -t && systemctl reload nginx

# sshd (after changing Match/ForceCommand)
sshd -t && systemctl reload ssh
```

> `codefever-service-template` is only for the legacy VM HTTP-gateway init script. In the
> container, supervisor manages it; for a source install, prefer systemd directly.

#### E-mail (Sendmail / SMTP)

The image does **not** install an MTA, so the `sendmail`/`mail` fallback is normally
unavailable. Configure SMTP at deployment time:

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<password>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls
```

Without SMTP, the overview may show sendmail as stopped and e-mail notifications only log
failures in the worker; Git operations are unaffected.
