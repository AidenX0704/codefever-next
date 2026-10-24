-- 006_perf_index_members.sql
-- 2026-10 性能整改 D1：成员表上的 (u_key, 目标表主键) 组合索引。
--
-- 现有索引是 u_key / g_key（或 r_key）各自单列。查询形态恒为
-- `WHERE u_key = ? AND g_key = ?`（权限判定，见 repo/group.go）与
-- `WHERE u_key = ? AND r_key = ?`（仓库权限判定，见 repo/repository_write.go、
-- repo/admin.go），两个单列索引只能二选一，随后回表过滤。
-- 组合索引可以直接定位到唯一一行。
--
-- 说明：cc_group_members 目前还存在 rm_role / rm_status 两个明显是从
-- cc_repository_members 复制过来的列名（历史遗留），本迁移不动它们。

ALTER TABLE cc_group_members
    ADD INDEX idx_gm_user_group (u_key, g_key);

ALTER TABLE cc_repository_members
    ADD INDEX idx_rm_user_repo (u_key, r_key);
