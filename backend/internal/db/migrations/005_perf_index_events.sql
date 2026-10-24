-- 005_perf_index_events.sql
-- 2026-10 性能整改 D1 / B8：cc_events 领取查询的排序索引。
--
-- 事件 worker 每 2 秒执行一次（internal/events/worker.go）：
--
--	SELECT … FROM cc_events
--	 WHERE (ev_status = ? OR (ev_status = ? AND (ev_lease_until IS NULL OR ev_lease_until < NOW())))
--	   AND ev_attempts < ?
--	 ORDER BY ev_created ASC
--	 LIMIT ? FOR UPDATE SKIP LOCKED
--
-- 现有索引是 ev_status、ev_type、(ev_status, ev_lease_until)，**ev_created 无索引**，
-- 因此每轮都要对候选集做一次 filesort。加上 (ev_status, ev_created) 后，
-- 领取可以走索引顺序扫描并直接满足 ORDER BY。
--
-- 注：OR 条件仍会让优化器只在部分分支上用到该索引；配合保留期清理
-- （见 010_perf_retention.sql 说明的清理任务）把候选集维持在小规模，
-- 两者一起才有效。

ALTER TABLE cc_events
    ADD INDEX idx_events_claim (ev_status, ev_created);
