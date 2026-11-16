-- 合并请求评论。
--
-- 用途：GET /api/repository/mergeRequestComments 与
-- POST /api/repository/addMergeRequestComment。
--
-- 设计要点：
--   1. mr_id 照抄 cc_merge_requests 的**主键** mr_key 的类型
--      （varchar(32)，utf8mb4_unicode_ci）—— 端点参数 mergeRequestId
--      传的就是 mr_key，与 mergeRequestClose 等既有端点一致。
--   2. c_id 用 bigint unsigned 自增：评论是高频追加数据，int 不够保险。
--   3. (mr_id, c_id) 复合索引同时服务"按 MR 拉评论"与"按 c_id asc 排序"，
--      单条 MR 的评论可以直接走索引顺序扫描。
--   4. 不设软删除列：第一版评论不可编辑/删除，保持表结构最小。
CREATE TABLE IF NOT EXISTS `cc_merge_request_comments` (
  `c_id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `mr_id` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '合并请求主键（cc_merge_requests.mr_key）',
  `u_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '评论人主键',
  `c_body` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '评论正文',
  `c_created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '评论时间',
  PRIMARY KEY (`c_id`),
  KEY `idx_mrc_mr` (`mr_id`, `c_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='合并请求评论';
