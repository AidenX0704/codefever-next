# =============================================================================
#  CodeFever Next —— 容器镜像（全部从本地源码构建）
# =============================================================================
#  用法 / Usage:
#      docker build -t codefever-next:local .
#      docker build \
#          --build-arg BASE_RUNTIME=webdevops/php-nginx:7.4 \
#          --build-arg BASE_GO=golang:1.25-bookworm \
#          -t codefever-next:local .
#
#  基础镜像使用未限定 tag：本机已有同名镜像时直接复用，缺失时才访问
#  Docker Hub。若 Docker Hub 超时，先用 scripts/build-local.ps1（支持
#  -Registry 镜像源）拉取并打本地 tag，再执行 docker compose build。
#
#  设计要点：
#    1. 单一 Go module（backend/）构建 5 个二进制：api / worker / hook /
#       http-gateway / ssh-gateway。不再有 GOPATH 相对导入与构建期 sed 改写。
#    2. 运行时仍为多进程（supervisor 管理）：网关与 API 隔离，SSH forced
#       command 不常驻。源码合并 ≠ 进程合并。
#    3. 不 COPY PHP 源码，并在构建末段显式删除基础镜像自带的 PHP 运行时。
#    4. 运行时数据目录（git-storage / file-storage）不来自仓库，由本文件
#       与容器入口在启动时创建；命名卷首次创建时会继承这里的目录骨架。
#    5. 行尾统一在构建期规范化，兼容 Windows 检出的 CRLF。
#
#  国内网络可用 build-arg 覆盖：
#      --build-arg BASE_RUNTIME=docker.1ms.run/webdevops/php-nginx:7.4
#      --build-arg BASE_GO=docker.1ms.run/library/golang:1.25-bookworm
#      --build-arg APT_MIRROR=https://mirrors.aliyun.com/debian-archive
#      --build-arg GOPROXY=https://mirrors.aliyun.com/goproxy,direct
# =============================================================================

ARG BASE_RUNTIME=webdevops/php-nginx:7.4
ARG BASE_GO=golang:1.25-bookworm

# ------------------------------------------------------------ Stage 1: 编译 --
FROM ${BASE_GO} AS gobuilder

ARG GOPROXY=https://proxy.golang.org,direct

ENV GOPROXY=${GOPROXY} \
    GOFLAGS=-mod=mod \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0

WORKDIR /build
# 先单独拉依赖，让依赖层可缓存（源码变更不会触发重新下载）。
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/
RUN set -eux; \
    cd backend; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-api             ./cmd/codefever-api; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-worker          ./cmd/codefever-worker; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-hook            ./cmd/codefever-hook; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-http-gateway    ./cmd/codefever-http-gateway; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-ssh-gateway     ./cmd/codefever-ssh-gateway

# ------------------------------------------------------------ Stage 2: 运行 --
FROM ${BASE_RUNTIME} AS app

ARG APT_MIRROR=https://archive.debian.org

LABEL org.opencontainers.image.title="CodeFever Next" \
      org.opencontainers.image.source="https://github.com/AidenX0704/codefever-next" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.description="CodeFever Next — Git hosting platform built from local source (Go backend, no PHP runtime)"

# -- 1) Debian buster 已 EOL：默认指向官方归档源；国内可覆盖 APT_MIRROR ----------
RUN set -eux; \
    printf '%s\n' \
      "deb ${APT_MIRROR}/debian buster main contrib non-free" \
      "deb ${APT_MIRROR}/debian-security buster/updates main contrib non-free" \
      > /etc/apt/sources.list; \
    rm -f /etc/apt/sources.list.d/*.list; \
    printf '%s\n' \
      'Acquire::Check-Valid-Until "false";' \
      'Acquire::AllowInsecureRepositories "true";' \
      'Acquire::Retries "5";' \
      > /etc/apt/apt.conf.d/99codefever-mirror; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates mariadb-client openssh-server zip; \
    rm -rf /var/lib/apt/lists/*

# -- 2) SSH 主机密钥 ---------------------------------------------------------
RUN set -eux; ssh-keygen -A

# -- 3) 应用源码 -------------------------------------------------------------
#    只保留前端静态产物（www/view/build）与 www/static；PHP 源码不在仓库里。
#    backend/deploy 收敛了数据库脚本、nginx/supervisor 模板与入口脚本。
WORKDIR /data/www/codefever-community
COPY www/ ./www/
# 应用内文档中心（/doc/zh-cn、/doc/en-us）从 <BASE>/doc 读取文件。
COPY doc/ ./doc/
COPY LICENSE ./LICENSE
COPY backend/deploy/ ./deploy/
COPY config.template.yaml env.template.yaml ./
RUN set -eux; \
    # PHP 前端控制器在二次开源时已删除；这里兜底清理旧构建上下文残留。
    rm -f ./www/index.php

# -- 4) nginx / supervisor / 容器入口 ----------------------------------------
COPY backend/deploy/docker/vhost-go.conf-template /opt/docker/etc/nginx/vhost.conf
COPY backend/deploy/docker/supervisor-codefever-http-gateway.conf /opt/docker/etc/supervisor.d/codefever-http-gateway.conf
COPY backend/deploy/docker/supervisor-codefever-api.conf /opt/docker/etc/supervisor.d/codefever-api.conf
COPY backend/deploy/docker/supervisor-codefever-worker.conf /opt/docker/etc/supervisor.d/codefever-worker.conf
COPY backend/deploy/docker/docker-entrypoint.sh /opt/docker/provision/entrypoint.d/20-codefever.sh

# -- 5) 行尾规范化 -----------------------------------------------------------
#    Windows 检出时 git core.autocrlf=true 会把文本转成 CRLF；容器内 sh 会报
#    `$'\r': command not found`。必须早于二进制拷入（sed 处理二进制会损坏它）。
RUN set -eux; \
    cd /data/www/codefever-community; \
    find . -type f \( \
        -name '*.sh' -o -name '*.yaml' -o -name '*.yml' -o \
        -name '*.sql' -o -name '*.template' -o -name '*.conf' \
    \) -exec sed -i 's/\r$//' {} +; \
    sed -i 's/\r$//' \
        /opt/docker/etc/nginx/vhost.conf \
        /opt/docker/etc/supervisor.d/codefever-http-gateway.conf \
        /opt/docker/etc/supervisor.d/codefever-api.conf \
        /opt/docker/etc/supervisor.d/codefever-worker.conf \
        /opt/docker/provision/entrypoint.d/20-codefever.sh

# -- 6) 二进制落位 -----------------------------------------------------------
#    全部放在 <BASE>/bin/，共享同一份 env.yaml 与同一个 gateway.token。
COPY --from=gobuilder /out/codefever-api          /data/www/codefever-community/bin/codefever-api
COPY --from=gobuilder /out/codefever-worker       /data/www/codefever-community/bin/codefever-worker
COPY --from=gobuilder /out/codefever-hook         /data/www/codefever-community/bin/codefever-hook
COPY --from=gobuilder /out/codefever-http-gateway /data/www/codefever-community/bin/codefever-http-gateway
COPY --from=gobuilder /out/codefever-ssh-gateway  /data/www/codefever-community/bin/codefever-ssh-gateway
RUN set -eux; \
    chmod +x /data/www/codefever-community/bin/*; \
    for f in /data/www/codefever-community/bin/*; do \
        test "$(head -c 4 "$f" | od -An -tx1 | tr -d ' \n')" = '7f454c46'; \
    done

# -- 7) 软链骨架 -------------------------------------------------------------
#    - www/app -> view/build（前端静态产物）
#    - deploy/hooks      -> 新裸仓库的 hooks 目标（../../deploy/hooks）
#    - misc/hooks        -> 兼容旧裸仓库的 hooks -> ../../misc/hooks
#    - git-storage/hooks -> 命名卷初始化用的骨架
#    - ssh-gateway/shell/main -> 兼容旧 authorized_keys 里的 forced command
RUN set -eux; \
    cd /data/www/codefever-community; \
    rm -f www/app; \
    ln -s view/build www/app; \
    mkdir -p deploy/hooks misc/hooks git-storage/hooks ssh-gateway/shell file-storage; \
    for d in deploy/hooks misc/hooks git-storage/hooks; do \
        rm -f "$d/main" "$d/pre-receive" "$d/post-receive" "$d/update"; \
        ln -s ../../bin/codefever-hook "$d/main"; \
        ln -s main "$d/pre-receive"; \
        ln -s main "$d/post-receive"; \
        ln -s main "$d/update"; \
    done; \
    ln -sfn ../../bin/codefever-ssh-gateway ssh-gateway/shell/main

# -- 8) 用户 / 配置 / 权限 ---------------------------------------------------
RUN set -eux; \
    id -u git >/dev/null 2>&1 || useradd -rm git; \
    cd /data/www/codefever-community; \
    cp ./deploy/codefever-service-template /etc/init.d/codefever; \
    chmod +x /etc/init.d/codefever; \
    mkdir -p config; \
    cp ./config.template.yaml ./config/config.yaml; \
    cp ./env.template.yaml ./config/env.yaml; \
    ln -sfn config/config.yaml ./config.yaml; \
    ln -sfn config/env.yaml ./env.yaml; \
    KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"; \
    TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"; \
    sed -i "s|key: <session_key>|key: ${KEY}|" config/env.yaml; \
    sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" config/env.yaml; \
    sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" config/env.yaml; \
    chown root:git ./config/config.yaml ./config/env.yaml; \
    chmod 0660 ./config/config.yaml ./config/env.yaml; \
    chown root:git ./config; \
    chmod 0770 ./config; \
    chmod 0777 ./git-storage ./file-storage; \
    chown -R git:git ./git-storage ./file-storage
# 注：deploy/hooks 与 git-storage/hooks 里的 main 是指向 ../../bin/codefever-hook
# 的软链；chmod/chown 默认不跟随软链，真实二进制保持 root:root 755，git 用户可执行。

# -- 9) 启用 ssh 服务 --------------------------------------------------------
RUN set -eux; \
    chmod +x /opt/docker/provision/entrypoint.d/20-codefever.sh; \
    docker-service enable ssh

# -- 10) 移除 PHP 运行时 ----------------------------------------------------
#    基础镜像 webdevops/php-nginx:7.4 自带 PHP 7.4 / php-fpm / 相关入口与
#    supervisor 配置。PHP 后端下线后这些都是死代码，显式删除并在构建期自检。
RUN set -eux; \
    rm -rf \
        /usr/local/bin/php \
        /usr/local/bin/php-config \
        /usr/local/bin/php-fpm \
        /usr/local/bin/phpize \
        /usr/local/bin/docker-php-entrypoint \
        /usr/local/bin/docker-php-ext-configure \
        /usr/local/bin/docker-php-ext-enable \
        /usr/local/bin/docker-php-ext-install \
        /usr/local/bin/docker-php-setting \
        /usr/local/bin/docker-php-source \
        /usr/local/sbin/php-fpm \
        /usr/local/lib/php \
        /usr/local/include/php \
        /usr/local/php \
        /usr/local/etc/php \
        /usr/local/etc/php-fpm.conf \
        /usr/local/etc/php-fpm.conf.default \
        /usr/local/etc/php-fpm.d \
        /opt/docker/etc/php \
        /opt/docker/bin/service.d/php-fpm.sh \
        /opt/docker/bin/service.d/php-fpm.d \
        /opt/docker/etc/supervisor.d/php-fpm.conf \
        /opt/docker/etc/supervisor.d/cron.conf \
        /opt/docker/provision/entrypoint.d/20-php.sh \
        /opt/docker/provision/entrypoint.d/20-php-fpm.sh \
        /usr/src/php.tar.xz \
        /usr/src/php.tar.xz.asc; \
    if command -v php >/dev/null 2>&1 || command -v php-fpm >/dev/null 2>&1; then \
        echo "错误: 镜像内仍存在 PHP 可执行文件" >&2; exit 1; \
    fi; \
    for p in /usr/local/lib/php /usr/local/include/php /usr/local/php \
             /usr/local/etc/php /usr/local/etc/php-fpm.conf /opt/docker/etc/php \
             /usr/src/php.tar.xz; do \
        if [ -e "$p" ]; then echo "错误: 仍存在 $p" >&2; exit 1; fi; \
    done; \
    if [ -e /opt/docker/etc/supervisor.d/php-fpm.conf ]; then \
        echo "错误: supervisor 仍会拉起 php-fpm" >&2; exit 1; \
    fi; \
    if [ -e /opt/docker/etc/supervisor.d/cron.conf ]; then \
        echo "错误: crond 仅为 PHP 调度器而存在, 不应保留" >&2; exit 1; \
    fi

EXPOSE 80 22
WORKDIR /data/www/codefever-community
