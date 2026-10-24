# 概览和系统服务

此页面没有管理功能，仅用于监视 `系统资源使用量` 和 `服务状态`。

### 进入概览页面

能查看 `概览页面` 需要满足以下条件：

- `CodeFever` 系统 `管理员`

进入 `概览页面` 的步骤：

1. 在 `CodeFever` 导航栏右侧找到 `管理后台` 按钮
2. 点击 `管理后台` 按钮进入 `管理后台`
3. 点击左侧菜单栏 `概览` 选项

### 统计

统计部分用于展示 `CodeFever` 使用量。

> `CodeFever` 对于使用量没有限制。

### 系统资源

统计部分用于展示 `系统资源` 使用量。

> 当出现性能问题时，需要查看此面板确定有足够的 `系统资源` 可以使用。

### 磁盘使用量

统计部分用于展示 `磁盘` 使用量。

> Git 仓库保存在应用根目录的 `git-storage/` 下：
> - Docker Compose：仓库根目录的 `codefever-data/git-storage/`；
> - 源码安装：`/data/www/codefever-next/git-storage`。
>
> 如果磁盘空间不足，请扩容或把该目录迁移到更大的磁盘后再挂载回来。

### 系统服务状态及维护

后端已经完全 Go 化，**没有 PHP-FPM / php-fpm / crond**。运行中的进程是：

| 组件 | 说明 | 运行方式 |
| --- | --- | --- |
| `nginx` | HTTP 反向代理、静态资源、Git HTTP 入口 | 基础镜像 / 系统服务 |
| `codefever-api` | API、服务端渲染页面、文件下载 | supervisor（容器）/ systemd（源码） |
| `codefever-worker` | 事件、站内信、邮件、webhook 投递 | supervisor（容器）/ systemd（源码） |
| `codefever-http-gateway` | Git over HTTP 网关（127.0.0.1:27555） | supervisor（容器）/ systemd（源码） |
| `codefever-ssh-gateway` | SSH forced command，每次连接由 sshd 拉起 | 不常驻 |
| `sshd` | Git over SSH 入口 | 基础镜像 / 系统服务 |

> 管理端概览的 `service` 段只返回 `nginx`、`codefever`（以 `codefever-api` 进程判定）、
> `sendmail` 三个键；没有 PHP/crond。`sendmail` 现在代表“系统邮件通道”，
> CodeFever 更推荐配置 SMTP。

#### 容器部署（Docker Compose）

```shell
# 查看全部常驻进程
docker exec -it codefever-next-app supervisorctl status

# 单个服务
docker exec -it codefever-next-app supervisorctl restart codefever-api
docker exec -it codefever-next-app supervisorctl restart codefever-worker
docker exec -it codefever-next-app supervisorctl restart codefever-http-gateway

# nginx
docker exec -it codefever-next-app nginx -t
docker exec -it codefever-next-app nginx -s reload

# 日志
docker compose logs -f codefever
```

#### 源码安装（systemd）

```shell
systemctl status codefever-api codefever-worker codefever-http-gateway
systemctl restart codefever-api codefever-worker codefever-http-gateway

# nginx
nginx -t && systemctl reload nginx

# sshd（改了 Match/ForceCommand 后）
sshd -t && systemctl reload ssh
```

> `codefever-service-template` 只用于旧式 VM 的 HTTP 网关 init 脚本；
> 容器内由 supervisor 管理，源码安装推荐直接用 systemd。

#### 邮件（Sendmail / SMTP）

镜像**不安装 MTA**，`sendmail`/`mail` 回退通常不可用。请在部署时配置 SMTP：

```shell
CODEFEVER_SMTP_HOST=smtp.example.com
CODEFEVER_SMTP_PORT=587
CODEFEVER_SMTP_USER=git@example.com
CODEFEVER_SMTP_PASSWORD='<口令>'
CODEFEVER_SMTP_FROM='CodeFever <git@example.com>'
CODEFEVER_SMTP_TLS=starttls
```

未配置 SMTP 时，概览里的 sendmail 状态可能为 stopped，邮件通知只会在 worker
日志里记录失败，不影响 Git 操作。
