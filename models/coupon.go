package models

import (
	"time"

	"gorm.io/gorm"
)

// CouponStatus 表示优惠券模板的上下架状态。
type CouponTemplateStatus string

const (
	CouponTemplateActive   CouponTemplateStatus = "active"
	CouponTemplateInactive CouponTemplateStatus = "inactive"
)

// CouponTemplate 表示一种满减券的模板：订单满 threshold_cent 减 discount_cent。
// TODO(PRD-009 进阶): remaining 的并发防超发需条件更新实现，本阶段只提供管理 CRUD（见 docs/ADVANCED-TASKS.md A4）。
type CouponTemplate struct {
	ID            uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string               `json:"name" gorm:"size:50;not null"`
	Type          string               `json:"type" gorm:"type:enum('threshold_discount');not null;default:threshold_discount"`
	ThresholdCent int64                `json:"threshold_cent" gorm:"not null"`
	DiscountCent  int64                `json:"discount_cent" gorm:"not null"`
	TotalCount    int                  `json:"total_count" gorm:"not null"`
	Remaining     int                  `json:"remaining" gorm:"not null"`
	PerUserLimit  int                  `json:"per_user_limit" gorm:"not null;default:1"`
	StartsAt      time.Time            `json:"starts_at" gorm:"not null"`
	EndsAt        time.Time            `json:"ends_at" gorm:"not null"`
	Status        CouponTemplateStatus `json:"status" gorm:"type:enum('active','inactive');not null;default:active;index"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	DeletedAt     gorm.DeletedAt       `json:"-" gorm:"index"`
}

// TableName 显式指定 CouponTemplate 对应的 MySQL 表名。
func (CouponTemplate) TableName() string { return "coupon_templates" }

// UserCouponStatus 表示用户持有券的状态。
type UserCouponStatus string

const (
	UserCouponUnused  UserCouponStatus = "unused"
	UserCouponUsed    UserCouponStatus = "used"
	UserCouponExpired UserCouponStatus = "expired"
)

// UserCoupon 表示用户领取的一张券。当前唯一键按每人每模板一张建模。
// UserCoupon 是用户持有的一张券。Seq（第几张）参与唯一键 uk_user_coupons_user_template_seq，
// 它是"每人限领 N 张"的数据库层防线：并发超限时第二个插入撞三元组唯一键被拦（PRD-009 A4）。
type UserCoupon struct {
	ID         uint64           `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     uint64           `json:"user_id" gorm:"not null;uniqueIndex:uk_user_coupons_user_template_seq,priority:1"`
	TemplateID uint64           `json:"template_id" gorm:"not null;uniqueIndex:uk_user_coupons_user_template_seq,priority:2;index"`
	Seq        int              `json:"seq" gorm:"not null;uniqueIndex:uk_user_coupons_user_template_seq,priority:3;default:1"` // 该用户在此模板下的第几张，从 1 起
	Status     UserCouponStatus `json:"status" gorm:"type:enum('unused','used','expired');not null;default:unused;index"`
	// 以下为不落库的展示字段（gorm:"-"）：券包页要显示券名与面额，
	// 这些属于模板表，由 service 聚合填充，避免前端为每张券再查一次模板。
	TemplateName  string         `json:"template_name" gorm:"-"`
	ThresholdCent int64          `json:"threshold_cent" gorm:"-"`
	DiscountCent  int64          `json:"discount_cent" gorm:"-"`
	OrderID       *uint64        `json:"order_id"`
	ClaimedAt     time.Time      `json:"claimed_at" gorm:"not null"`
	UsedAt        *time.Time     `json:"used_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"uniqueIndex:uk_user_coupons_user_template_seq,priority:4;index"`
}

// TableName 显式指定 UserCoupon 对应的 MySQL 表名。
func (UserCoupon) TableName() string { return "user_coupons" }
