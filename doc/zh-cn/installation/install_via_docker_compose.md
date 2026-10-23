# 使用 docker-compose 安装

## 安装步骤

### 编排文件说明

仓库根目录的 `docker-compose.yml` 是本项目**唯一**的编排文件，包含两个服务：

| 服务 | 说明 |
| --- | --- |
| `codefever` | 应用本体，镜像 **从本地源码构建**（`Dockerfile`），不依赖任何上游预构建镜像 |
| `db` | MySQL 8.0，独立成服务，便于单独备份数据库或连接外部数据库 |

数据全部存放在**命名卷**里（`db-data` / `git-storage` / `file-storage`）。
用命名卷而不是绑定挂载，是因为命名卷首次创建时会从镜像内容初始化，
`git-storage/hooks/*` 的符号链接才能得以保留 —— 绑定挂载会把宿主机目录整个盖上去，钩子会失效。

### 拉取代码并启动

```shell
git clone https://github.com/AidenX0704/codefever-next.git codefever-next
cd codefever-next

docker compose up -d --build
```

`--build` 会强制用当前源码重新构建镜像；镜像已存在时直接 `docker compose up -d` 即可复用。

1. 服务启动后访问 `http://127.0.0.1` 或 `http://<server ip>` 登录。
2. 默认端口是 `9080`（HTTP）与 `2222`（Git SSH）。80/22/8080 常被占用，因此默认改了端口。
   需要改端口时设置环境变量：

   ```shell
   export CODEFEVER_HTTP_PORT=18080
   export CODEFEVER_SSH_PORT=18022
   docker compose up -d
   ```

3. 如果服务异常，可以进入 Shell 人工维护，也可以直接 `docker compose restart codefever`。
4. 默认管理员用户：`root@codefever.cn`，密码：`123456`。**登录后请立即修改密码并绑定 MFA 设备。**

### 可配置的环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CODEFEVER_HTTP_PORT` | `9080` | 宿主 HTTP 端口 |
| `CODEFEVER_SSH_PORT` | `2222` | 宿主 Git SSH 端口 |
| `CODEFEVER_DB_PASS` | `123456` | 数据库 root 口令（`MYSQL_ROOT_PASSWORD` 与 `DB_PASS` 共用此变量，保证一致） |
| `CODEFEVER_IMAGE` | `codefever-next:local` | 应用镜像名 |
| `CODEFEVER_MYSQL_IMAGE` | `mysql:8.0` | 数据库镜像（**必须 8.0+**） |
| `CODEFEVER_BASE_RUNTIME` | `docker.1ms.run/webdevops/php-nginx:7.4` | 构建期运行时基础镜像 |
| `CODEFEVER_BASE_GO` | `docker.1ms.run/library/golang:1.25-bookworm` | 构建期 Go 基础镜像 |
| `CODEFEVER_PASSWORD_HASH` | `legacy` | 新口令哈希格式，`legacy`（`md5(md5(x))`）或 `argon2id` |

> **关于 `CODEFEVER_PASSWORD_HASH`**：存量账号的口令哈希是历史遗留的 `legacy` 格式，
> 因此默认保持 `legacy`，**存量账号无需重置口令即可登录**。
> 待全部账号重置过口令之后，再把它切成 `argon2id`（OWASP 推荐的内存硬函数）。

### 接入外部数据库

如果数据库独立于容器之外（例如云数据库），把 `db` 服务注释掉，并调整 `codefever` 的数据库环境变量：

```yaml
  codefever:
    environment:
      DB_HOST: "your-db-host"
      DB_PORT: "3306"
      DB_USER: "root"
      DB_PASS: "your-password"
      DB_NAME: "codefever_community"
```

### 注意：数据库必须是 MySQL 8.0+

事件 worker 与 webhook 投递器用 `FOR UPDATE SKIP LOCKED` 领取任务行，
该语法自 **MySQL 8.0.1** 起才支持。在 MySQL 5.7 上这两条查询直接报语法错误，
活动流、站内信、邮件通知与 webhook 会**静默失效** —— 症状是日志里每 2 秒刷一次
`SKIP LOCKED` 报错。compose 默认已经用 `mysql:8.0`。

compose 还给数据库加了 `--default-authentication-plugin=mysql_native_password`：
MySQL 8 的默认插件 `caching_sha2_password` 不在应用容器内 MariaDB 客户端的插件目录里，
不改这一项的话 `misc/create_db.sh` 建库会直接失败。

`MySQL 8.0` 的默认 `SQL MODE` 已经满足要求，无需额外修改。
如果你坚持要用 `MySQL 5.7`，需要手动修改 `SQL MODE` 变量，否则创建数据库时会报错：

```sql
set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION';
```

### 构建自己的镜像

编排文件已经内置 `build` 段，指向仓库根目录的 `Dockerfile`：

```shell
docker compose build codefever
```

如果想推到自己的镜像仓库：

```shell
docker tag codefever-next:local you-name/codefever-next:latest
docker push you-name/codefever-next:latest
```

然后把 `CODEFEVER_IMAGE` 设成你自己的镜像名。

### 服务维护

```shell
docker compose ps                # 状态
docker compose logs -f codefever # 跟踪日志
docker compose restart codefever # 重启应用
docker compose down              # 停止（保留数据卷）
docker compose down -v           # 停止并删除数据卷（数据全部丢失）
```

服务维护请参见 [管理员设置/概览和系统服务](../admin/dashboard.md) 中的 `系统服务状态及维护` 一节

## 此文档适用条件

如果你的情况符合以下条件，你需要使用 [从零开始安装](install_from_scratch.md) 的方式安装 `CodeFever`，否则请跳过本章节继续使用 `docker-compose 安装` 方式安装。

- 学习和技术交流
- 需要做定制化修改
- `Docker 镜像安装` 不能满足处理 `Bug` 和提交 `PR` 的需求
- Docker 镜像不能在当前 `操作系统` 或 `硬件架构` 上使用

参见: [从零开始安装](install_from_scratch.md)
