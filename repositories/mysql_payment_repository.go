package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "ggg/models"
	"gorm.io/gorm"
)

// CreatePayment 新增支付单。
func (r *MySQLRepository) CreatePayment(ctx context.Context, payment model.Payment) (model.Payment, error) {
	if err := r.db.WithContext(ctx).Create(&payment).Error; err != nil {
		return model.Payment{}, fmt.Errorf("创建支付单：%w", err)
	}
	return payment, nil
}

// GetPaymentByNo 按支付单号查询当前用户自己的支付单。
func (r *MySQLRepository) GetPaymentByNo(ctx context.Context, userID uint64, paymentNo string) (model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Where("payment_no = ? AND user_id = ?", paymentNo, userID).First(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Payment{}, model.ErrPaymentNotFound
	}
	if err != nil {
		return model.Payment{}, fmt.Errorf("查询支付单：%w", err)
	}
	return payment, nil
}

// GetActivePaymentByOrder 查询订单当前待支付的支付单。
func (r *MySQLRepository) GetActivePaymentByOrder(ctx context.Context, userID, orderID uint64) (model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Where("order_id = ? AND user_id = ? AND status = ?", orderID, userID, model.PaymentPending).First(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Payment{}, model.ErrPaymentNotFound
	}
	if err != nil {
		return model.Payment{}, fmt.Errorf("查询待支付支付单：%w", err)
	}
	return payment, nil
}

// UpdatePaymentResult 把支付单从 fromStatus 推进到 success 并记录事件号和完成时间；
// from 条件让重复回调第二次执行时 RowsAffected 为 0，由调用方按幂等处理。
func (r *MySQLRepository) UpdatePaymentResult(ctx context.Context, paymentNo string, fromStatus model.PaymentStatus, eventNo *string, paidAt *time.Time) (model.Payment, error) {
	result := r.db.WithContext(ctx).Model(&model.Payment{}).
		Where("payment_no = ? AND status = ?", paymentNo, fromStatus).
		Updates(map[string]any{"status": model.PaymentSuccess, "event_no": eventNo, "paid_at": paidAt})
	if result.Error != nil {
		return model.Payment{}, fmt.Errorf("更新支付单状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Payment{}, model.ErrPaymentNotFound
	}
	var payment model.Payment
	if err := r.db.WithContext(ctx).Where("payment_no = ?", paymentNo).First(&payment).Error; err != nil {
		return model.Payment{}, fmt.Errorf("查询支付单：%w", err)
	}
	return payment, nil
}

// CreateCallbackLog 保存回调原始报文。
func (r *MySQLRepository) CreateCallbackLog(ctx context.Context, log model.PaymentCallbackLog) error {
	if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
		return fmt.Errorf("记录支付回调日志：%w", err)
	}
	return nil
}
