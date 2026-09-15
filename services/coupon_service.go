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
// TODO(PRD-009 进阶): 下单事务内占用券与优惠金额分摊未实现（见 docs/ADVANCED-TASKS.md A8）。
// 领取已按 A4 落地：remaining 条件更新 + seq 唯一键双保险防超发超领。
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

// Claim 用户领取优惠券（PRD-009 进阶 A4）。
//
// 结构是经典的"预检 + 原子执行"两段式：
//
//	预检（本函数）：查模板、验状态和有效期——这些值在检查后【可能立刻变化】，
//	               所以它们只负责给出友好错误，不承担正确性；
//	执行（ClaimCouponTx）：扣量 + 插记录全在事务里用条件更新裁决，
//	               并发正确性 100% 由数据库保证（两个闸门详见 repository 层注释）。
//
// 类比前端表单校验：客户端校验是为了体验，服务端校验才是为了安全——同一个分层思想。
func (s *CouponService) Claim(ctx context.Context, userID, templateID uint64) (model.UserCoupon, error) {
	if userID == 0 {
		return model.UserCoupon{}, model.ErrInvalidUserID
	}
	template, err := s.repository.GetCouponTemplate(ctx, templateID)
	if err != nil {
		return model.UserCoupon{}, fmt.Errorf("查询优惠券模板：%w", err)
	}
	now := time.Now().UTC()
	// 注意这里删掉了旧版的 `template.Remaining <= 0` 判断——它读的是过期快照，
	// 防不住并发（查到 1 → 百人涌入 → 都以为自己有份）。剩余量的真相由事务里的
	// WHERE remaining > 0 独裁。保留的这几项（下架/未开始/已结束）是低频变化的配置型条件，预检有效。
	if template.Status != model.CouponTemplateActive || now.Before(template.StartsAt) || now.After(template.EndsAt) {
		return model.UserCoupon{}, model.ErrCouponNotClaimable
	}
	// 限领预检：多数超限请求在这里就被拦下（省一次事务开销），
	// 漏网的并发竞态由 seq 唯一键收口——这就是"快速失败 + 兜底正确"的标准分工。
	held, err := s.repository.CountUserCouponsByTemplate(ctx, userID, templateID)
	if err != nil {
		return model.UserCoupon{}, err
	}
	if held >= int64(template.PerUserLimit) {
		return model.UserCoupon{}, model.ErrCouponAlreadyClaimed
	}
	return s.repository.ClaimCouponTx(ctx, userID, templateID, template.PerUserLimit)
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
	// 补齐券的展示信息（券名/门槛/面额来自模板表）——券包页要显示"满 X 减 Y"，
	// 只给 template_id 前端无法展示。同一模板的券共用一次查询（map 缓存模板，避免重复查库）。
	templateCache := map[uint64]model.CouponTemplate{}
	for i := range coupons {
		tid := coupons[i].TemplateID
		tpl, ok := templateCache[tid]
		if !ok {
			fetched, err := s.repository.GetCouponTemplate(ctx, tid)
			if err != nil {
				continue // 模板被删：保留持有记录但无展示信息，不阻断列表
			}
			tpl = fetched
			templateCache[tid] = fetched
		}
		coupons[i].TemplateName = tpl.Name
		coupons[i].ThresholdCent = tpl.ThresholdCent
		coupons[i].DiscountCent = tpl.DiscountCent
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
