#!/bin/bash
# =============================================================================
#  CodeFever Next —— 数据库初始化（幂等）
# =============================================================================
#  用法:
#      sh backend/deploy/sql/create_db.sh            # 使用 <repo>/env.yaml
#      ENV_FILE=/path/to/env.yaml sh create_db.sh    # 指定配置
#
#  行为:
#      1. 创建数据库（IF NOT EXISTS）；
#      2. 首次初始化时导入表结构与种子数据；
#      3. 每次运行都执行增量更新（db_update_*.sql 自身是幂等的）。
#
#  与旧版 misc/create_db.sh 的差异：不再依赖调用者的当前目录、不再用
#  `grep host:` 这种会跨 section 误匹配的解析方式、不再要求 UID==0、
#  并且在数据库已存在但表缺失时会补建，而不是默默跳过初始化。
# =============================================================================
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# env.yaml 默认位于仓库根目录（源码: backend/deploy/sql 上三级；
# 镜像运行时: <base>/deploy/sql 上两级）。
if [ -z "${ENV_FILE:-}" ]; then
    if [ -f "$SCRIPT_DIR/../../../env.yaml" ]; then
        ENV_FILE="$SCRIPT_DIR/../../../env.yaml"
    else
        ENV_FILE="$SCRIPT_DIR/../../env.yaml"
    fi
fi
if [ ! -f "$ENV_FILE" ]; then
    echo "错误: 找不到 env.yaml ($ENV_FILE)" >&2
    echo "请先执行: cp env.template.yaml env.yaml" >&2
    exit 1
fi
# 去掉 Windows 检出可能残留的 CR。
ENV_FILE_CONTENT=$(tr -d '\r' < "$ENV_FILE")

# 只解析 mysql: 段，避免匹配到其它段里的同名字段。
mysql_value() {
    printf '%s\n' "$ENV_FILE_CONTENT" |
        sed -n '/^mysql:[[:space:]]*$/,/^[^[:space:]]/p' |
        sed -n "s/^[[:space:]]*$1:[[:space:]]*//p" |
        head -n 1 |
        sed "s/^[\"']//; s/[\"']$//"
}

DB_HOST=$(mysql_value host)
DB_PORT=$(mysql_value port)
DB_USER=$(mysql_value username)
DB_PASS=$(mysql_value password)
DB_NAME=$(mysql_value db)

DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-3306}
DB_USER=${DB_USER:-root}
DB_PASS=${DB_PASS:-123456}
DB_NAME=${DB_NAME:-codefever_community}

case "$DB_NAME" in
    *[!A-Za-z0-9_]*|'')
        echo "错误: 非法的数据库名: $DB_NAME" >&2
        exit 1
        ;;
esac

if ! command -v mysql >/dev/null 2>&1; then
    echo "错误: 找不到 mysql 客户端" >&2
    exit 1
fi

# 用 MYSQL_PWD 而不是 -p<password>，避免口令出现在进程列表里。
export MYSQL_PWD="$DB_PASS"
mysql_run() {
    mysql --default-character-set=utf8mb4 -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" "$@"
}

echo "==> 创建数据库 $DB_NAME（如不存在）"
mysql_run -e "CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

table_exists() {
    count=$(mysql_run -N -B -e \
        "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$DB_NAME' AND table_name='$1';")
    [ "$count" != "0" ]
}

if table_exists cc_users; then
    echo "==> 数据库已初始化，跳过表结构与种子数据"
else
    echo "==> 导入表结构"
    mysql_run -D"$DB_NAME" < "$SCRIPT_DIR/db_structure.sql"

    echo "==> 导入默认数据"
    mysql_run -D"$DB_NAME" < "$SCRIPT_DIR/db_data.sql"
fi

echo "==> 执行增量更新"
for f in "$SCRIPT_DIR"/db_update_*.sql; do
    [ -f "$f" ] || continue
    echo "    - $(basename "$f")"
    mysql_run -D"$DB_NAME" < "$f"
done

echo
echo '=== 初始化完成 ==='
echo '1. 默认登录邮箱: root@codefever.cn'
echo '2. 默认口令:     123456'
echo '3. 首次登录后请立即修改口令并绑定 MFA。'
