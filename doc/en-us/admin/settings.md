# Settings

The admin **Settings** page controls global options: **host URLs (HTTP/SSH)**,
**sender name/address** and **open registration**.

Open it from **Admin → Settings** in the left sidebar.

### Host

The host settings affect:

- the HTTP/SSH repository URLs shown in the repository clone panel;
- the repository URL hint on an empty repository page.

| Field | Example | Notes |
| --- | --- | --- |
| URL prefix (`host`) | `https://git.example.com` | HTTP scheme + domain/IP; do not end with `/` |
| SSH prefix (`ssh`) | `ssh://git@git.example.com:2222/` | Use the `ssh://` form to carry a port; keep the trailing `/` |

> The container entrypoint generates defaults from `CODEFEVER_HTTP_PORT` /
> `CODEFEVER_SSH_PORT`. Behind a reverse proxy or on a domain, prefer
> `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH`; changes made here are written to
> `config.yaml` and persist in `codefever-data/config/` under the repository root.

Steps:

1. Open **Admin → Settings**.
2. Edit the URL / SSH prefixes under **Host**.
3. Click **Save**.
4. Refresh a repository page and check the clone panel.

### E-mail / SMTP

The **Sender name** and **Sender address** fields in the Settings page only set the From
display name/address. The SMTP server, port, credentials and TLS mode are configured with
environment variables (not on this page):

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<password>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls     # port 465 is treated as implicit TLS
```

> The image does not install an MTA. Without `CODEFEVER_SMTP_HOST`, e-mail falls back to
> the system `mail` command, which is normally unavailable; failures are only logged by
> the worker.

To avoid spam classification, configure SPF for the sender domain and PTR if needed:

```plain
<sender-domain>        TXT   v=spf1 ip4:<server IP> ~all
<server IP>            PTR   <sender-domain>
```

### Registration

**Open registration** controls whether new users can sign up.

> For public deployments, **turning open registration off is strongly recommended**.
> `config.template.yaml` ships with `allowRegister: true`; disable it in this page after the
> first login.

Steps:

1. Open **Admin → Settings**.
2. Find **Registration**.
3. Turn off **Open registration** and save.
