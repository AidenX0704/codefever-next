-- 公开 API 访问令牌表。
--
-- 用途：给外部脚本 / CI 以 `Authorization: token <明文>` 的方式访问
-- 公开 API（/api/v1），而不必使用浏览器会话 Cookie。
--
-- 设计要点：
--   1. **只存 SHA-256，不存明文** —— 明文仅在创建时返回一次；
--      校验时对传入令牌取 SHA-256 后比对（at_sha256 唯一索引）。
--   2. 令牌 ≡ 用户：令牌不带 scope，权限完全继承
--      所属用户，第一版不实现 scope（at_scope 预留为空串）。
--   3. at_last_used 记录最后一次成功使用时间，便于用户识别失效令牌。
--
-- 表名前缀 cc_ 与既有表保持一致；列命名沿用 cc_ssh_keys 的写法。
CREATE TABLE IF NOT EXISTS `cc_access_tokens` (
  `at_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `u_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `at_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `at_sha256` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `at_scope` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `at_last_used` datetime NULL DEFAULT NULL,
  `at_created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `at_updated` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `at_deleted` datetime NULL DEFAULT NULL,
  `at_status` tinyint(3) NOT NULL DEFAULT '1',
  PRIMARY KEY (`at_key`),
  UNIQUE KEY `uk_at_sha256` (`at_sha256`),
  UNIQUE KEY `uk_at_user_name` (`u_key`,`at_name`),
  KEY `idx_at_user` (`u_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='公开 API 访问令牌（只存 SHA-256）';
