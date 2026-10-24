# 安全策略

## 支持的版本

CodeFever Next 目前以 `main` 分支为主要维护对象，尚未建立长期支持（LTS）分支：

| 版本 | 支持情况 |
| --- | --- |
| `main`（当前开发分支） | ✅ 接收安全修复 |
| 历史 0.1.0 快照 | ⚠️ 仅在报告者提供可复现步骤的前提下评估是否修复 |

> 0.1.0 是首个公开快照；如仓库没有对应的 `v0.1.0` tag，请以 commit 为准。

## 报告安全漏洞

**请不要通过公开的 Issue 报告安全漏洞。**

请通过 GitHub 的
[私密漏洞报告](https://github.com/AidenX0704/codefever-next/security/advisories/new)
提交（若仓库未开启该功能，请通过 GitHub [@AidenX0704](https://github.com/AidenX0704)
的资料页所列联系方式私下联系维护者）。
请在标题或正文中注明 `security`，避免被当作普通 Issue 公开。

报告中请尽量包含：

1. 受影响的组件与版本（前端 / Go 后端 / Git 网关 / 镜像）
2. 漏洞类型（如越权、路径穿越、注入、权限提升、信息泄露等）
3. 最小可复现步骤或 PoC
4. 影响面评估（谁能利用、需要什么前提条件）
5. 你已知的缓解方案

### 我们的响应节奏

| 阶段 | 目标时间 |
| --- | --- |
| 确认收到并初步定性 | 3 个工作日 |
| 给出修复方案或缓解建议 | 10 个工作日 |
| 发布修复版本 | 与方案确定时间同步 |

我们会在修复发布后**公开披露**漏洞信息（除非披露会造成额外风险），并在
[CHANGELOG](CHANGELOG.md) 中记录。

## 部署方需要注意的默认配置

以下默认值**仅为方便首次启动**，正式部署前务必修改：

| 项 | 默认值 | 风险 |
| --- | --- | --- |
| 管理员初始口令 | `123456` | 任何人都能登录管理后台，**必须在首次登录后立即修改并绑定 MFA** |
| 数据库 root 口令 | `123456` | 同上；可用 `CODEFEVER_DB_PASS` 设置含 `/`、`&`、`\` 的强口令 |
| 开放注册 | `true` | 公网实例应关闭，否则任何人可注册账号 |
| 口令哈希格式 | `legacy`（`md5(md5(x))`） | 弱哈希。存量账号迁移完成、全部账号重置口令后，应切到 `argon2id` |
| 服务端口 | `9080` / `2222` | 如直接暴露公网，建议配合防火墙/反向代理 |
| 站点地址 | 按端口生成 | 反代/域名场景需设置 `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH` |
| 网关 token | 镜像构建期随机生成；源码安装为模板占位符 | 源码安装**必须**手填随机值；不要把 token 提交到仓库或在实例间复用 |

对应的环境变量：

```shell
CODEFEVER_DB_PASS='<强口令>'
CODEFEVER_PASSWORD_HASH=argon2id       # 全部账号重置口令后再切换
CODEFEVER_SITE_HOST=https://git.example.com
CODEFEVER_SITE_SSH=ssh://git@git.example.com:2222/
```

### SMTP 与邮件

镜像**不安装 MTA**；不配置 SMTP 时，系统 `mail` 回退通常不可用，邮件通知只在 worker
日志里记录失败。生产部署请配置：

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<口令>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls
```

### 配置与数据

- 容器内 `config.yaml` / `env.yaml` 是 `<base>/config/` 的软链，`config/` 位于
  `codefever-data/config/` 绑定目录；管理后台修改会持久化，但也意味着**该目录包含
  DB 口令、SMTP 口令、session key、TOTP salt 与 gateway token**，备份/迁移时要按敏感数据处理。
- `config.yaml` / `env.yaml` 权限为 `0660`（容器内 `root:git`）；源码安装请保持
  `root:git` + `0660`，不要用 `0777`。
- `git-storage` / `file-storage` 是用户数据；容器入口只对必要目录做 0777，
  新仓库由 API 创建时设置权限。

## 已知的安全相关限制

| 项 | 说明 |
| --- | --- |
| 404 页面引用了仓库中不存在的图片 `/static/assets/images/404_pic.png` | 上游从未提供该资源，404 页面会出现一个破图。不影响功能与安全。 |
| SSH 公钥类型识别沿用上游正则 | 对部分非 RSA 公钥（如某些 ed25519 变体）可能识别不稳定，建议使用 `ssh-keygen -t rsa`。 |
| 添加 SSH 公钥时 `key` 字段只传 base64 主体 | 不含 `ssh-rsa` 前缀与注释，与前端 `UserSettingSSHKey.js` 的解析一致。 |
| 基础镜像为 Debian buster（EOL） | 运行时基础镜像较旧，apt 走归档源；这是后续迁移项，建议关注上游安全公告。 |
| `legacy` 口令哈希 | 默认保持历史兼容；新部署/重置口令时可切 `argon2id`。 |
| 登录 / 2FA / 邮件验证码没有内建速率限制 | 公网实例请在 nginx 或外层网关配置 `limit_req`，按 IP + 账号限制失败次数；这是后续版本的内建目标。 |

> 仓库名**可以**包含连字符：nginx vhost 与 Go 后端统一使用 `[0-9a-z_.-]+`。
> 旧版“仓库名不能含连字符”的限制已解除。

## 依赖安全

前端依赖锁定在 `www/view/yarn.lock`，后端依赖锁定在 `backend/go.sum`。
仓库已恢复 CI（Go 门禁 + compose config），但仍未配置 Dependabot 等自动化依赖检查；
升级依赖前请自行核对上游安全公告。

如发现依赖存在漏洞，请按上面的方式私下报告。
