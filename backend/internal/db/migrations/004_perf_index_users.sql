-- 004_perf_index_users.sql
-- 2026-10 性能整改 D1：cc_users 缺失索引。
--
-- 背景（已与线上 information_schema.STATISTICS 逐列核对）：
--   * u_tel 完全没有任何索引 → 手机号注册/登录走全表扫描；
--   * u_status 无索引 → 管理端按状态分类的 COUNT(*) 全表扫描。
--
-- 拆成独立文件的原因：迁移器把每个文件作为一条记录写入
-- cc_schema_migrations（internal/db/db.go），而 MySQL 的 DDL 会隐式提交，
-- 事务对它并不原子。一表一文件可以把"部分成功后失败"的恢复代价降到最低：
-- 重跑时只会重试失败的那个文件，不会撞上先前的 "Duplicate key name"。

ALTER TABLE cc_users
    ADD INDEX idx_users_tel (u_tel),
    ADD INDEX idx_users_status (u_status);
