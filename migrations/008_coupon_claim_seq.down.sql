-- 回退：恢复"每人每模板一张"的旧唯一键并丢弃 seq 列。
-- ⚠️ 若库中已存在同人同模板多行（seq>1），ADD 旧唯一键时会撞键导致回退失败——
-- 这是有意的保守设计：结构回退不该静默丢数据，需人工先清理超额券再执行。
-- 顺序与 up 相反：先建旧键（接管外键索引职责）再删新键（同 error 1553 原理）。
ALTER TABLE user_coupons ADD UNIQUE KEY uk_user_coupons_user_template (user_id, template_id, deleted_at);
ALTER TABLE user_coupons DROP INDEX uk_user_coupons_user_template_seq;
ALTER TABLE user_coupons DROP COLUMN seq;
