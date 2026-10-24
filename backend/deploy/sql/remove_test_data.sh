#!/bin/bash
# =============================================================================
#  删除 CodeFever 种子数据里的演示用户/仓库/组（可选）
# =============================================================================
#  仅在"全新安装后想去掉演示数据"时使用。会删除固定的演示账号与仓库，
#  不会影响你自己创建的账号。
#
#  用法:
#      sh backend/deploy/sql/remove_test_data.sh
#      ENV_FILE=/path/to/env.yaml sh remove_test_data.sh
# =============================================================================
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -z "${ENV_FILE:-}" ]; then
    if [ -f "$SCRIPT_DIR/../../../env.yaml" ]; then
        ENV_FILE="$SCRIPT_DIR/../../../env.yaml"
    else
        ENV_FILE="$SCRIPT_DIR/../../env.yaml"
    fi
fi
if [ ! -f "$ENV_FILE" ]; then
    echo "错误: 找不到 env.yaml ($ENV_FILE)" >&2
    exit 1
fi

ENV_FILE_CONTENT=$(tr -d '\r' < "$ENV_FILE")
mysql_value() {
    printf '%s\n' "$ENV_FILE_CONTENT" |
        sed -n '/^mysql:[[:space:]]*$/,/^[^[:space:]]/p' |
        sed -n "s/^[[:space:]]*$1:[[:space:]]*//p" |
        head -n 1 | sed "s/^[\"']//; s/[\"']$//"
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

export MYSQL_PWD="$DB_PASS"
mysql_run() {
    mysql --default-character-set=utf8mb4 -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -D"$DB_NAME" "$@"
}

echo "==> 删除演示用户 / 仓库 / 仓库组（数据库: $DB_NAME）"
mysql_run -e "DELETE FROM cc_ssh_keys WHERE u_key IN ('26c20714af8bc20aa85be657a5170e71','f4a9c54adef17599d4f709a1167f0fcd');"
mysql_run -e "DELETE FROM cc_repositories WHERE r_key = '2f941aef39bc8a048da55dd28c678655';"
mysql_run -e "DELETE FROM cc_groups WHERE g_key = 'e6508c1a66e86f697c96bdffb30c1ae3';"
mysql_run -e "DELETE FROM cc_users WHERE u_key IN ('26c20714af8bc20aa85be657a5170e71','f4a9c54adef17599d4f709a1167f0fcd');"

# 仅删除种子数据对应的裸仓库目录（源码/运行时两种布局都兼容）。
GIT_STORAGE="$SCRIPT_DIR/../../../git-storage"
if [ ! -d "$GIT_STORAGE" ]; then
    GIT_STORAGE="$SCRIPT_DIR/../../git-storage"
fi
rm -rf "$GIT_STORAGE/7b56eed88bda864c6e5ae1540549f8ab"

echo 'Done!'
