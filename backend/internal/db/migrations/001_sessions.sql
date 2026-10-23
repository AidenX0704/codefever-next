-- 会话表：替换 PHP 侧的 CI2 加密 Cookie 会话。
--
-- Cookie 中只保存一个 32 位随机 session id，其余状态在服务端。
-- 表名前缀 cc_ 与既有 17 张表保持一致（application/config/database.php 的
-- db_prefix 为 db_，而 db_structure.sql 中已显式写成 cc_）。
CREATE TABLE IF NOT EXISTS `cc_sessions` (
  `s_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `u_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `s_ip` varchar(64) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  `s_user_agent` varchar(255) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  `s_created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `s_expires` datetime NOT NULL,
  `s_last_seen` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`s_key`),
  KEY `idx_sessions_user` (`u_key`),
  KEY `idx_sessions_expires` (`s_expires`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
