package services

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	model "ggg/models"
	"ggg/repositories"
)

// CouponService 负责优惠券模板管理和用户领取。
// TODO(PRD-009 进阶): remaining 条件更新防超发、下单事务内占用券与优惠金额分摊未实现（见 docs/ADVANCED-TASKS.md A4/A8）。
// 本阶段领取只校验有效期和上架状态，唯一键保证每人每模板一张。
type CouponService struct{ repository repositories.CouponRepository }

// NewCouponService 创建优惠券业务服务。
func NewCouponService(repository repositories.CouponRepository) *CouponService {
	return &CouponService{repository: repository}
}

// CreateTemplateInput 表示创建优惠券模板所需的业务参数。
type CreateTemplateInput struct {
	Name          string
	ThresholdCent int64
	DiscountCent  int64
	TotalCount    int
	StartsAt      time.Time
	EndsAt        time.Time
}

// UpdateTemplateInput 表示更新优惠券模板所需的业务参数，nil 字段保持原值。
type UpdateTemplateInput struct {
	TemplateID    uint64
	Name          *string
	ThresholdCent *int64
	DiscountCent  *int64
	TotalCount    *int
	Remaining     *int
	StartsAt      *time.Time
	EndsAt        *time.Time
	Status        *model.CouponTemplateStatus
}

// CreateTemplate 校验后创建满减券模板，初始剩余数量等于发行总量。
func (s *CouponService) CreateTemplate(ctx context.Context, input CreateTemplateInput) (model.CouponTemplate, error) {
	if err := validateTemplate(input.Name, input.ThresholdCent, input.DiscountCent, input.TotalCount, input.StartsAt, input.EndsAt); err != nil {
		return model.CouponTemplate{}, err
	}
	template, err := s.repository.CreateCouponTemplate(ctx, model.CouponTemplate{
		Name: input.Name, Type: "threshold_discount", ThresholdCent: input.ThresholdCent, DiscountCent: input.DiscountCent,
		TotalCount: input.TotalCount, Remaining: input.TotalCount, PerUserLimit: 1,
		StartsAt: input.StartsAt, EndsAt: input.EndsAt, Status: model.CouponTemplateActive,
	})
	if err != nil {
		return model.CouponTemplate{}, fmt.Errorf("创建优惠券模板：%w", err)
	}
	return template, nil
}

// ListTemplates 分页查询优惠券模板（管理端）。
func (s *CouponService) ListTemplates(ctx context.Context, query repositories.ListCouponTemplatesQuery) (int64, []model.CouponTemplate, error) {
	total, templates, err := s.repository.ListCouponTemplates(ctx, query)
	if err != nil {
		return 0, nil, fmt.Errorf("查询优惠券模板列表：%w", err)
	}
	return total, templates, nil
}

// GetTemplate 查询单个优惠券模板。
func (s *CouponService) GetTemplate(ctx context.Context, templateID uint64) (model.CouponTemplate, error) {
	template, err := s.repository.GetCouponTemplate(ctx, templateID)
	if err != nil {
		return model.CouponTemplate{}, fmt.Errorf("查询优惠券模板：%w", err)
	}
	return template, nil
}

// UpdateTemplate 局部更新模板；名称若提交需重新校验长度。
func (s *CouponService) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (model.CouponTemplate, error) {
	if input.Name != nil {
		name := trimRunes(*input.Name, 50)
		if utf8.RuneCountInString(name) < 2 {
			return model.CouponTemplate{}, model.ErrInvalidCouponInput
		}
		input.Name = &name
	}
	fields := repositories.UpdateCouponTemplateFields{
		Name: input.Name, ThresholdCent: input.ThresholdCent, DiscountCent: input.DiscountCent,
		TotalCount: input.TotalCount, Remaining: input.Remaining, StartsAt: input.StartsAt, EndsAt: input.EndsAt, Status: input.Status,
	}
	template, err := s.repository.UpdateCouponTemplate(ctx, input.TemplateID, fields)
	if err != nil {
		return model.CouponTemplate{}, fmt.Errorf("更新优惠券模板：%w", err)
	}
	return template, nil
}

// DeleteTemplate 软删除模板，历史领取记录保留可审计。
func (s *CouponService) DeleteTemplate(ctx context.Context, templateID uint64) error {
	if err := s.repository.DeleteCouponTemplate(ctx, templateID); err != nil {
		return fmt.Errorf("删除优惠券模板：%w", err)
	}
	return nil
}

// Claim 用户领取优惠券：校验上架与有效期后写入持有记录，重复领取由唯一键阻止。
func (s *CouponService) Claim(ctx context.Context, userID, templateID uint64) (model.UserCoupon, error) {
	if userID == 0 {
		return model.UserCoupon{}, model.ErrInvalidUserID
	}
	template, err := s.repository.GetCouponTemplate(ctx, templateID)
	if err != nil {
		return model.UserCoupon{}, fmt.Errorf("查询优惠券模板：%w", err)
	}
	now := time.Now().UTC()
	if template.Status != model.CouponTemplateActive || now.Before(template.StartsAt) || now.After(template.EndsAt) || template.Remaining <= 0 {
		return model.UserCoupon{}, model.ErrCouponNotClaimable
	}
	coupon, err := s.repository.CreateUserCoupon(ctx, model.UserCoupon{
		UserID: userID, TemplateID: templateID, Status: model.UserCouponUnused, ClaimedAt: now,
	})
	if err != nil {
		return model.UserCoupon{}, fmt.Errorf("领取优惠券：%w", err)
	}
	return coupon, nil
}

// ListMine 查询当前用户持有的全部券。
func (s *CouponService) ListMine(ctx context.Context, userID uint64) ([]model.UserCoupon, error) {
	if userID == 0 {
		return nil, model.ErrInvalidUserID
	}
	coupons, err := s.repository.ListUserCoupons(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询用户优惠券列表：%w", err)
	}
	return coupons, nil
}

// validateTemplate 统一校验模板参数：名称长度、门槛面额为正、结束晚于开始、发行量为正。
func validateTemplate(name string, threshold, discount int64, total int, startsAt, endsAt time.Time) error {
	name = trimRunes(name, 50)
	if utf8.RuneCountInString(name) < 2 || threshold <= 0 || discount <= 0 || total <= 0 || !endsAt.After(startsAt) {
		return model.ErrInvalidCouponInput
	}
	return nil
}
