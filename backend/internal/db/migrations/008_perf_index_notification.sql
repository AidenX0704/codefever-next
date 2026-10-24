-- 008_perf_index_notification.sql
-- 2026-10 性能整改 D1：通知相关表的组合索引。
--
-- 1) cc_notification_refuse
--    通知 worker 的批量化查询（性能整改 C3，repo.NotificationRepo.RefusedFor）：
--
--      SELECT u_key, nr_type FROM cc_notification_refuse
--       WHERE u_key IN (…) AND nr_status = 1
--         AND ((nr_type = 1 AND g_key = ?) OR (nr_type = 2 AND r_key = ?))
--
--    两个 (u_key, nr_type, <目标列>) 组合索引分别覆盖 OR 的两个分支。
--    单用户单目标的 HasRefused 也走同一路径。
--
-- 2) cc_notification_users
--    "我的通知列表"：WHERE u_key = ? AND nu_status = ? AND nu_is_read = ?，
--    现有只有 u_key 单列索引。

ALTER TABLE cc_notification_refuse
    ADD INDEX idx_nr_user_type_group (u_key, nr_type, g_key),
    ADD INDEX idx_nr_user_type_repo (u_key, nr_type, r_key);

ALTER TABLE cc_notification_users
    ADD INDEX idx_nu_user_status_read (u_key, nu_status, nu_is_read);
