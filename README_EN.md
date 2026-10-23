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
[![CI](https://img.shields.io/badge/CI-passing-4CAF50?logo=githubactions&logoColor=white)](.github/workflows/ci.yml)

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
process, PHP executable or PHP configuration.

| Area | Status |
| --- | --- |
| Backend | Go (`backend/`), replacing the former CodeIgniter 2.1.1 + PHP 7.4 implementation |
| API contract | 108 `/api/*` endpoints + 4 `/apiv1/internal` callbacks, **contract regression 123/123 as expected** |
| Web routes | 43 server-rendered routes + 5 pages, **byte-for-byte identical to the PHP version (43/43)** (one brand string is a deliberate change; the baseline was synced) |
| Front-end | React 17 + MUI v4; the bundle `www/view/build` is committed; **dependencies were not reinstalled**, only the brand strings changed for the second release |
| Runtime | **No PHP, no php-fpm, no crond** in the image |
| Images | **Built entirely from local source**, no upstream pre-built image involved |
| Size | The front-end vendor tree shrank from 23.2 MB / 2072 files to 3.5 MB / 27 files (only provably unreferenced files were removed) |

> The full dependency surface, the stability gate and the verification data of the migration
> are documented in [`docs/PHP_REMOVAL.md`](docs/PHP_REMOVAL.md).

## Features

- 🆓 **Free:** No paywall, no crippled features
- ❤️ **Open source:** MIT licensed, no compiled or encrypted code
- 🥬 **Lightweight:** One `docker compose up -d --build` and you are done
- ⚡️ **Efficient:** Runs on 1 vCPU / 1 GB RAM; measured p99 ≈ 178 ms at 64 concurrent clients with a 0 % error rate
- ♾️ **Unlimited:** No limit on the number of repositories
- ⌨️ **Code review:** Highlighted diff between versions
- 🛤️ **Branches:** Full support for native Git branches and tags
- 🙋️ **Merge requests:** Team collaboration and review
- 🙅 **Branch protection:** Keeps commits safe and reviewable
- 👥 **Collaboration:** Roles and permissions per member
- 🔌 **Webhook:** Integrates with Jenkins / Jira and friends
- 🖥️ **Admin portal:** Super administrators manage all projects and users
- 💬 **Bilingual:** Native Chinese and English
- 🔑 **SSH push:** Clone / pull / push over SSH public-key authentication

## Quick start

Requirements: Docker 20.10+ and Docker Compose v2.

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

Once it is up:

| Item | Value |
| --- | --- |
| Web UI | <http://localhost:9080> |
| Default account | `root@codefever.cn` |
| Default password | `123456` |
| Git over HTTP | `http://<user>@localhost:9080/<group>/<repo>.git` |
| Git over SSH | `ssh -p 2222 git@localhost` |

> **Change the password and bind an MFA device right after the first login.**

Common commands:

```shell
docker compose ps                         # status
docker compose logs -f codefever-next-app # follow the logs
docker compose restart codefever-next-app # restart the app
docker compose down                       # stop (keep the volumes)
docker compose down -v                    # stop and delete the volumes (all data is lost)
```

The image is **built from the current source**, so there is no pre-built image to pull.
Ports, the database image and the base image registries are configurable via environment
variables:

```shell
CODEFEVER_HTTP_PORT=18080 CODEFEVER_SSH_PORT=18022 docker compose up -d
```

See [Install with docker-compose](doc/en-us/installation/install_via_docker_compose.md).
For offline distribution (`docker save` / `docker load`), switching registry mirrors or the
one-shot PowerShell scripts, see [Local image build and deployment](LOCAL_DEPLOY.md).

<details>
<summary><b>Installing without Docker (from source)</b></summary>

You need `nginx`, `git`, `Go 1.25+` and `MySQL 8.0+`. The core steps:

```shell
# 1. Prepare the configuration
cp config.template.yaml config.yaml
cp env.template.yaml env.yaml

# 2. Build the Go services
cd backend && go build -o ../bin/codefever-api    ./cmd/codefever-api \
                 && go build -o ../bin/codefever-worker ./cmd/codefever-worker \
                 && go build -o ../bin/codefever-hook   ./cmd/codefever-hook

# 3. Initialise the database
cd ../misc && sh ./create_db.sh
```

The full 15-step guide lives in [Install From Scratch](doc/en-us/installation/install_from_scratch.md).

</details>

## Project status

### Done

- ✅ **PHP runtime removed** — the backend is 100 % Go; no PHP / php-fpm / crond in the image
- ✅ **Built from local source** — a single recipe, `Dockerfile`; no upstream pre-built image
- ✅ **Contract regression** — API 123/123, SSR routes 43/43 byte-for-byte identical to PHP
- ✅ **Front-end slimmed down** — vendor tree trimmed from 23.2 MB / 2072 files to 3.5 MB / 27 files
- ✅ **Governance in place** — CONTRIBUTING / CODE_OF_CONDUCT / SECURITY / CHANGELOG / CI / Issue & PR templates
- ✅ **Documentation rewritten** — installation, configuration and usage docs rewritten for the Go backend in both languages

### Planned

- ⬜ Lift the hyphen restriction on repository names (extend the nginx vhost rule)
- ⬜ Support ed25519 / ECDSA SSH public keys
- ⬜ Move password hashes from `legacy` to `argon2id` without a flag day
- ⬜ Add a nightly image-build and contract-regression pipeline to CI

> Pick up any of the above, or just open an Issue / PR — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Architecture

```
                        ┌───────────────────────────────┐
   Browser  ──HTTP──▶   │  nginx                        │
   git client ──SSH──▶  │   ├── /api /user /doc …  ──▶  │ 127.0.0.1:9001
                        │   ├── /*.git              ──▶  │ 127.0.0.1:27555
                        │   └── /static /app        ──▶  │ static bundle
                        └───────────────────────────────┘
                                          │
      ┌───────────────────────────────────┴────────────────────────────────┐
      │                                                                │
┌─────▼──────────────────┐                              ┌──────────────────▼─────┐
│ codefever-api  (Go)    │                              │ http-gateway   (Go)   │
│  /api/*  /apiv1/*      │                              │  Git over HTTP        │
│  43 web routes         │                              └────────────────────────┘
│  5 server-rendered     │
└─────┬──────────────────┘                              ┌────────────────────────┐
      │                                                 │ ssh-gateway/shell (Go) │
┌─────▼──────────────────┐                              │  Git over SSH forced  │
│ codefever-worker (Go)  │                              │  command              │
│  events / inbox / mail │                              └────────────────────────┘
│  webhook delivery      │
└────────────────────────┘
      │
┌─────▼──────────────────┐   ┌──────────────┐   ┌───────────────┐
│ codefever-hook  (Go)   │──▶│  git-storage │   │  MySQL 8.0    │
│  pre-receive / update  │   │  bare repos  │   │  cc_* tables  │
│  post-receive          │   └──────────────┘   └───────────────┘
└────────────────────────┘
```

| Directory | Description |
| --- | --- |
| `backend/` | The Go backend (API / worker / git hook / contract tooling), module `codefever-next/backend` |
| `http-gateway/` | Git over HTTP gateway |
| `ssh-gateway/shell/` | Git over SSH forced command |
| `www/` | Front-end sources `www/view/src`, the built bundle `www/view/build`, static assets |
| `misc/` | nginx / supervisor templates, database schema and bootstrap scripts, git hooks |
| `git-storage/` | Where the bare repositories live (a Docker volume mount point; only an empty skeleton is kept in the repo) |
| `doc/` | User documentation (`zh-cn` / `en-us`) |
| `docs/` | Analysis, plan and implementation records of the backend rewrite |
| `scripts/` | Local build, run, verification and screenshot comparison scripts |
| `.github/` | CI workflow, issue templates, PR template, Dependabot configuration |

## Configuration

### Orchestration environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | Host port for HTTP |
| `CODEFEVER_SSH_PORT` | `2222` | Host port for Git SSH |
| `CODEFEVER_DB_PASS` | `123456` | Database root password |
| `CODEFEVER_IMAGE` | `codefever-next:local` | Application image name |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | Database image (**8.0+ is required**, see below) |
| `CODEFEVER_BASE_RUNTIME` | `docker.1ms.run/webdevops/php-nginx:7.4` | Runtime base image used at build time |
| `CODEFEVER_BASE_GO` | `docker.1ms.run/library/golang:1.25-bookworm` | Go base image used at build time |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | Password hash format for new accounts: `legacy` (`md5(md5(x))`) or `argon2id` |

> Existing accounts still carry the historical `legacy` hashes, so `legacy` is the default —
> **existing accounts keep working without any password reset**. Switch to `argon2id` once
> every account has had its password reset.

> ⚠️ **The database must be MySQL 8.0+.** The event worker and the webhook deliverer claim
> their work with `FOR UPDATE SKIP LOCKED`, which only exists since MySQL 8.0.1. On 5.7
> those two queries fail with a syntax error, which silently kills the activity feed,
> in-app notifications, e-mail notifications and webhooks — the symptom is a `SKIP LOCKED`
> error in the logs every two seconds.

### Application configuration

`env.yaml` (database, mail, gateway token, site info) and `config.yaml` (site title,
registration number, …) are derived from `env.template.yaml` / `config.template.yaml`.
On container start-up a random `session_key` and `totp_salt` are generated into `env.yaml`.

## Development and testing

```shell
cd backend

go build ./...          # build
go vet ./...            # static analysis
gofmt -l .              # formatting
go test ./internal/...  # unit tests
```

> ⚠️ **On Windows, prefer running the tests inside a Linux container** — some environments
> block file names such as `config.test.exe` and fail with `Access is denied`
> (a local policy issue, unrelated to the code). See
> [CONTRIBUTING.md · Testing](CONTRIBUTING.md#单元测试) for the containerised command.

One-shot verification pipeline (build + unit tests + contract comparison + screenshot
comparison + image packaging):

```powershell
pwsh -File scripts/verify.ps1
```

Rebuilding the front-end bundle (**do not** reinstall the dependencies —
`@pgyer/mui-theme-preset` is declared as `latest` and would drift):

```shell
cd www/view && yarn install && yarn build
```

The contract baseline lives in `backend/testdata/contract/`; the Go side compares the
response shape of all 123 endpoints against it.

> Line endings are normalised to LF by `.gitattributes`; editor conventions are
> declared in `.editorconfig`.

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

## Known limitations

The following limitations are inherited from the upstream implementation and have not been
lifted in this second release:

| Limitation | Impact | Workaround |
| --- | --- | --- |
| Repository names cannot contain hyphens (over HTTP) | The nginx vhost rule is `^/[0-9a-z_]+/[0-9a-z_]+\.git`, which excludes `-` | Avoid `-` in names (e.g. `demorepo`); **SSH access is not affected** |
| Only RSA SSH public keys are recognised | The key type is matched by a regex on the base64-decoded line, so ed25519 is misdetected as `ssh-rsa` | Generate keys with `ssh-keygen -t rsa` |
| The `key` field for adding an SSH public key takes the base64 body only | Without the `ssh-rsa` prefix and the trailing comment | Matches how the front-end `UserSettingSSHKey.js` parses it |

> The full list of limitations, and the item-by-item differences against the upstream build
> recipe, live in [Local image build and deployment](LOCAL_DEPLOY.md).

## User guide

- [Repository](doc/en-us/repo) · [Repository group](doc/en-us/repo_group) · [Administrator](doc/en-us/admin) · [Git](doc/en-us/git) · [FAQ](doc/en-us/common)

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
| [.github/](.github/) | CI workflow, issue templates, PR template, Dependabot configuration |

## About

The `CodeFever` project was initially developed and maintained by `PGYER`, went live in
June 2020 and was open sourced under the MIT license in February 2022; the open-source
version is called `CodeFever Community`.

`CodeFever Next` is a **second open-source release** built on top of that version: the
backend has been completely rewritten in Go, the runtime image no longer contains PHP, and
all images are now built from local source. It retains most of the upstream features and
is optimised for standalone deployment.

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
