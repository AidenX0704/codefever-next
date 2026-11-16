-- 仓库合并设置。
--
-- 用途：mergeBranch 的评审校验（最少通过评审人数）、合并方式白名单
-- （merge commit / squash），由 repository/config 暴露、repository/update 维护。
--
-- 设计要点：
--   1. 默认值保持**现行为**：allow_merge_commit = 1、allow_squash = 1
--      （两种方式都可用），min_reviewers = 0（沿用"全部评审人通过"的旧规则），
--      因此存量仓库在迁移后的行为与迁移前完全一致。
--   2. r_min_reviewers 的取值范围在应用层收敛为 0–10（repository/update 校验），
--      列本身用 int unsigned 留出余地。
ALTER TABLE `cc_repositories`
  ADD COLUMN `r_allow_merge_commit` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '允许 merge commit 方式合并：1 允许 0 禁止',
  ADD COLUMN `r_allow_squash` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '允许 squash 方式合并：1 允许 0 禁止',
  ADD COLUMN `r_min_reviewers` int unsigned NOT NULL DEFAULT 0 COMMENT '最少通过评审人数（0 = 全部评审人通过）';
