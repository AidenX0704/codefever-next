# 欢迎使用 CodeFever Next

> **CodeFever Next 是基于 [PGYER/codefever](https://github.com/PGYER/codefever) 进行二次开发的 Git 代码托管平台。**
> 本项目为非官方衍生版本，由 [GiserXiao](https://github.com/AidenX0704) 独立维护，与蒲公英官方团队无隶属关系。
> 原始 CodeFever 项目版权归蒲公英开发者服务平台所有，基于 MIT 协议开源，本仓库保留其 LICENSE 及版权声明。

### 安装

`CodeFever Next` 提供 `docker compose` 和 `从零开始安装` 两种方式，可以根据实际需要选择。

- **(推荐)** 最简单的安装方式是 `docker compose up -d --build`；首次构建需要能访问两个基础镜像、Debian 归档源与 Go module 代理。具体见：[使用 docker-compose 安装](../installation/install_via_docker_compose.md)

- 如果想了解镜像构建细节，或已经有一个外部 MySQL，可见：[Docker 镜像安装](../installation/install_via_docker.md)

- 如果出于学习、研究或定制化需求，也可以手动方式安装，具体可见: [从零开始安装](../installation/install_from_scratch.md)

### 更新

`CodeFever Next` 可以进行保留原有数据更新, 按照如下文档即可完成更新操作。具体可见：[更新 CodeFever Next](../installation/update.md)

### 使用

- [仓库](../repo)
- [仓库组](../repo_group)
- [管理员设置](../admin)
- [Git](../git)
- [常见问题](../common)

### 问题反馈

- 如果你在使用过程中遇到期望外的结果，欢迎提交 `Issue`。参照: [提交问题](../contribute/bug_fix_issue.md)
- 如果你希望在 `CodeFever Next` 添加一些特性, 也欢迎提交 `Issue`。参照: [提交特性请求](../contribute/request_feature_issue.md)

### 贡献代码

欢迎提交 `PR`, 请确定修复 `Bug` 后或者新增 `Feature` 后进行适当的测试。

- 如果 `PR` 内容是问题，请先提交 `Issue` 并在提交 PR 时引用该 `Issue`。参照: [提交问题修复](../contribute/bug_fix_pr.md)
- 如果 `PR` 内容是新特性，请在 `PR` 中请尽量详细描述此特性的内容, 如果此 PR 是针对某个特性请求的提交, 在提交 PR 时引用该 `Issue`。参照: [提交新特性](../contribute/new_feature_pr.md)

### 贡献文档

欢迎指正文档中的错误或参加翻译工作，提交 `PR` 前请确定文档内容表述是否准确，具体参照: [提交文档](../contribute/doc_pr.md)

### 关于 CodeFever

`CodeFever` 项目起初由 `蒲公英开发者服务平台` 开发和维护, 项目于 2020 年 6 月上线 [https://codefever.pgyer.com/](https://codefever.pgyer.com/) 。 经过将近两年时间打磨和稳定性验证, 于 2022 年 2 月开源, 接受社区的考验。

`CodeFever` 开源后, 开源版本称为 `CodeFever Community` 版本。同时公有云版本继续服务，继续称为 `CodeFever`。

`CodeFever Community` 由 `蒲公英开发者服务平台` 团队完整自主研发并基于 `MIT` 协议进行完整开源, 拥有完全自主的知识产权, 因此您可以放心使用而不必考虑是否侵犯他人权利。

`CodeFever Community` 保留了 `CodeFever` 的大部分功能, 并且进行了更适合单机部署的优化。

### 关于 CodeFever Next

`CodeFever Next` 是在上述开源版本基础上的二次开发版本: 后端已**用 Go 完整重写**,
运行镜像中不再包含任何 PHP, 并且镜像全部改为从本地源码构建。

完整的架构说明、迁移注意事项与后续计划, 见[仓库 README](../../../README.md)。
