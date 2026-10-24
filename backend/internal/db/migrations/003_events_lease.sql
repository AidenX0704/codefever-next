-- 事件 worker 的租约列。
--
-- 原实现把事件置为 Processing 后没有超时回收：worker 在"已置 2、未置 3"
-- 之间崩溃（panic/OOM/supervisor 重启）时，这批事件会永久停在 Processing，
-- 活动流、站内信、邮件与 webhook 都会静默丢失。
--
-- 加入 ev_lease_until 后，claim 会同时领取"Processing 且租约已过期"的行，
-- 崩溃后最迟一个租约周期即可被重新消费。
ALTER TABLE `cc_events`
  ADD COLUMN `ev_lease_until` datetime NULL DEFAULT NULL,
  ADD KEY `idx_events_lease` (`ev_status`, `ev_lease_until`);
