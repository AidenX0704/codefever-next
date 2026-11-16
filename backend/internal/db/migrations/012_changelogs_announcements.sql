-- 站点更新日志与公告。
--
-- 用途：/api/site/changelog 与 /api/site/announcement 的数据源；
-- 管理端通过 /api/admin/{changelogSave,changelogDelete,announcementSave,announcementDelete}
-- 维护内容。
--
-- 设计要点：
--   1. 两张表都是**追加型内容表**，量级极小（几十行），不设软删除列 ——
--      删除即物理删除，避免"停用了一条 changelog 还占着列表分页"的歧义。
--   2. cl_released_at 用 DATE 而不是 datetime：发布日期是"哪一天"的概念，
--      列表也按它排序（released_at desc, cl_id desc）。
--   3. 表名前缀 cc_ 与既有表保持一致；主键用业务前缀 cl_ / an_ +
--      _id 后缀（本批新表不参与 PHP 契约，采用自增主键而非 32 位随机键）。
CREATE TABLE IF NOT EXISTS `cc_changelogs` (
  `cl_id` int unsigned NOT NULL AUTO_INCREMENT,
  `cl_version` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '版本号（如 v1.2.0）',
  `cl_title` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '标题',
  `cl_body` mediumtext COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '正文',
  `cl_released_at` date NULL DEFAULT NULL COMMENT '发布日期',
  `cl_created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`cl_id`),
  KEY `idx_cl_released` (`cl_released_at`, `cl_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='站点更新日志';

CREATE TABLE IF NOT EXISTS `cc_announcements` (
  `an_id` int unsigned NOT NULL AUTO_INCREMENT,
  `an_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'support' COMMENT '类型：security|privacy|support',
  `an_body` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '内容',
  `an_enabled` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '是否启用：1 启用 0 停用',
  `an_created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`an_id`),
  KEY `idx_an_enabled` (`an_enabled`, `an_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='站点公告';
