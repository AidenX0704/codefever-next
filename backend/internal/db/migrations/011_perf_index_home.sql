-- 011_perf_index_home.sql
-- 2026-10-27 首页聚合（`/api/home/overview`）所需的索引。
--
-- 以下查询此前都没有可用索引，数据量上去后会退化成全表扫描：
--
-- 1) cc_merge_request_reviewers —— "待我评审"
--      WHERE mrr.u_key = ? AND mrr.mrr_is_review = 2 AND mrr.mrr_status = 1
--      JOIN cc_merge_requests ON mr_key
--    现有索引只有单列 mrr_key / mr_key / u_key，命中 u_key 后要逐行回表判断
--    是否已评审；组合索引可以直接定位"我还没评审的那批"。
--
-- 2) cc_merge_requests —— "我发起的"
--      WHERE u_key = ? AND mr_status = 1 AND mr_proceed_status = 1
--      ORDER BY mr_updated DESC
--    现有索引是单列。组合索引同时承担过滤与排序，EXPLAIN 里可以看到
--    `Backward index scan`，省掉 filesort（首页要 LIMIT 5，filesort 的代价是全量排序）。
--
-- 3) cc_activities —— "团队动态"（仓库维度）
--      WHERE a_relative_r_key IN (…) AND a_status = 1 ORDER BY a_created DESC LIMIT n
--    009 已建 (a_relative_r_key, a_relative_mr_key)，那是"某个 MR 的活动"用；
--    "按仓库取最近活动"需要 (a_relative_r_key, a_created)。
--
-- 4) cc_activities —— "团队动态"（仓库组维度）
--      WHERE a_relative_g_key IN (…) AND a_status = 1 ORDER BY a_created DESC LIMIT n
--    没有这条索引时，组维度只能靠单列 a_relative_g_key 再 filesort。
--
-- 说明：均为纯 ADD INDEX，不改表结构、不迁移数据，可在已部署实例上直接执行
-- （对已部署实例**必须**走 migrations，写进 backend/deploy/sql/db_structure.sql 无效）。

ALTER TABLE cc_merge_request_reviewers
    ADD INDEX idx_mrr_user_review (u_key, mrr_is_review, mr_key);

ALTER TABLE cc_merge_requests
    ADD INDEX idx_mr_creator_status (u_key, mr_proceed_status, mr_updated);

ALTER TABLE cc_activities
    ADD INDEX idx_act_repo_created (a_relative_r_key, a_created);

ALTER TABLE cc_activities
    ADD INDEX idx_act_group_created (a_relative_g_key, a_created);
