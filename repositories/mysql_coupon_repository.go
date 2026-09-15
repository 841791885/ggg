package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "ggg/models"

	"gorm.io/gorm"
)

// CreateCouponTemplate 新增优惠券模板。
func (r *MySQLRepository) CreateCouponTemplate(ctx context.Context, template model.CouponTemplate) (model.CouponTemplate, error) {
	if err := r.db.WithContext(ctx).Create(&template).Error; err != nil {
		return model.CouponTemplate{}, fmt.Errorf("创建优惠券模板：%w", err)
	}
	return template, nil
}

// GetCouponTemplate 按 ID 查询优惠券模板。
func (r *MySQLRepository) GetCouponTemplate(ctx context.Context, templateID uint64) (model.CouponTemplate, error) {
	var template model.CouponTemplate
	err := r.db.WithContext(ctx).First(&template, templateID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CouponTemplate{}, model.ErrCouponNotFound
	}
	if err != nil {
		return model.CouponTemplate{}, fmt.Errorf("查询优惠券模板：%w", err)
	}
	return template, nil
}

// ListCouponTemplates 分页查询优惠券模板，默认排上架的在前。
func (r *MySQLRepository) ListCouponTemplates(ctx context.Context, query ListCouponTemplatesQuery) (int64, []model.CouponTemplate, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	tx := r.db.WithContext(ctx).Model(&model.CouponTemplate{})
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计优惠券模板：%w", err)
	}
	var templates []model.CouponTemplate
	if err := tx.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&templates).Error; err != nil {
		return 0, nil, fmt.Errorf("查询优惠券模板列表：%w", err)
	}
	return total, templates, nil
}

// UpdateCouponTemplate 局部更新优惠券模板字段。
func (r *MySQLRepository) UpdateCouponTemplate(ctx context.Context, templateID uint64, fields UpdateCouponTemplateFields) (model.CouponTemplate, error) {
	updates := map[string]any{}
	if fields.Name != nil {
		updates["name"] = *fields.Name
	}
	if fields.ThresholdCent != nil {
		updates["threshold_cent"] = *fields.ThresholdCent
	}
	if fields.DiscountCent != nil {
		updates["discount_cent"] = *fields.DiscountCent
	}
	if fields.TotalCount != nil {
		updates["total_count"] = *fields.TotalCount
	}
	if fields.Remaining != nil {
		updates["remaining"] = *fields.Remaining
	}
	if fields.StartsAt != nil {
		updates["starts_at"] = *fields.StartsAt
	}
	if fields.EndsAt != nil {
		updates["ends_at"] = *fields.EndsAt
	}
	if fields.Status != nil {
		updates["status"] = *fields.Status
	}
	if len(updates) == 0 {
		return model.CouponTemplate{}, model.ErrInvalidCouponInput
	}
	result := r.db.WithContext(ctx).Model(&model.CouponTemplate{}).Where("id = ?", templateID).Updates(updates)
	if result.Error != nil {
		return model.CouponTemplate{}, fmt.Errorf("更新优惠券模板：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.CouponTemplate{}, model.ErrCouponNotFound
	}
	return r.GetCouponTemplate(ctx, templateID)
}

// DeleteCouponTemplate 软删除优惠券模板；历史 user_coupons 保留可审计。
func (r *MySQLRepository) DeleteCouponTemplate(ctx context.Context, templateID uint64) error {
	result := r.db.WithContext(ctx).Delete(&model.CouponTemplate{}, templateID)
	if result.Error != nil {
		return fmt.Errorf("删除优惠券模板：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrCouponNotFound
	}
	return nil
}

// CreateUserCoupon 记录用户领取的券。
func (r *MySQLRepository) CreateUserCoupon(ctx context.Context, coupon model.UserCoupon) (model.UserCoupon, error) {
	if err := r.db.WithContext(ctx).Create(&coupon).Error; err != nil {
		// 唯一键冲突表示已领过，转换为业务错误而不是 500。
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.UserCoupon{}, model.ErrCouponAlreadyClaimed
		}
		return model.UserCoupon{}, fmt.Errorf("创建用户优惠券：%w", err)
	}
	return coupon, nil
}

// GetUserCouponByTemplate 查询用户是否已持有某模板的券。
func (r *MySQLRepository) GetUserCouponByTemplate(ctx context.Context, userID, templateID uint64) (model.UserCoupon, error) {
	var coupon model.UserCoupon
	err := r.db.WithContext(ctx).Where("user_id = ? AND template_id = ?", userID, templateID).First(&coupon).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.UserCoupon{}, model.ErrCouponNotFound
	}
	if err != nil {
		return model.UserCoupon{}, fmt.Errorf("查询用户优惠券：%w", err)
	}
	return coupon, nil
}

// ListUserCoupons 查询当前用户持有的全部券。
func (r *MySQLRepository) ListUserCoupons(ctx context.Context, userID uint64) ([]model.UserCoupon, error) {
	var coupons []model.UserCoupon
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&coupons).Error; err != nil {
		return nil, fmt.Errorf("查询用户优惠券列表：%w", err)
	}
	return coupons, nil
}

// CountUserCouponsByTemplate 统计某用户在某模板下已持有的张数（service 层做限领预检用）。
func (r *MySQLRepository) CountUserCouponsByTemplate(ctx context.Context, userID, templateID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserCoupon{}).
		Where("user_id = ? AND template_id = ?", userID, templateID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计用户持券数量：%w", err)
	}
	return count, nil
}

// ClaimCouponTx 在一个事务内完成"扣发行量 + 插持有记录"，是防超发的核心（PRD-009 A4）。
//
// ── 为什么必须是一个事务？──
// 领一张券要改两张表：coupon_templates.remaining（总量 -1）和 user_coupons（+一行记录）。
// 如果分开提交：扣了量但插记录失败 → 用户的券凭空蒸发；插了记录但扣量失败 → 超发。
// 同事务保证【量减 ⇔ 券增】同生共死——和 A1 下单扣库存完全同构。
//
// ── 并发安全靠什么？两道闸门 ──
// ① remaining 条件更新：UPDATE ... WHERE remaining > 0。
//
//	"查剩余量→判断→扣减"三步里，只有扣减这条 SQL 是原子的（行锁内比较 + 修改一气呵成），
//	所以判断必须写进 WHERE，绝不能留在 Go 代码里（先查后改=超卖老路，A1 讲过三遍的那个坑）。
//
// ② (user_id, template_id, seq) 唯一键：
//
//	每人限领 N 张靠 seq 编号实现——本次的 seq = 已持有张数 +1。
//	同一用户连点两次领取，两个请求可能都算出 seq=2（各自 COUNT 时对方还没插），
//	但唯一键让第二个 INSERT 撞键失败。应用层的 count<preUserLimit 预检只是快速失败优化，
//	正确性由数据库兜底——双保险模式第四次出现（幂等键/默认地址/event_no 都是它）。
func (r *MySQLRepository) ClaimCouponTx(ctx context.Context, userID, templateID uint64, perUserLimit int) (model.UserCoupon, error) {
	var claimed model.UserCoupon
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// ── 闸门①：原子扣发行量 ──
		// 生成的 SQL：UPDATE coupon_templates SET remaining = remaining - 1 WHERE id=? AND remaining > 0
		// RowsAffected=0 只可能是 remaining 已经是 0（券被抢光）——注意这不是 error，
		// MySQL 觉得"没匹配到行"是正常结果，翻译责任在我们（A1 同款判断）。
		result := tx.Model(&model.CouponTemplate{}).
			Where("id = ? AND remaining > 0", templateID).
			UpdateColumn("remaining", gorm.Expr("remaining - 1"))
		if result.Error != nil {
			return fmt.Errorf("扣减优惠券发行量：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			return model.ErrCouponNotClaimable // 抢光了：语义上仍是"当前不可领取"
		}

		// ── 计算本次 seq（第几张）──
		// COUNT 和下面的 INSERT 在同一事务里。会不会两个事务同时 COUNT 到相同值？
		// 会——这正是唯一键存在的理由：撞键分支处理这个竞态，而不是假装 COUNT 可靠。
		var held int64
		if err := tx.Model(&model.UserCoupon{}).
			Where("user_id = ? AND template_id = ?", userID, templateID).
			Count(&held).Error; err != nil {
			return fmt.Errorf("统计用户持券数量：%w", err)
		}
		nextSeq := int(held) + 1
		if nextSeq > perUserLimit {
			// 超限走 error 返回 → 事务回滚 → 刚才那次 remaining-1 自动撤销。
			// 这就是"为什么扣量放在事务第一步也没关系"：回滚会收拾一切中途写入。
			return model.ErrCouponAlreadyClaimed
		}

		// ── 闸门②：插入持有记录，唯一键裁决并发竞争 ──
		coupon := model.UserCoupon{
			UserID: userID, TemplateID: templateID, Seq: nextSeq,
			Status: model.UserCouponUnused, ClaimedAt: time.Now().UTC(),
		}
		if err := tx.Create(&coupon).Error; err != nil {
			// TranslateError:true（database/mysql.go）已把 MySQL 1062 转成 gorm.ErrDuplicatedKey，
			// 不用再 errors.As 挖驱动错误号——项目配置替你做了。
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return model.ErrCouponAlreadyClaimed // 同人同 seq 撞键=并发双开被拦下的是你
			}
			return fmt.Errorf("写入用户优惠券：%w", err)
		}
		claimed = coupon
		return nil
	})
	if err != nil {
		return model.UserCoupon{}, err
	}
	return claimed, nil
}
