# Welcome to CodeFever Next

> **CodeFever Next is a second-generation fork of [PGYER/codefever](https://github.com/PGYER/codefever).**
> It is an unofficial derivative, independently maintained by
> [GiserXiao](https://github.com/AidenX0704), and is not affiliated with the
> original PGYER team. The original project is copyright 蒲公英开发者服务平台 and was
> open sourced under the MIT license; this repository retains its LICENSE and copyright notice.

### Installation

`CodeFever Next` can be installed with `docker compose` or `from scratch` — pick the one
that matches your needs.

- **(Recommended)** The shortest route is `docker compose up -d --build`. The first build
  needs access to two base images, the Debian archive and a Go module proxy. See:
  [Install with docker-compose](../installation/install_via_docker_compose.md)
- If you want to inspect the image build or already have an external MySQL, see:
  [Install via Docker Image](../installation/install_via_docker.md)
- If you need it for learning, research or customisation, install it manually. See:
  [Install From Scratch](../installation/install_from_scratch.md)

### Upgrade

`CodeFever Next` can be upgraded while keeping your existing data. See:
[Update CodeFever Next](../installation/update.md)

### Usage

- [Repository](../repo)
- [Repository group](../repo_group)
- [Administrator](../admin)
- [Git](../git)
- [FAQ](../common)

### Feedback

- If you hit an unexpected result while using it, please open an `Issue`. See:
  [Bug Fix Issue](../contribute/bug_fix_issue.md)
- If you would like to see a new feature, please open an `Issue` as well. See:
  [Feature Request Issue](../contribute/request_feature_issue.md)

### Contribute

Feel free to submit a `PR`, and please make sure to test it properly after fixing a `Bug`
or adding a `Feature`.

- If the PR is a bug fix, please submit the `Issue` first and reference it. See:
  [Bug Fix PR](../contribute/bug_fix_pr.md)
- If the PR is a new feature, please describe it in as much detail as possible, and reference
  the `Issue` when it targets a feature request. See: [New Feature PR](../contribute/new_feature_pr.md)

### Contribute to docs

You are welcome to correct errors in the documentation or to take part in the translation.
Please make sure the content is accurate before submitting a `PR`, see:
[Document PR](../contribute/doc_pr.md)

### About CodeFever

The `CodeFever` project was initially developed and maintained by `PGYER`, went live in
June 2020 and was open sourced under the MIT license in February 2022, after nearly two years
of refinement and stability verification.

After the open sourcing, the community edition is called `CodeFever Community`, while the
public cloud offering continues under the name `CodeFever`.

`CodeFever Community` was developed entirely by the `PGYER` team and released under the `MIT`
license with fully independent intellectual property, so you can use it without worrying about
infringing anyone's rights. It retains most of the features of `CodeFever` and is optimised for
standalone deployment.

### About CodeFever Next

`CodeFever Next` is built on top of that open-sourced version. The backend has been
**completely rewritten in Go** — the runtime image no longer contains PHP at all — and every
image is now built from local source.

See the [repository README](../../../README_EN.md) for the full architecture overview, the
migration notes and the project roadmap.
