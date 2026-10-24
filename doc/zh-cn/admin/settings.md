# 设置

管理员设置中的“设置”页用于修改 CodeFever 的全局配置：**主机地址（HTTP/SSH）**、
**发件人名称/地址** 与 **开放注册**。

进入方式：导航栏右侧“管理后台” → 左侧“设置”。

### 主机

“主机”影响：

- 仓库 `clone` 面板中的 HTTP / SSH 仓库 URL；
- 空仓库首页提示中的仓库 URL。

两个字段：

| 字段 | 示例 | 说明 |
| --- | --- | --- |
| URL 前缀（`host`） | `https://git.example.com` | HTTP 协议 + 域名/IP；不要以 `/` 结尾 |
| SSH 前缀（`ssh`） | `ssh://git@git.example.com:2222/` | 必须用 `ssh://` 形式才能带端口；不要省略结尾 `/` |

> 默认值由容器入口生成：`http://localhost:<CODEFEVER_HTTP_PORT>` 与
> `ssh://git@localhost:<CODEFEVER_SSH_PORT>/`。放在反向代理/域名后面时，
> 推荐直接用 `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH` 覆盖；管理后台的修改
> 会写入 `config.yaml`，并持久化在仓库根目录的 `codefever-data/config/`。

修改步骤：

1. 打开“管理后台 → 设置”；
2. 修改“主机”下的 URL / SSH 前缀；
3. 点击“保存”；
4. 回到仓库首页刷新 clone 面板确认。

### 邮件 / SMTP

“设置”页里的“发件人名称”和“发件人地址”只决定邮件的 **From 显示名与地址**；
真正的 SMTP 服务器、端口、账号、密码、TLS 通过环境变量配置（不在页面上）：

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<口令>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls     # 465 端口自动按 implicit TLS
```

> 镜像不安装 MTA；未配置 `CODEFEVER_SMTP_HOST` 时，邮件会回退到系统 `mail`，
> 通常不可用，只会在 worker 日志中记录失败。

为避免被收件方判为垃圾邮件，建议为发件域名配置 SPF，必要时配置 PTR：

```plain
<发件域名>             TXT   v=spf1 ip4:<服务器 IP> ~all
<服务器 IP>            PTR   <发件域名>
```

### 注册

“开放注册”控制新用户能否自行注册。

> 出于安全考虑，公网私有化部署**强烈建议关闭开放注册**。
> `config.template.yaml` 中默认是 `allowRegister: true`，首次登录后请在“设置”页关闭。

修改步骤：

1. 打开“管理后台 → 设置”；
2. 找到“注册”；
3. 关闭“开放注册”并保存。
