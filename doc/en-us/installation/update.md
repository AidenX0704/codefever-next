# Upgrade

`CodeFever Next` is released on a regular basis, driven by new features and community
feedback. The upgrade procedure differs per installation method — **always back up the
database first**.

## Docker Compose installation

```shell
cd codefever-community

# 1. Back up the database
docker compose exec -T db mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. Pull the new code and rebuild the image
git pull
docker compose up -d --build

# 3. Watch the start-up log until the initialisation is done
docker compose logs -f codefever
```

> `db-data` / `git-storage` / `file-storage` are named volumes. `docker compose up -d --build`
> leaves them alone, so your data and repositories are preserved. **Do not** run
> `docker compose down -v` — that deletes the data as well.

## Docker image installation

```shell
# 1. Back up the database
docker container exec codefever mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. Pull the new code and rebuild the image
cd codefever-community
git pull
docker build -f Dockerfile -t codefever-next:local .

# 3. Recreate the container
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

## Install from scratch

```shell
cd /data/www/codefever-community

# 1. Back up the database
mysqldump -uroot -p123456 codefever_community > backup-$(date +%Y%m%d).sql

# 2. Pull the new code
git pull

# 3. Rebuild the Go services
cd backend
export GOPROXY=https://mirrors.aliyun.com/goproxy,direct
export GOFLAGS=-mod=mod
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-api    ./cmd/codefever-api
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-worker ./cmd/codefever-worker
go build -trimpath -ldflags="-s -w" -o ../bin/codefever-hook   ./cmd/codefever-hook
cd ..

# 4. Rebuild the Git gateways (see step 7 of install_from_scratch.md)
#    ...

# 5. Restore the symlinks and restart the services (see step 8 of install_from_scratch.md)
#    ...

systemctl restart codefever-api codefever-worker codefever-http-gateway
```

> **Note**: the PHP-era `misc/update.sh` was removed together with the PHP backend — update
> manually with the steps above.

## Database schema upgrades

Schema changes are described by the `misc/db_update_*.sql` files:

```shell
cd /data/www/codefever-community/misc
mysql -h<host> -P<port> -u<user> -p<password> -D<codefever_community> -e "source db_update_20220215.sql"
```

## Upgrade without keeping the data

Simply pick your preferred installation method again and reinstall from scratch, following
[Install From Scratch](install_from_scratch.md) or
[Install via Docker Image](install_via_docker.md).
