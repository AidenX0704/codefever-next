# 更新

`CodeFever Next` 会根据功能更新与社区反馈不定期发布版本。不同安装方式的更新步骤不同，
**更新前请务必备份 `codefever-data/` 数据目录**（含 MySQL、仓库、附件与配置）。

## Docker Compose 安装

```shell
cd codefever-next

# 1. 停服务并备份整个数据目录（保证 MySQL 数据一致）
docker compose stop
tar czf codefever-data-$(date +%Y%m%d).tar.gz codefever-data
docker compose start

# 2. 拉取新代码并重新构建镜像
git pull
docker compose up -d --build

# 3. 观察启动日志，确认初始化完成
docker compose logs -f codefever
```

> 数据默认在仓库根目录的 `codefever-data/`（`mysql/`、`git-storage/`、
> `file-storage/`、`config/`）。`docker compose up -d --build` 不会动它，数据、仓库、
> 管理后台设置都会保留。该目录已被 `.gitignore` 忽略，不要提交。
> **不要**手工删除 `codefever-data/`；`docker compose down -v` 也不会删除绑定目录，
> 但会删除容器与网络。

> 数据库结构升级由入口脚本自动执行：每次启动都会运行
> `backend/deploy/sql/db_update_*.sql`（脚本自身幂等）。

## Docker 镜像安装

单容器方式只适用于**已经有外部 MySQL 8.0.x** 的场景。更推荐直接用 Compose 更新；
如果坚持单容器，请先备份、重建镜像，并用与首次部署完全相同的 `docker run` 参数
（包括 `config` / `git-storage` / `file-storage` 三个卷和数据库环境变量）重建容器。
完整参数见 [Docker 镜像安装](install_via_docker.md#运行)。

```shell
# 备份（容器名以实际为准）
docker container exec codefever mysqldump -uroot -p'<口令>' codefever_community > backup-$(date +%Y%m%d).sql

# 重建镜像
cd codefever-next
git pull
docker build -f Dockerfile -t codefever-next:local .
```

> 不要使用本文旧版本里的 `docker run ... -e DB_HOST=db` 示例：该示例没有启动 db 容器，
> 也没有挂载 `config` 卷，会丢失管理后台设置。

## 从零开始安装

```shell
cd /data/www/codefever-next

# 1. 备份数据库
mysqldump -uroot -p'<口令>' codefever_community > backup-$(date +%Y%m%d).sql

# 2. 备份配置
cp config.yaml config.yaml.bak
cp env.yaml env.yaml.bak

# 3. 拉取新代码
git pull

# 4. 重新编译 5 个二进制
cd backend
export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod
go build ./...
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api          ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker       ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook         ./cmd/codefever-hook
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-http-gateway ./cmd/codefever-http-gateway
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-ssh-gateway  ./cmd/codefever-ssh-gateway
cd ..

# 5. 恢复 hooks / 前端软链（新版本若已存在可跳过）
mkdir -p deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell
for d in deploy/hooks misc/hooks git-storage/hooks; do
  ln -sfn ../../bin/codefever-hook "$d/main"
  ln -sfn main "$d/pre-receive"; ln -sfn main "$d/post-receive"; ln -sfn main "$d/update"
done
ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main
ln -sfn view/build www/app

# 6. 执行数据库增量（也可由 create_db.sh 幂等执行）
cd backend/deploy/sql
sh ./create_db.sh
cd ../..

# 7. 重启服务
systemctl restart codefever-api codefever-worker codefever-http-gateway
systemctl reload nginx
```

> PHP 时代的 `misc/update.sh` 已随 PHP 后端移除；现在没有 PHP 更新步骤。
> 如果新版本修改了 `config.yaml`/`env.yaml` 的默认字段，请对照
> `config.template.yaml` / `env.template.yaml` 手工合并，不要直接覆盖生产配置。

## 数据库结构升级

数据库结构变更由 `backend/deploy/sql/db_update_*.sql` 描述；`create_db.sh` 会按文件名
顺序执行所有增量脚本（幂等），通常不需要手工操作。需要单条执行时：

```shell
cd /data/www/codefever-next/backend/deploy/sql
mysql -h<host> -P<port> -u<user> -p'<password>' -D<codefever_community> \
  -e "source db_update_20220215.sql"
```

## 不保留历史数据更新

直接选择喜欢的安装方式重新安装即可，参见
[从零开始安装](install_from_scratch.md) 与 [Docker 镜像安装](install_via_docker.md)。
