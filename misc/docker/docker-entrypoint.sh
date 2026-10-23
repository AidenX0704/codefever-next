#!/bin/sh

set +u

# db
ENV_FILE=/data/www/codefever-community/env.yaml
DB_HOST=${DB_HOST:-"db"}
DB_PORT=${DB_PORT:-"3306"}
DB_USER=${DB_USER:-"root"}
DB_PASS=${DB_PASS:-"123456"}
DB_NAME=${DB_NAME:-"codefever_community"}

sed -i "s/host: localhost/host: ${DB_HOST}/" ${ENV_FILE}
sed -i "s/port: 3306/port: ${DB_PORT}/" ${ENV_FILE}
sed -i "s/username: root/username: ${DB_USER}/" ${ENV_FILE}
sed -i "s/password: 123456/password: ${DB_PASS}/" ${ENV_FILE}
sed -i "s/db: codefever_community/db: ${DB_NAME}/" ${ENV_FILE}

sleep 10

# init db
if ! mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e "use ${DB_NAME}"; then
    # sql_mode 说明：
    #   NO_AUTO_CREATE_USER —— MySQL 8.0.3+ 已移除该模式，MySQL 8 下设置会报 ERROR 1231，故移除。
    #   NO_ZERO_DATE / NO_ZERO_IN_DATE —— 移除以保留本项目的零日期存储语义。
    #     cc_* 表的 *_updated / *_deleted / mr_merge_time 等列显式使用 '0000-00-00 00:00:00'，
    #     且该值在 API 响应里必须是 -62169984000（非 Go 的 time.Time 零值 -62135596800），
    #     Go 侧为此做了显式映射，见 backend/internal/domain/group.go 与 repository.go。
    #     实测：MySQL 5.7 与 8.0 在放开该模式后，对同一份 DDL 与种子数据的隐式默认与存储值完全一致。
    mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" -e"set global sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION';";
    cd /data/www/codefever-community/misc/
    sh ./create_db.sh
fi

# git-cli
sed -i 's!git: /usr/local/git/bin/git!git: /usr/bin/git!' /data/www/codefever-community/env.yaml
sed -i 's!shell: /usr/local/git/bin/git-shell!shell: /usr/bin/git-shell!' /data/www/codefever-community/env.yaml
sed -i 's!http: /usr/local/git/libexec/git-core/git-http-backend!http: /usr/lib/git-core/git-http-backend!' /data/www/codefever-community/env.yaml

# git 钩子：git-storage 是**命名卷**，容器重建时不会被镜像刷新，
# 里面可能还留着 PHP 时代的 misc/hooks/main 副本（从旧版本升级上来时）。
# PHP 已下线，这里统一把钩子链指向 Go 的 bin/codefever-hook。
#
#   {pre-receive,post-receive,update} -> main -> ../../bin/codefever-hook
#
# 裸仓库的 hooks 目录软链目标 ../../misc/hooks 不变
# （见 backend/internal/gitsvc/write.go）—— 运行期真正生效的钩子在
# 镜像内的 misc/hooks/，本目录只是同名副本，所以缺失时直接跳过即可。
HOOK_DIR=/data/www/codefever-community/git-storage/hooks
if [ -d "${HOOK_DIR}" ]; then
    rm -f "${HOOK_DIR}/main" \
          "${HOOK_DIR}/post-receive" "${HOOK_DIR}/pre-receive" "${HOOK_DIR}/update"
    ln -s ../../bin/codefever-hook "${HOOK_DIR}/main"
    ln -s main "${HOOK_DIR}/post-receive"
    ln -s main "${HOOK_DIR}/pre-receive"
    ln -s main "${HOOK_DIR}/update"
fi

chmod -R 0777 /data/www/codefever-community/git-storage
