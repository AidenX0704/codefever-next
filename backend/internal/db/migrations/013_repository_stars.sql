-- 仓库标星。
--
-- 用途：POST /api/repository/{star,unstar} 与 repository/list 的
-- starCount / starred / mode=starred 筛选。
--
-- 设计要点：
--   1. 键类型照抄 cc_repositories.r_key / cc_users.u_key 的真实类型
--      （varchar(32)，与 010_access_tokens.sql 的写法一致用 utf8mb4_unicode_ci）。
--   2. 主键 (u_key, r_key) 天然防重：标星是幂等操作，重复标星走
--      INSERT IGNORE 即可，不必先查后插。
--   3. 索引 (r_key) 支撑"按仓库数星星"（repository/list 与 config 的
--      starCount 批量统计）。
CREATE TABLE IF NOT EXISTS `cc_repository_stars` (
  `u_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `r_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `s_created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '标星时间',
  PRIMARY KEY (`u_key`, `r_key`),
  KEY `idx_rs_repo` (`r_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='仓库标星';
