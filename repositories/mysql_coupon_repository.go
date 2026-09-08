package repositories

import (
	"context"
	"errors"
	"fmt"

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
