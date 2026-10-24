-- 009_perf_index_activity_rule.sql
-- 2026-10 性能整改 D1：活动流与保护分支规则。
--
-- 1) cc_activities
--    合并请求详情页的活动列表：
--      WHERE a_relative_r_key = ? AND a_relative_mr_key = ?
--    现有 a_relative_r_key / a_relative_mr_key 各是单列索引。
--
-- 2) cc_proteced_branch_rules（表名拼写为历史遗留，勿改）
--    受保护分支列表：
--      WHERE r_key = ? AND pbr_status = ?  ORDER BY pbr_updated
--    现有 r_key / pbr_status 各是单列索引，组合后可省掉 filesort。

ALTER TABLE cc_activities
    ADD INDEX idx_act_repo_mr (a_relative_r_key, a_relative_mr_key);

ALTER TABLE cc_proteced_branch_rules
    ADD INDEX idx_pbr_repo_status (r_key, pbr_status, pbr_updated);
