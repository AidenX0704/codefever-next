#!/bin/sh
# =============================================================================
#  CodeFever Next —— 容器入口初始化
# =============================================================================
#  由 webdevops 基础镜像的 entrypoint 机制在容器启动时执行。
#
#  做五件事：
#    1. 把 docker-compose 传入的数据库参数写进 env.yaml；
#    2. 按实际暴露端口生成站点 host / ssh（前端 clone 面板直接用它们）；
#    3. 等待 MySQL 真正可用（而不是 sleep 固定秒数）；
#    4. 幂等地初始化数据库（首次建表 + 每次执行增量 SQL）；
#    5. 修复运行时目录/软链（命名卷从旧版本升级上来时尤其重要）。
#
#  配置持久化：<BASE>/config.yaml 与 env.yaml 是指向 <BASE>/config/ 的软链，
#  而 config/ 由 docker-compose 的命名卷挂载。因此管理后台修改的主机名、
#  邮件设置等在容器重建/镜像升级后仍然保留。
# =============================================================================
set -eu

BASE=/data/www/codefever-community
ENV_FILE=$BASE/env.yaml
CONFIG_FILE=$BASE/config.yaml

# 配置目录由 config-data 命名卷挂载。正常情况下镜像已经写入初始文件；
# 如果卷是手动创建的空目录，这里从模板恢复并生成随机密钥，避免容器起不来。
mkdir -p "$BASE/config"
# 从旧版本升级：配置还是普通文件时先搬进卷，避免被软链覆盖。
if [ -f "$CONFIG_FILE" ] && [ ! -L "$CONFIG_FILE" ]; then
    cp "$CONFIG_FILE" "$BASE/config/config.yaml"
fi
if [ -f "$ENV_FILE" ] && [ ! -L "$ENV_FILE" ]; then
    cp "$ENV_FILE" "$BASE/config/env.yaml"
fi
if [ ! -f "$BASE/config/config.yaml" ] && [ -f "$BASE/config.template.yaml" ]; then
    cp "$BASE/config.template.yaml" "$BASE/config/config.yaml"
fi
if [ ! -f "$BASE/config/env.yaml" ] && [ -f "$BASE/env.template.yaml" ]; then
    cp "$BASE/env.template.yaml" "$BASE/config/env.yaml"
    KEY="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
    TOKEN="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-32)"
    sed -i "s|key: <session_key>|key: ${KEY}|" "$BASE/config/env.yaml"
    sed -i "s|salt: <totp_salt_for_codefever>|salt: ${KEY}|" "$BASE/config/env.yaml"
    sed -i "s|token: <codefever_community_api_token>|token: ${TOKEN}|" "$BASE/config/env.yaml"
fi
ln -sfn config/config.yaml "$CONFIG_FILE"
ln -sfn config/env.yaml "$ENV_FILE"

if [ ! -f "$ENV_FILE" ] || [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 找不到 $ENV_FILE 或 $CONFIG_FILE" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# YAML 字段替换（不用 sed 拼值，避免口令含 / & \ 时写坏文件）
# ---------------------------------------------------------------------------

# mysql_set <key> <value>：只替换 mysql: 段里的字段。
mysql_set() {
    key=$1
    value=$2
    tmp="$ENV_FILE.tmp.$$"
    VALUE="$value" awk -v key="$key" '
        /^mysql:[ 	]*$/ { in_mysql = 1; print; next }
        /^[^ 	#]/       { in_mysql = 0 }
        {
            if (in_mysql && $0 ~ "^[ 	]*" key "[ 	]*:") {
                if (!done) {
                    print "  " key ": " ENVIRON["VALUE"]
                    done = 1
                }
                next
            }
            print
        }
        END {
            if (in_mysql && !done) print "  " key ": " ENVIRON["VALUE"]
        }
    ' "$ENV_FILE" > "$tmp"
    cat "$tmp" > "$ENV_FILE"; rm -f "$tmp"
}

# top_set <key> <value>：替换顶层字段（host / ssh）。
top_set() {
    key=$1
    value=$2
    tmp="$CONFIG_FILE.tmp.$$"
    VALUE="$value" awk -v key="$key" '
        $0 ~ "^" key "[ 	]*:" {
            if (!done) {
                print key ": " ENVIRON["VALUE"]
                done = 1
            }
            next
        }
        { print }
        END { if (!done) print key ": " ENVIRON["VALUE"] }
    ' "$CONFIG_FILE" > "$tmp"
    cat "$tmp" > "$CONFIG_FILE"; rm -f "$tmp"
}

# ---------------------------------------------------------------------------
# 1) 数据库参数（优先 CODEFEVER_MYSQL_*，兼容旧 DB_*）
# ---------------------------------------------------------------------------
DB_HOST=${CODEFEVER_MYSQL_HOST:-${DB_HOST:-"db"}}
DB_PORT=${CODEFEVER_MYSQL_PORT:-${DB_PORT:-"3306"}}
DB_USER=${CODEFEVER_MYSQL_USER:-${DB_USER:-"root"}}
DB_PASS=${CODEFEVER_MYSQL_PASSWORD:-${DB_PASS:-"123456"}}
DB_NAME=${CODEFEVER_MYSQL_DB:-${DB_NAME:-"codefever_community"}}

mysql_set host "$DB_HOST"
mysql_set port "$DB_PORT"
mysql_set username "$DB_USER"
mysql_set password "$DB_PASS"
mysql_set db "$DB_NAME"

# ---------------------------------------------------------------------------
# 2) 站点地址
#
# 前端 clone 面板直接使用 config.yaml 的 host / ssh。compose 默认把 80/22
# 映射到 9080/2222，如果这里不写端口，面板会给出错误地址。
# CODEFEVER_SITE_HOST / CODEFEVER_SITE_SSH 可覆盖（反代、域名、非默认端口）。
# ---------------------------------------------------------------------------
SITE_HTTP_PORT=${CODEFEVER_HTTP_PORT:-9080}
SITE_SSH_PORT=${CODEFEVER_SSH_PORT:-2222}
SITE_HOST=${CODEFEVER_SITE_HOST:-"http://localhost:${SITE_HTTP_PORT}"}
SITE_SSH=${CODEFEVER_SITE_SSH:-"ssh://git@localhost:${SITE_SSH_PORT}/"}

top_set host "$SITE_HOST"
top_set ssh "'$SITE_SSH'"

# ---------------------------------------------------------------------------
# 3) 等待 MySQL 可用
#
# 旧实现固定 `sleep 10`：数据库慢启动超过 10 秒时，后续建库会失败；
# 更糟的是下一次容器重启时 `use db` 成功会跳过初始化，留下空库。
# 这里改为带超时的轮询。
# ---------------------------------------------------------------------------
export MYSQL_PWD="$DB_PASS"
echo "==> 等待 MySQL ${DB_HOST}:${DB_PORT} ..."
i=0
while ! mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e 'SELECT 1' >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 90 ]; then
        echo "错误: 等待 MySQL 超时（180 秒）" >&2
        exit 1
    fi
    sleep 2
done
echo "==> MySQL 已就绪"

# sql_mode 说明：
#   NO_AUTO_CREATE_USER —— MySQL 8.0.3+ 已移除，设置会报 ERROR 1231，故不设置。
#   NO_ZERO_DATE / NO_ZERO_IN_DATE —— 移除以保留 cc_* 表的零日期语义
#     （API 响应里的 -62169984000 依赖它）。
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -e \
    "set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';"

# ---------------------------------------------------------------------------
# 4) 数据库初始化（create_db.sh 自身幂等）
# ---------------------------------------------------------------------------
ENV_FILE="$ENV_FILE" sh "$BASE/deploy/sql/create_db.sh"

# ---------------------------------------------------------------------------
# 5) git CLI 路径（env.yaml 模板可能写成上游 VM 路径）
# ---------------------------------------------------------------------------
# 仅当还是旧模板路径时替换；用户自定义路径不覆盖。
tmp="$ENV_FILE.tmp.$$"
awk '
    /^cli:[ 	]*$/ { in_cli = 1; print; next }
    /^[^ 	#]/      { in_cli = 0 }
    {
        if (in_cli && $0 ~ /^[ 	]*git:[ 	]*\/usr\/local\/git\/bin\/git[ 	]*$/) {
            print "  git: /usr/bin/git"; next
        }
        if (in_cli && $0 ~ /^[ 	]*shell:[ 	]*\/usr\/local\/git\/bin\/git-shell[ 	]*$/) {
            print "  shell: /usr/bin/git-shell"; next
        }
        if (in_cli && $0 ~ /^[ 	]*http:[ 	]*\/usr\/local\/git\/libexec\/git-core\/git-http-backend[ 	]*$/) {
            print "  http: /usr/lib/git-core/git-http-backend"; next
        }
        print
    }
' "$ENV_FILE" > "$tmp"
cat "$tmp" > "$ENV_FILE"; rm -f "$tmp"
unset tmp

# ---------------------------------------------------------------------------
# 6) 运行时目录与软链修复
#
#   钩子链：
#     <base>/deploy/hooks/{main,pre-receive,post-receive,update}
#       main -> ../../bin/codefever-hook
#
#   兼容链（旧部署 / 旧裸仓库仍在使用）：
#     <base>/misc/hooks/*          —— 旧裸仓库的 hooks -> ../../misc/hooks
#     <base>/ssh-gateway/shell/main —— 旧 authorized_keys 里的 forced command
#
#   软件更新后本脚本每次启动都会重跑，因此不会因为镜像里的软链
#   被旧命名卷覆盖而失效。
# ---------------------------------------------------------------------------
setup_hooks_dir() {
    dir=$1
    mkdir -p "$dir"
    rm -f "$dir/main" "$dir/pre-receive" "$dir/post-receive" "$dir/update"
    ln -s ../../bin/codefever-hook "$dir/main"
    ln -s main "$dir/pre-receive"
    ln -s main "$dir/post-receive"
    ln -s main "$dir/update"
}

setup_hooks_dir "$BASE/deploy/hooks"
setup_hooks_dir "$BASE/misc/hooks"
setup_hooks_dir "$BASE/git-storage/hooks"

mkdir -p "$BASE/ssh-gateway/shell"
ln -sfn ../../bin/codefever-ssh-gateway "$BASE/ssh-gateway/shell/main"

mkdir -p "$BASE/file-storage" "$BASE/git-storage"

# 配置卷的权限（命名卷首次创建时继承镜像，但旧卷/手工卷可能不对）。
if id git >/dev/null 2>&1; then
    chown root:git "$BASE/config" "$BASE/config/config.yaml" "$BASE/config/env.yaml" 2>/dev/null || true
    chmod 0770 "$BASE/config" 2>/dev/null || true
    chmod 0660 "$BASE/config/config.yaml" "$BASE/config/env.yaml" 2>/dev/null || true
fi

# 只改必要的目录，不对整个 git-storage 做递归 chmod —— 仓库很多时
# `chmod -R` 会显著拖慢容器启动。新建仓库由 API 侧递归设成 0777。
chmod 0777 "$BASE/git-storage" "$BASE/file-storage" 2>/dev/null || true
find "$BASE/git-storage" -mindepth 1 -maxdepth 1 -type d -exec chmod 0777 {} + 2>/dev/null || true

echo "==> CodeFever Next 容器初始化完成"
