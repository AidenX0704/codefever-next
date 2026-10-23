# 更新

`CodeFever Next 版本` 会根据功能更新以及社区反馈不定期进行版本更新。
不同安装方式的更新步骤不同，更新前请务必**备份数据库**。

## Docker Compose 安装

```shell
cd codefever-community

# 1. 备份数据库
docker compose exec -T db mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. 拉取新代码并重新构建镜像
git pull
docker compose up -d --build

# 3. 观察启动日志，确认初始化完成
docker compose logs -f codefever
```

> `db-data` / `git-storage` / `file-storage` 是命名卷，`docker compose up -d --build` 不会动它们，
> 数据与仓库都会保留。**不要**使用 `docker compose down -v`，那会连数据一起删掉。

## Docker 镜像安装

```shell
# 1. 备份数据库
docker container exec codefever mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. 拉取新代码并重建镜像
cd codefever-community
git pull
docker build -f Dockerfile -t codefever-next:local .

# 3. 重建容器
docker container rm -f codefever
docker container run -d --name codefever \
  -p 80:80 -p 22:22 \
  -v codefever-next-db:/var/lib/mysql \
  -v codefever-git-storage:/data/www/codefever-community/git-storage \
  -v codefever-file-storage:/data/www/codefever-community/file-storage \
  -e DB_HOST=db -e DB_PORT=3306 -e DB_USER=root -e DB_PASS=123456 \
  -e DB_NAME=codefever_community -e CONTAINER_UID=git \
  codefever-next:local
```

## 从零开始安装

```shell
cd /data/www/codefever-community

# 1. 备份数据库
mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. 拉取新代码
git pull

# 3. 重新编译 Go 服务
cd backend
export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api    ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook   ./cmd/codefever-hook
cd ..

# 4. 重新编译 Git 网关（见 install_from_scratch.md 第 7 步）
#    ...

# 5. 恢复符号链接并重启服务（见 install_from_scratch.md 第 8 步）
#    ...

systemctl restart codefever-api codefever-worker codefever-http-gateway
```

> **注意**：PHP 时代的 `misc/update.sh` 已随 PHP 后端一起移除，请按上面的步骤手动更新。

## 数据库结构升级

数据库结构变更由 `misc/db_update_*.sql` 描述。执行方式：

```shell
cd /data/www/codefever-community/misc
mysql -h<host> -P<port> -u<user> -p<password> -D<codefever_community> -e "source db_update_20220215.sql"
```

## 不保留历史数据更新

直接选择你喜欢的安装方式，参照 [从零开始安装](install_from_scratch.md) 和 [Docker 镜像安装](install_via_docker.md) 文档重新安装即可。
