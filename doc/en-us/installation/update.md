# Upgrade

`CodeFever Next` is released as needed. The upgrade procedure depends on the installation
method — **always back up the `codefever-data/` directory first** (MySQL, repositories,
attachments and configuration).

## Docker Compose installation

```shell
cd codefever-next

# 1. Stop the service and back up the whole data directory (consistent MySQL files)
docker compose stop
tar czf codefever-data-$(date +%Y%m%d).tar.gz codefever-data
docker compose start

# 2. Pull the new code and rebuild the image
git pull
docker compose up -d --build

# 3. Watch the start-up log until initialisation completes
docker compose logs -f codefever
```

> Data lives in `codefever-data/` under the repository root (`mysql/`, `git-storage/`,
> `file-storage/`, `config/`). `docker compose up -d --build` leaves it alone, so data,
> repositories and admin settings are preserved. The directory is ignored by `.gitignore`;
> do not commit it. **Do not** delete `codefever-data/` manually; `docker compose down -v`
> removes containers/networks but not this bind-mounted directory.

> Schema upgrades run in the entrypoint: every start executes
> `backend/deploy/sql/db_update_*.sql` (idempotent).

## Docker image installation

A standalone container is only valid when you already have an external MySQL 8.0.x.
Prefer Compose for upgrades; if you keep the standalone container, back up first, rebuild the
image, and recreate the container with exactly the same volumes (`config`, `git-storage`,
`file-storage`) and database environment as the initial deployment. See
[Install via Docker Image](install_via_docker.md#run).

```shell
# Back up (container name may differ)
docker container exec codefever mysqldump -uroot -p'<password>' codefever_community > backup-$(date +%Y%m%d).sql

# Rebuild
cd codefever-next
git pull
docker build -f Dockerfile -t codefever-next:local .
```

> Do not use the old `docker run ... -e DB_HOST=db` example: it starts no `db` container and
> mounts no `config` volume, so admin settings are lost.

## Install from scratch

```shell
cd /data/www/codefever-next

# 1. Back up the database
mysqldump -uroot -p'<password>' codefever_community > backup-$(date +%Y%m%d).sql

# 2. Back up the configuration
cp config.yaml config.yaml.bak
cp env.yaml env.yaml.bak

# 3. Pull the new code
git pull

# 4. Rebuild the five binaries
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

# 5. Restore hooks/front-end symlinks (skip if already present)
mkdir -p deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell
for d in deploy/hooks misc/hooks git-storage/hooks; do
  ln -sfn ../../bin/codefever-hook "$d/main"
  ln -sfn main "$d/pre-receive"; ln -sfn main "$d/post-receive"; ln -sfn main "$d/update"
done
ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main
ln -sfn view/build www/app

# 6. Run schema updates (also done idempotently by create_db.sh)
cd backend/deploy/sql
sh ./create_db.sh
cd ../..

# 7. Restart services
systemctl restart codefever-api codefever-worker codefever-http-gateway
systemctl reload nginx
```

> The PHP-era `misc/update.sh` no longer exists. If the new release changes
> `config.yaml`/`env.yaml` defaults, merge them manually from the templates; do not overwrite
> production configuration.

## Database schema upgrades

Schema changes live in `backend/deploy/sql/db_update_*.sql`; `create_db.sh` runs them in
order (idempotent). To run one manually:

```shell
cd /data/www/codefever-next/backend/deploy/sql
mysql -h<host> -P<port> -u<user> -p'<password>' -D<codefever_community> \
  -e "source db_update_20220215.sql"
```

## Upgrade without keeping data

Reinstall using your preferred method: [Install From Scratch](install_from_scratch.md) or
[Install via Docker Image](install_via_docker.md).
