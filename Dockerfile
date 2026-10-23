# =============================================================================
#  CodeFever Next —— 本地镜像构建 (local / offline-capable image build)
# =============================================================================
#  用法 / Usage:
#      docker build -f Dockerfile.local -t codefever-next:local .
#      docker build -f Dockerfile.local \
#          --build-arg REGISTRY=mirror.gcr.io \
#          -t codefever-next:local .
#
#  与上游 Dockerfile 的关键差异 / Key differences vs. the upstream Dockerfile:
#    1. 基础镜像走本地镜像仓库 (默认 docker.1ms.run), 避免直连 Docker Hub 失败
#    2. Debian buster 已 EOL (2024-06) -> apt 源改指 mirrors.aliyun.com/debian-archive
#    3. Go 网关改为多阶段构建 + module 模式
#       (上游用 GOPATH 模式: Go >= 1.22 已移除该模式, 且 GOPATH 导入路径不允许连字符)
#    4. 直接 COPY 本地源码, 构建期不再 git clone (离线可复现)
#    5. 行尾规范化 (Windows CRLF)
#    6. 恢复 git 符号链接 (Windows 检出时 symlink 会退化成普通文本文件)
#    7. 去掉运行时不需要的 Node.js (前端产物已随仓库提交, 见 www/view/build)
#    8. **移除 PHP 运行时**: PHP 后端下线后, 基础镜像自带的 php-fpm 与
#       PHP 解释器全部是死代码, 构建时显式删除 (见第 11 步)。
#       因此本镜像运行时**不存在任何 PHP 进程、PHP 可执行文件与 PHP 配置**,
#       application/ 与 system/ 也不再进镜像。
# =============================================================================

ARG REGISTRY=docker.1ms.run
# 运行期基础镜像。
#
# 沿用 webdevops/php-nginx:7.4 —— 它的 nginx / supervisor / sshd / 容器入口
# 集成正是本项目需要的。但**PHP 部分在第 11 步被整体删除**，
# 因此最终镜像里没有任何 PHP。
ARG BASE_RUNTIME=${REGISTRY}/webdevops/php-nginx:7.4
ARG BASE_GO=${REGISTRY}/library/golang:1.25-bookworm

# --------------------------------------------------------- Stage 1: Go 网关 --
FROM ${BASE_GO} AS gobuilder

ARG GO_YAML_VERSION=v2.4.0
# GODEBUG=http2client=0 —— 强制 Go 的模块下载走 HTTP/1.1。
# 实测: 国内 goproxy 镜像的 HTTP/2 连接会**挂起且不超时** (goroutine 停在
# net/http2ClientConn.roundTrip, 模块缓存零增长), 构建期表现为无限等待。
# 改用 HTTP/1.1 后同一镜像立即完成下载。Go 的模块代理对 HTTP/1.1 支持完整,
# 因此这里直接关掉 HTTP/2 客户端, 换取构建的确定性。
ENV GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
    GOSUMDB=off \
    GOFLAGS=-mod=mod \
    GOTOOLCHAIN=local \
    GODEBUG=http2client=0 \
    CGO_ENABLED=0

WORKDIR /build
COPY http-gateway/ ./http-gateway/
COPY ssh-gateway/ ./ssh-gateway/

# 上游使用相对导入 ("./Request", "../JsonHttpClient") + GOPATH 模式。
# 现代 Go 已不支持 GOPATH 模式, 这里在构建期转换为 module 模式 (不改动仓库源码)。
RUN set -eux; \
    printf 'module codefever\n\ngo 1.21\n\nrequire gopkg.in/yaml.v2 %s\n' "${GO_YAML_VERSION}" > go.mod; \
    sed -i 's#"\./Request"#"codefever/http-gateway/Request"#' http-gateway/main.go; \
    sed -i 's#"\.\./JsonHttpClient"#"codefever/http-gateway/JsonHttpClient"#' http-gateway/Request/main.go; \
    sed -i 's#"\.\./JsonHttpClient"#"codefever/ssh-gateway/JsonHttpClient"#' ssh-gateway/shell/main.go; \
    cd http-gateway && go build -trimpath -ldflags="-s -w" -o /out/http-gateway .; \
    cd ../ssh-gateway/shell && go build -trimpath -ldflags="-s -w" -o /out/ssh-shell .

# ------------------------------------------------ Stage 1b: Go 后端 (新实现) --
# 新后端是 Go module 项目 (backend/go.mod)，与上面的 GOPATH 改造无关，
# 因此单独一个阶段构建，产物为三个静态二进制（api / worker / git 钩子）。
FROM ${BASE_GO} AS gobackend

ENV GOPROXY=https://mirrors.aliyun.com/goproxy,direct \
    GOSUMDB=off \
    GOFLAGS=-mod=mod \
    GOTOOLCHAIN=local \
    GODEBUG=http2client=0 \
    CGO_ENABLED=0

WORKDIR /build
COPY backend/go.mod backend/go.sum ./backend/
# 先单独拉依赖，让依赖层可缓存（源码变更不会触发重新下载）。
RUN cd backend && go mod download

COPY backend/ ./backend/
RUN set -eux; \
    cd backend; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-api ./cmd/codefever-api; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-worker ./cmd/codefever-worker; \
    go build -trimpath -ldflags="-s -w" -o /out/codefever-hook ./cmd/codefever-hook

# ------------------------------------------------------------ Stage 2: 应用 --
FROM ${BASE_RUNTIME} AS app

LABEL org.opencontainers.image.title="CodeFever Next" \
      org.opencontainers.image.source="https://github.com/AidenX0704/codefever-next/codefever-next" \
      org.opencontainers.image.version="local" \
      org.opencontainers.image.description="CodeFever Next — Git hosting platform built from local source (Go backend, no PHP runtime)"

# -- 1) Debian buster 已 EOL -> apt 源切换到阿里云归档源 ----------------------
RUN set -eux; \
    printf '%s\n' \
      'deb https://mirrors.aliyun.com/debian-archive/debian buster main contrib non-free' \
      'deb https://mirrors.aliyun.com/debian-archive/debian-security buster/updates main contrib non-free' \
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
RUN set -eux; \
    ssh-keygen -A

# -- 4) 应用源码 (本地 COPY, 不在构建期联网 clone) ---------------------------
#    注意: PHP 运行时已下线, 因此 **不** COPY application/ 与 system/,
#    也不 COPY www/index.php (CodeIgniter 前端控制器)。
#    只保留前端静态产物 (www/view/build) 与 www/static。
WORKDIR /data/www/codefever-community
COPY www/ ./www/
COPY misc/ ./misc/
COPY git-storage/ ./git-storage/
COPY config.template.yaml env.template.yaml ./
# www/index.php 是 PHP 前端控制器, 这里显式移除。
RUN set -eux; \
    rm -f /data/www/codefever-community/www/index.php

# -- 5) nginx / supervisor / 容器入口配置 -------------------------------------
#    只有 Go 版站点模板: PHP 后端已下线, 不再需要 php-fpm 的 upstream。
COPY misc/docker/vhost-go.conf-template /opt/docker/etc/nginx/vhost.conf
COPY misc/docker/supervisor-codefever-http-gateway.conf /opt/docker/etc/supervisor.d/codefever-http-gateway.conf
COPY misc/docker/supervisor-codefever-api.conf /opt/docker/etc/supervisor.d/codefever-api.conf
COPY misc/docker/supervisor-codefever-worker.conf /opt/docker/etc/supervisor.d/codefever-worker.conf
COPY misc/docker/docker-entrypoint.sh /opt/docker/provision/entrypoint.d/20-codefever.sh

# -- 6) 行尾规范化 (Windows 检出时 git core.autocrlf=true 会把文本转成 CRLF,
#        容器内 sh 脚本会报 `$'\r': command not found`,
#        且 misc/create_db.sh 用 grep|awk 解析 env.yaml 时会把 \r 带进库名)
#        必须早于 Go 二进制拷入 —— 二进制一旦被 sed 处理就会损坏
#        (表现为运行期 SIGILL)。misc/hooks/main 与 git-storage/hooks/main
#        **不再由 sed 处理**：它们现在是 bin/codefever-hook 的符号链接，
#        真实内容是二进制，必须原样保留（见第 8 步）。 -------------------
RUN set -eux; \
    cd /data/www/codefever-community; \
    find . -type f \( \
        -name '*.sh' -o -name '*.yaml' -o -name '*.yml' -o \
        -name '*.sql' -o -name '*.template' \
    \) -exec sed -i 's/\r$//' {} +; \
    sed -i 's/\r$//' \
        /opt/docker/etc/nginx/vhost.conf \
        /opt/docker/etc/supervisor.d/codefever-http-gateway.conf \
        /opt/docker/etc/supervisor.d/codefever-api.conf \
        /opt/docker/etc/supervisor.d/codefever-worker.conf \
        /opt/docker/provision/entrypoint.d/20-codefever.sh

# -- 7b. Go 后端二进制 (替换 PHP 后端的新实现) ------------------------------
# 落位约定：<BASE>/bin/ 下。BASE 是 env.yaml 所在目录 —— Go 后端通过
# CODEFEVER_BASE_DIR 读取同一份 env.yaml，因此与 PHP 后端、Go 网关、
# git 钩子共享同一套配置与同一个 gateway.token。
COPY --from=gobuilder /out/http-gateway /data/www/codefever-community/http-gateway/main
COPY --from=gobuilder /out/ssh-shell   /data/www/codefever-community/ssh-gateway/shell/main
# 完整性自检: 必须是可执行的 ELF, 且不能被上面的 sed 破坏
RUN set -eux; \
    chmod +x /data/www/codefever-community/http-gateway/main \
             /data/www/codefever-community/ssh-gateway/shell/main; \
    test "$(head -c 4 /data/www/codefever-community/http-gateway/main | od -An -tx1 | tr -d ' \n')" = '7f454c46'; \
    test "$(head -c 4 /data/www/codefever-community/ssh-gateway/shell/main | od -An -tx1 | tr -d ' \n')" = '7f454c46'

# -- 7c. Go 后端二进制 (替换 PHP 后端的新实现) ------------------------------
COPY --from=gobackend /out/codefever-api    /data/www/codefever-community/bin/codefever-api
COPY --from=gobackend /out/codefever-worker /data/www/codefever-community/bin/codefever-worker
# git 钩子可执行体: 替代原 misc/hooks/main 的 PHP CLI 脚本。
# PHP 一删, 钩子就无法执行, 所有 git push 都会被拒 —— 它是"移除 PHP 运行时"
# 的第一块拼图。钩子从自身所在目录逐级向上找 env.yaml, 因此放在 <BASE>/bin/
# 下同样能定位到与后端同一份配置 (gateway.token / gateway.hooks)。
COPY --from=gobackend /out/codefever-hook   /data/www/codefever-community/bin/codefever-hook
RUN set -eux; \
    chmod +x /data/www/codefever-community/bin/codefever-api \
             /data/www/codefever-community/bin/codefever-worker \
             /data/www/codefever-community/bin/codefever-hook; \
    test "$(head -c 4 /data/www/codefever-community/bin/codefever-api | od -An -tx1 | tr -d ' \n')" = '7f454c46'; \
    test "$(head -c 4 /data/www/codefever-community/bin/codefever-worker | od -An -tx1 | tr -d ' \n')" = '7f454c46'; \
    test "$(head -c 4 /data/www/codefever-community/bin/codefever-hook | od -An -tx1 | tr -d ' \n')" = '7f454c46'

# -- 8) 恢复 git 符号链接 ----------------------------------------------------
#    Windows 上检出时 symlink 会退化成普通文本文件, 必须重建, 否则:
#      - www/app -> view/build 缺失 => 前端页面与 /static 资源 404
#      - */hooks/* -> main 缺失   => git push 钩子不生效
#    钩子链: {pre-receive,post-receive,update} -> main -> ../../bin/codefever-hook
#    (裸仓库的 hooks 目录软链目标 ../../misc/hooks 不变, 见 backend/internal/gitsvc/write.go)
#
#    注意: 仓库侧已不再保留 git-storage/hooks/* 的脚本本体 (历史上是 PHP 钩子的副本,
#    PHP 下线后已无作用), 只保留 git-storage/hooks/.gitkeep 作为目录骨架。
#    下面的 ln -s 会从零重建这 4 个软链, 因此不依赖仓库中的任何源文件。
RUN set -eux; \
    cd /data/www/codefever-community; \
    rm -f www/app \
          git-storage/hooks/main git-storage/hooks/post-receive git-storage/hooks/pre-receive git-storage/hooks/update \
          misc/hooks/main misc/hooks/post-receive misc/hooks/pre-receive misc/hooks/update; \
    mkdir -p git-storage/hooks; \
    ln -s view/build www/app; \
    ln -s ../../bin/codefever-hook git-storage/hooks/main; \
    ln -s main git-storage/hooks/post-receive; \
    ln -s main git-storage/hooks/pre-receive; \
    ln -s main git-storage/hooks/update; \
    ln -s ../../bin/codefever-hook misc/hooks/main; \
    ln -s main misc/hooks/post-receive; \
    ln -s main misc/hooks/pre-receive; \
    ln -s main misc/hooks/update

# -- 9) 用户 / 目录 / 权限 --------------------------------------------------
RUN set -eux; \
    useradd -rm git; \
    cd /data/www/codefever-community; \
    cp ./misc/codefever-service-template /etc/init.d/codefever; \
    chmod +x /etc/init.d/codefever; \
    cp ./config.template.yaml ./config.yaml; \
    cp ./env.template.yaml ./env.yaml; \
    chmod 0777 ./config.yaml ./env.yaml; \
    KEY="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"; \
    sed -i "s|key: <session_key>|key: ${KEY}|" env.yaml; \
    sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" env.yaml; \
    mkdir -p ./file-storage; \
    chown -R git:git ./file-storage ./misc; \
    chmod 0777 ./misc; \
    chmod -R 0777 ./git-storage; \
    chown -R git:git ./git-storage
# 注: misc/hooks/main 与 git-storage/hooks/main 现在是指向
# ../../bin/codefever-hook 的符号链接 (第 8 步), 权限位在符号链接上无意义,
# 真实二进制的 +x 已在第 7c 步设置; chown/chmod -R 默认不跟随符号链接,
# 因此 bin/codefever-hook 保持 root:root 755 —— git 用户可执行。

# -- 10) 启用 ssh 服务 -------------------------------------------------------
# 注: crond 已下线 —— 它原本只为 PHP 的定时任务调度器而存在,
#     webhook 投递现由 Go worker 内部承担 (backend/internal/webhook)。
RUN set -eux; \
    chmod +x /opt/docker/provision/entrypoint.d/20-codefever.sh; \
    docker-service enable ssh

# -- 11) 移除 PHP 运行时 -----------------------------------------------------
# 基础镜像 webdevops/php-nginx:7.4 自带 PHP 7.4 运行时、php-fpm、
# 若干启动脚本与 supervisor program。PHP 后端下线后这些全部是死代码，
# 这里显式删除, 使镜像内**不存在任何 PHP 可执行文件与配置**。
#
# 同时删掉基础镜像里 PHP 专属的入口脚本与 supervisor program,
# 否则容器启动时仍会尝试拉起 php-fpm。
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
