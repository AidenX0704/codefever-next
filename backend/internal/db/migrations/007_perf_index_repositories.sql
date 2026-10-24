-- 007_perf_index_repositories.sql
-- 2026-10 性能整改 D1：cc_repositories 的组合索引。
--
-- 两种高频形态：
--   g_key + r_status —— 组下仓库列表（repo/group.go、repo/notification.go）
--   u_key + r_status —— 用户自有的正常仓库（repo/repository.go 的 ListJoined
--                       第 4 个来源、以及 CountJoinedByUsers 的对应分支）
--
-- 现有 g_key / u_key / r_status 都是单列索引，组合后可直接满足过滤 + 排序前的
-- 候选集定位，避免先按单列取出再回表筛 r_status。

ALTER TABLE cc_repositories
    ADD INDEX idx_repo_group_status (g_key, r_status),
    ADD INDEX idx_repo_user_status (u_key, r_status);
