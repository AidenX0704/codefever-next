CREATE TABLE IF NOT EXISTS `cc_repository_webhooks` (
  `rw_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `r_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `u_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_secret` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_events` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_active` tinyint(3) NOT NULL DEFAULT '1',
  `rw_created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `rw_updated` timestamp NOT NULL DEFAULT '0000-00-00 00:00:00' ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`rw_key`),
  KEY `r_key` (`r_key`),
  KEY `u_key` (`u_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `cc_repository_webhook_events` (
  `rwe_key` char(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwe_user` char(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_key` char(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwe_type` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwe_data` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwe_created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  -- 投递器状态列（Go worker 使用；历史库由 webhook.EnsureSchema 补齐）
  `rwe_status` tinyint(3) NOT NULL DEFAULT '1' COMMENT '投递状态：1 待投递 2 投递中 3 死信',
  `rwe_attempts` int(11) NOT NULL DEFAULT '0' COMMENT '已投递次数',
  `rwe_next_time` datetime NULL DEFAULT NULL COMMENT '下次可投递时间（退避到期或租约到期）',
  PRIMARY KEY (`rwe_key`),
  KEY `rw_key` (`rw_key`),
  KEY `rwe_user` (`rwe_user`),
  KEY `idx_webhook_events_status` (`rwe_status`, `rwe_next_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `cc_repository_webhook_logs` (
  `rwl_id` char(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rw_key` char(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwl_request` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwl_response` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `rwl_start` double NOT NULL,
  `rwl_end` double NOT NULL,
  `rwl_status` int(11) NOT NULL,
  `rwl_created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`rwl_id`),
  KEY `rw_key` (`rw_key`),
  KEY `rwl_created` (`rwl_created`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
