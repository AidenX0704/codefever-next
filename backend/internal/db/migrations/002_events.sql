-- 事件队列表。
--
-- 设计要点：事件与业务写入**同一事务**落库 —— 业务写成功则事件必在，
-- 业务写回滚则事件一并消失，从根本上避免"业务失败但事件已发"的错位。
-- 后台 worker 随后消费本表并派发给各个 handler（活动流 / 通知 / webhook）。
--
-- 表名前缀 cc_ 与既有 17 张表保持一致。
CREATE TABLE IF NOT EXISTS `cc_events` (
  `ev_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `ev_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `ev_data` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `ev_generated_by` varchar(32) COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL,
  `ev_timestamp` bigint NOT NULL DEFAULT '0',
  `ev_language` varchar(8) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'zh-cn',
  `ev_status` tinyint(3) NOT NULL DEFAULT '1',
  `ev_created` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `ev_processed` datetime NULL DEFAULT NULL,
  `ev_attempts` int(11) NOT NULL DEFAULT '0',
  PRIMARY KEY (`ev_key`),
  KEY `idx_events_status` (`ev_status`),
  KEY `idx_events_type` (`ev_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='领域事件队列表，由后台 worker 消费';
