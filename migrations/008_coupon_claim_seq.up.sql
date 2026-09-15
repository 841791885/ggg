-- PRD-009 进阶 A4：优惠券支持"每人限领 N 张"（per_user_limit > 1）。
--
-- 为什么旧结构撑不住：原来 UNIQUE KEY (user_id, template_id) 的语义是"每人每模板最多一行"，
-- 等价于写死了 PerUserLimit=1。要允许领 N 张，就必须让同一 (user, template) 能存在多行——
-- 但不能无限多。解法：给每行编号 seq（第 1 张、第 2 张…），唯一键升级为三元组。
--
-- seq 从 1 开始。领取时以 COUNT(已有行)+1 作为候选 seq 插入：
--   已有 0 张 → 插 seq=1；已有 1 张 → 插 seq=2 ……
--   已有 N 张 → 插 seq=N+1 —— 若 N+1 超过 per_user_limit，应用层先拒绝（快速失败）；
--   并发下两个请求都算出 seq=N+1 时，数据库唯一键保证只有一个成功，另一个撞键→"已超限"。
-- 即：应用层校验管体验，唯一键管正确性（双保险，和默认地址那张表同一个思路）。
ALTER TABLE user_coupons
    ADD COLUMN seq INT NOT NULL DEFAULT 1 COMMENT '该用户在此模板下的第几张券（从 1 起），与 (user_id,template_id) 共同构成限额防线' AFTER template_id;

-- GORM 软删除会把 deleted_at 置为时间戳而非物理删行；唯一索引包含 deleted_at 才能让
-- "删掉再领"成为可能（NULL≠NULL 不参与唯一比较，MySQL 惯例）。与 006 中其他唯一键口径一致。
-- ⚠️ 顺序讲究：必须先建【新】索引再删【旧】索引。
-- user_fk 外键引用 (user_id) 需要一条以 user_id 开头的索引支撑；直接 DROP 旧键时
-- MySQL 发现没有替代索引会拒绝（error 1553: needed in a foreign key constraint）。
-- 新键 (user_id, template_id, seq, ...) 的最左前缀恰好能接管这个职责，先加后删就安全了。
ALTER TABLE user_coupons
    ADD UNIQUE KEY uk_user_coupons_user_template_seq (user_id, template_id, seq, deleted_at);
ALTER TABLE user_coupons DROP INDEX uk_user_coupons_user_template;
