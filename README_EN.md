<div align="center">
  <br />
  <img src="www/static/images/logo-community.png" alt="CodeFever" height="42" />
  <br /><br />
  <p>
    <b>
      CodeFever Next — A fully open-source Git hosting platform · the backend has been rewritten in Go
    </b>
  </p>
  <p>

[![Contributions welcome](https://img.shields.io/badge/Contributions-welcome-brightgreen?logo=github)](CONTRIBUTING.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)](backend/)
[![PHP](https://img.shields.io/badge/PHP-removed-8892B0?logo=php&logoColor=white)](docs/PHP_REMOVAL.md)
[![Docker](https://img.shields.io/badge/Docker-supported-2496ED?logo=docker&logoColor=white)](doc/en-us/installation/install_via_docker_compose.md)

  </p>
  <br />
</div>

[中文说明](README.md) · [Build & deploy notes](LOCAL_DEPLOY.md) · [Documentation](doc/en-us/index.md) ·
[Contributing](CONTRIBUTING.md) · [Security policy](SECURITY.md) · [Changelog](CHANGELOG.md)

---

## ⚠️ Project notice

> **CodeFever Next is a Git hosting platform developed as a second-generation fork of [PGYER/codefever](https://github.com/PGYER/codefever).**
>
> **This is an unofficial derivative project, independently maintained by [GiserXiao](https://github.com/AidenX0704), and is not affiliated with the original PGYER team.**
>
> **The original CodeFever project is copyrighted by 蒲公英开发者服务平台 (PGYER) and was open sourced under the MIT license. This repository retains the original LICENSE and copyright notice.**

Upstream project: <https://github.com/PGYER/codefever> ｜ Original website: <https://codefever.cn>

---

## What is this

`CodeFever Next` is a lightweight, self-hosted Git hosting service: repositories,
branches, tags, merge requests, branch protection, webhooks, team collaboration and role
management — with a Chinese and English interface.

**The biggest difference compared to the upstream version is that the backend has been
completely rewritten from PHP in Go** — the runtime image no longer contains any PHP
process, PHP executable or PHP configuration. `http-gateway` and `ssh-gateway` have also
been merged into the same Go module (`backend/`).

| Area | Status |
| --- | --- |
| Backend | Go (`backend/`), replacing the former CodeIgniter 2.1.1 + PHP 7.4 implementation |
| API contract | 108 `/api/*` endpoints + 4 `/apiv1/internal` callbacks; **123/123 is a historical acceptance record**, see [implementation notes](docs/IMPLEMENTATION_STATUS.md) for how to re-run |
| Web routes | 43 server-rendered routes + 5 pages; **43/43 is a historical comparison record** (one brand string is a deliberate change) |
| Front-end | React 17 + MUI v4; the bundle `www/view/build` is committed, so the build does not need Node.js |
| Runtime | **No PHP, no php-fpm, no crond** in the image |
| Images | The application image is built from local source; two base images still have to be pulled (see *Quick start*) |
| Front-end size | `www/static/vendor` is about 3.7 MB / 27 files (historical comparison: 23.2 MB / 2072 files) |

> The full dependency surface, stability gate and historical verification data are
> documented in [`docs/PHP_REMOVAL.md`](docs/PHP_REMOVAL.md). The image contains `doc/` (the
> in-app documentation centre) and `LICENSE`; `docs/` is rewrite history and is not shipped.
> The contract numbers above are a snapshot of one acceptance run, not a guarantee for the
> current environment; re-running them requires a working instance and database.

## Features

- 🆓 **Free:** No paywall, no crippled features
- ❤️ **Open source:** MIT licensed, no compiled or encrypted code
- 🥬 **Lightweight:** `docker compose up -d --build` is enough to deploy
- ⚡️ **Efficient:** Runs on 1 vCPU / 1 GB RAM; historical benchmark p99 ≈ 178 ms at 64 concurrent clients
- ♾️ **Unlimited:** No limit on the number of repositories
- ⌨️ **Code review:** Highlighted diff between versions
- 🛤️ **Branches:** Full support for native Git branches and tags
- 🙋 **Merge requests:** Team collaboration and review
- 🙅 **Branch protection:** Keeps commits safe and reviewable
- 👥 **Collaboration:** Roles and permissions per member
- 🔌 **Webhook:** Integrates with Jenkins / Jira and friends
- 🖥️ **Admin portal:** Super administrators manage all projects and users
- 💬 **Bilingual UI:** Chinese and English UI (English translations of some user docs are still in progress)
- 🔑 **SSH push:** Clone / pull / push over SSH public-key authentication

## Quick start

Requirements:

- Docker 20.10+ and Docker Compose v2;
- **Network access for the first build**: pull `webdevops/php-nginx:7.4`,
  `golang:1.25-bookworm` and `mysql:8.0`, and reach the Debian buster archive and a Go
  module proxy;
- About 10 GB of free disk space for images, database and repositories.

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

> **First build stuck on `load metadata ... docker.io` / `auth.docker.io`?**
> The base images are not on this machine yet and Docker is reaching Docker Hub.
> On restricted networks run `pwsh -File scripts/build-local.ps1 -SkipPackage`
> (pulls through a mirror and tags locally), or set `CODEFEVER_BASE_*` as described
> in *Image sources* below. When the images are present locally, Compose reuses them
> and does not contact Docker Hub.

Once it is up:

| Item | Value |
| --- | --- |
| Web UI | <http://localhost:9080> |
| Default account | `root@codefever.cn` |
| Default password | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh://git@localhost:2222/<group>/<repo>.git` |

> **Change the password, bind an MFA device and disable open registration right after the
> first login.** `allowRegister` defaults to `true`; turn it off on any public instance.

Common commands (the Compose service is `codefever`; `codefever-next-app` is the container
name and is only used with `docker exec`):

```shell
docker compose ps                          # status
docker compose logs -f codefever           # follow application logs
docker compose restart codefever           # restart the app
docker compose down                        # stop (keep codefever-data)
docker compose down -v                     # remove containers/networks (codefever-data stays)
docker exec -it codefever-next-app sh     # open a shell in the app container
```

### Ports and public URLs

The container's `80/22` are mapped to the host's `9080/2222` by default:

```shell
CODEFEVER_HTTP_PORT=18080 CODEFEVER_SSH_PORT=18022 docker compose up -d
```

The container entrypoint writes `host`/`ssh` from these ports, so the clone panel shows the
correct URLs. Behind a reverse proxy, on a domain or on non-default ports, override them:

```shell
CODEFEVER_SITE_HOST=https://git.example.com \
CODEFEVER_SITE_SSH=ssh://git@git.example.com:2222/ \
docker compose up -d
```

> The `ssh` value must use `ssh://git@host:port/` to carry a port; the scp-like
> `git@host:path` form cannot express a non-22 port. You can also change both values in
> *Admin → Settings → Host*; they are persisted in `codefever-data/config/config.yaml`.

### China / mirror setup

Compose defaults to Docker Hub, `proxy.golang.org` and the official Debian archive. For
networks in China, override them:

```shell
CODEFEVER_BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4 \
CODEFEVER_BASE_GO=docker.1ms.run/library/golang:1.25-bookworm \
CODEFEVER_MYSQL_IMAGE=docker.1ms.run/library/mysql:8.0 \
CODEFEVER_GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
CODEFEVER_APT_MIRROR=https://mirrors.aliyun.com/debian-archive \
docker compose up -d --build
```

> A fully offline installation needs `docker save` on a connected machine and
> `docker load` on the target; see [LOCAL_DEPLOY.md](LOCAL_DEPLOY.md).

## Configuration

### Orchestration environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | Host port for HTTP |
| `CODEFEVER_SSH_PORT` | `2222` | Host port for Git SSH |
| `CODEFEVER_SITE_HOST` | derived from HTTP port | Host URL shown in the clone panel, e.g. `https://git.example.com` |
| `CODEFEVER_SITE_SSH` | derived from SSH port | SSH URL shown in the clone panel, e.g. `ssh://git@host:2222/` |
| `CODEFEVER_DB_PASS` | `123456` | Database root password; `/`, `&` and `\` are supported |
| `CODEFEVER_DATA_DIR` | `./codefever-data` | Data directory (MySQL / bare repos / attachments / config) |
| `CODEFEVER_IMAGE` | `codefever-next:local` | Application image name |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | Database image (**8.0.x**; 8.4+ removed the old auth flag used by compose) |
| `CODEFEVER_BASE_RUNTIME` | `webdevops/php-nginx:7.4` | Runtime base image used at build time |
| `CODEFEVER_BASE_GO` | `golang:1.25-bookworm` | Go base image used at build time |
| `CODEFEVER_GOPROXY` | `https://proxy.golang.org,direct` | Go module proxy used at build time |
| `CODEFEVER_APT_MIRROR` | `https://archive.debian.org` | Debian buster archive mirror |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | Password hash format for new accounts: `legacy` (`md5(md5(x))`) or `argon2id` |

> Existing accounts still carry the historical `legacy` hashes, so `legacy` is the default —
> **existing accounts keep working without any password reset**. Switch to `argon2id` once
> every account has had its password reset.

> ⚠️ **Use MySQL 8.0.x.** The event worker and webhook deliverer use
> `FOR UPDATE SKIP LOCKED` (MySQL 8.0.1+). The compose authentication flag was removed in
> MySQL 8.4, so do not use 8.4/9.x with the provided compose file.

### E-mail (SMTP)

The image does **not** install an MTA, so the system `mail` fallback is normally unavailable.
Configure SMTP explicitly:

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<password>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls     # port 465 is treated as implicit TLS
```

Without SMTP, e-mail notifications fail and are only logged by the worker; Git operations
are not affected.

### Application configuration and persistence

All persistent data lives under **`codefever-data/`** in the repository root (ignored by
`.gitignore`):

```text
codefever-data/
├── mysql/          # MySQL data files
├── git-storage/    # bare repositories + hooks symlinks
├── file-storage/   # uploaded files and attachments
└── config/         # config.yaml + env.yaml (host/ssh, SMTP, token, …)
```

`env.yaml` and `config.yaml` inside the container are symlinks to `<base>/config/`, so
admin UI changes land directly in `codefever-data/config/` and survive container/image
upgrades. Set `CODEFEVER_DATA_DIR=/absolute/path` to move the directory.

Back up with the service stopped (consistent MySQL files):

```shell
docker compose stop
tar czf codefever-data-$(date +%Y%m%d).tar.gz codefever-data
docker compose start
```

> Windows PowerShell: `Compress-Archive -Path codefever-data -DestinationPath codefever-data.zip`.
> **Migrating from the old named volumes on Windows** requires `mysqldump` + restore:
> the Windows bind mount is case-insensitive, so MySQL changes
> `lower_case_table_names` from 0 to 2 and cannot reuse the old data directory directly.

- The image generates random `session.key`, `totp.salt` and `gateway.token` values at build time.
- A source installation must fill in `gateway.token` (a random string); otherwise the
  internal gateway callbacks cannot authenticate.
- Never commit `gateway.token` or reuse it across instances.

## Installing without Docker (from source)

You need `nginx`, `git`, `Go 1.23+` and `MySQL 8.0.x` (`backend/go.mod` declares
`go 1.23.0`; the Docker build uses a Go 1.25 base image, which is fine). The full guide is
[Install From Scratch](doc/en-us/installation/install_from_scratch.md); the core steps:

```shell
# 0. Source path convention: /data/www/codefever-next
#    The in-container path is /data/www/codefever-community; do not mix them.
mkdir -p /data/www
cd /data/www
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

# 1. Configuration: a source install uses the repository root
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml
chmod 0660 config.yaml env.yaml

# 2. Fill in random session key / TOTP salt / gateway token
KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml
sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml
sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" env.yaml

# 2.1 Set config.yaml host / ssh (source install defaults to 80/22)
#     host: http://<server-ip>
#     ssh:  git@<server-ip>:        # use ssh://git@<server-ip>:<port>/ for non-22

# 3. Build the five binaries
cd backend
mkdir -p ../bin
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api          ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker       ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook         ./cmd/codefever-hook
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-http-gateway ./cmd/codefever-http-gateway
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-ssh-gateway  ./cmd/codefever-ssh-gateway
cd ..

# 4. Initialise the database (reads the repository-root env.yaml)
cd backend/deploy/sql
sh ./create_db.sh
```

The source installation also needs:

- A `git` system user and home directory:
  ```shell
  useradd -rm -d /home/git -s /bin/bash git
  mkdir -p /home/git/.ssh && chown -R git:git /home/git/.ssh
  chown root:git config.yaml env.yaml
  chmod 0660 config.yaml env.yaml
  ```
- Runtime hook symlinks (the Dockerfile does this automatically; a source install must do
  it once):
  ```shell
  mkdir -p deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell
  for d in deploy/hooks misc/hooks git-storage/hooks; do
    ln -sfn ../../bin/codefever-hook "$d/main"
    ln -sfn main "$d/pre-receive"; ln -sfn main "$d/post-receive"; ln -sfn main "$d/update"
  done
  ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main
  ln -sfn view/build www/app
  ```
- nginx configured from `backend/deploy/docker/vhost-go.conf-template`, replacing
  `/data/www/codefever-community` with `/data/www/codefever-next`;
- SSH `ForceCommand` pointing at `<source>/bin/codefever-ssh-gateway`;
- systemd units for `bin/codefever-api`, `bin/codefever-worker` and
  `bin/codefever-http-gateway`, with the source root as working directory.

## Development and testing

```shell
cd backend

go build ./...          # build all packages (including the five commands)
go vet ./...            # static analysis
gofmt -l .              # formatting
go test ./internal/...  # unit tests
```

> **CI is back**: `.github/workflows/ci.yml` runs gofmt / build / Linux cross-build /
> vet / test on Go 1.23, plus `docker compose config`.
>
> ⚠️ **On Windows, prefer running the tests inside a Linux container** — some environments
> block file names such as `config.test.exe` and fail with `Access is denied` (a local
> policy issue, unrelated to the code). See
> [CONTRIBUTING.md · Testing](CONTRIBUTING.md#单元测试) for the containerised command.

Maintainer local quality gate (gofmt → build → Linux cross-build → vet → unit tests →
compose config; add `-BuildImage` to build the image):

```powershell
pwsh -File scripts/verify.ps1
```

> This gate does not start a database or application, and does not run contract, screenshot
> or write-path smoke tests. Those deeper checks have extra prerequisites; see
> [CONTRIBUTING.md](CONTRIBUTING.md#测试与验证). Normal deployments only need Compose.

Rebuilding the front-end bundle (do not reinstall dependencies during a normal deployment —
`@pgyer/mui-theme-preset` is declared as `latest` and would drift):

```shell
cd www/view && NODE_OPTIONS=--openssl-legacy-provider yarn build
```

The contract baseline lives in `backend/testdata/contract/`; collection and comparison are
historical acceptance tools, not runtime requirements. See
[CONTRIBUTING.md](CONTRIBUTING.md#契约回归).

> Line endings are normalised to LF by `.gitattributes`; editor conventions are declared in
> `.editorconfig`.

## Migrating from the PHP version

If you are running the old upstream PHP version:

- **No data migration is needed** — the database schema did not change. You have to log in
  again once (old sessions live in the client cookie, new sessions live in the server-side
  `cc_sessions` table and the two do not recognise each other).
- **The password hash stays on `legacy` by default** — existing accounts log in without
  any password reset.
- The `admin.systemStatus` endpoint lost the `php` and `crond` keys (they are meaningless
  once PHP is gone).
- `repository.fileContent` was fixed from the PHP-era "always 500" to 200.
- The old `misc/hooks` and `ssh-gateway/shell/main` paths are kept as compatibility symlinks
  in the image; new repositories use `deploy/hooks`.

## Known limitations

| Limitation | Impact | Workaround |
| --- | --- | --- |
| SSH key type detection follows the upstream regex | Some non-RSA keys (e.g. certain ed25519 variants) may be misdetected | Prefer `ssh-keygen -t rsa` |
| The `key` field for adding an SSH public key takes the base64 body only | Without the `ssh-rsa` prefix and the trailing comment | Matches how the front-end `UserSettingSSHKey.js` parses it |
| The 404 page references a missing image | `/static/assets/images/404_pic.png` renders as a broken image | Add the image yourself if desired; no functional impact |
| English user docs are not complete yet | Some pages under `doc/en-us/` are still in Chinese | See the note below; translation contributions welcome |

> Repository names **may** contain hyphens: both the nginx vhost and the Go backend use the
> `[0-9a-z_.-]+` rule. See [LOCAL_DEPLOY.md](LOCAL_DEPLOY.md) for build differences.

## Documentation languages

The Chinese documentation under `doc/zh-cn/` is the primary and most complete set. The
English documentation under `doc/en-us/` is being translated incrementally; installation,
compose and upgrade pages are kept in sync, while some admin/repository pages may still be
in Chinese. If a page matters to you, please open an Issue or send a translation PR.

## User guide

- [Repository](doc/en-us/repo) · [Repository group](doc/en-us/repo_group) · [Administrator](doc/en-us/admin) · [Git](doc/en-us/git) · [Webhook](doc/en-us/repo/webhooks.md) · [FAQ](doc/en-us/common)

## Feedback

- If you hit an unexpected result, please open an `Issue`. See: [Bug Fix Issue](doc/en-us/contribute/bug_fix_issue.md)
- If you have an idea for a new feature, please open an `Issue` as well. See: [Feature Request issue](doc/en-us/contribute/request_feature_issue.md)
- Please do **not** report security vulnerabilities publicly — see the [Security Policy](SECURITY.md).

## Contribute

Feel free to submit a `PR`, and please make sure to test it properly after fixing a `Bug`
or adding a `Feature`.

- If the PR is a bug fix, please submit the `Issue` first and reference it. See: [Bug Fix PR](doc/en-us/contribute/bug_fix_pr.md)
- If the PR is a new feature, please describe it in as much detail as possible, and reference the `Issue` when it targets a feature request. See: [New Feature PR](doc/en-us/contribute/new_feature_pr.md)

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for the development environment, the project
layout, the code and commit conventions, the testing workflow and the PR process.

## Contribute to docs

You are welcome to correct errors in the documentation or to take part in the translation.
Please make sure the content is accurate before submitting a `PR`, see: [Document PR](doc/en-us/contribute/doc_pr.md)

## Project governance

| File | Content |
| --- | --- |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Dev environment, code and commit conventions, testing, PR workflow |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community code of conduct |
| [SECURITY.md](SECURITY.md) | Private vulnerability reporting, hardening the default configuration, known limitations |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |
| [.github/](.github/) | Issue templates, PR template |

## About

The `CodeFever` project was initially developed and maintained by `PGYER`, went live in
June 2020 and was open sourced under the MIT license in February 2022; the open-source
version is called `CodeFever Community`.

`CodeFever Next` is a **second open-source release** built on top of that version: the
backend has been completely rewritten in Go, the runtime image no longer contains PHP, and
the application image is built from local source. It retains most of the upstream features
and is optimised for standalone deployment.

### Maintainer

| | |
| --- | --- |
| Maintainer | [GiserXiao](https://github.com/AidenX0704) |
| Repository | <https://github.com/AidenX0704/codefever-next> |
| Issue tracker | <https://github.com/AidenX0704/codefever-next/issues> |
| Security issues | Not reported publicly — see [SECURITY.md](SECURITY.md) |

This repository is an **unofficial derivative** developed independently by the maintainer
listed above, with no affiliation to the original PGYER team. Trademarks, logos and other
intellectual property of the upstream project remain the property of their original
rightsholders; the visual assets are kept here only as the version identifier.

## License

[MIT](LICENSE) © CodeFever authors

This repository retains the original LICENSE and copyright notice of the CodeFever project
(Copyright © 2022 CodeFever authors). The second-generation changes are copyright the
maintainers of this repository and are likewise released under the MIT license.

## Acknowledgements

- [CodeFever](https://codefever.cn) — the original project and product design
- [CodeIgniter](https://codeigniter.com/) — the framework used by the historical PHP implementation
- Everyone who contributed to the Go backend rewrite, the contract regression suite and the documentation
