package repositories

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	model "ggg/models"

	"github.com/go-sql-driver/mysql"
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

// GetPaymentByNoAnyUser 按支付单号查询（不限用户）。仅供渠道回调路径使用：
// 回调没有登录态，越权面由"只推进该支付单自身状态、不返回他人数据给调用方"控制。
func (r *MySQLRepository) GetPaymentByNoAnyUser(ctx context.Context, paymentNo string) (model.Payment, error) {
	var payment model.Payment
	err := r.db.WithContext(ctx).Where("payment_no = ?", paymentNo).Take(&payment).Error
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

// errConcurrentCallbackLost 是内部哨兵错误：本事务在 event_no 唯一键上输给并发请求。
// 触发外层"以新事务重试一次"，重试会命中"事件号已登记"分支实现幂等返回。
var errConcurrentCallbackLost = errors.New("并发回调竞争失败，需新事务重试")

// ConsumePaymentCallback 在一个事务内幂等消费支付回调（PRD-007 进阶 A2）。
// 返回 bool=false 表示该事件此前已被处理过（重复回调），调用方应直接确认成功而不产生二次业务效果。
//
// 幂等采用双保险，缺一不可：
//
//	① payments.event_no 唯一键：并发下两个相同事件的请求同时通过"查无此事件"检查时，
//	   后插入者撞 uk_payments_event_no 被拦——这是唯一能对抗时间差的防线（同 Idempotency-Key 模式）。
//	② status='pending' 前态条件：防不同事件号对同一支付单的竞态（如 success 与 failed 几乎同时到达）。
func (r *MySQLRepository) ConsumePaymentCallback(ctx context.Context, input ConsumeCallbackInput) (model.Payment, bool, error) {
	// 竞争失败重试一次（仅一次）：第二次必然命中"事件号已登记"或前态不匹配分支，无需循环。
	payment, first, err := r.consumePaymentCallbackOnce(ctx, input)
	if errors.Is(err, errConcurrentCallbackLost) {
		return r.consumePaymentCallbackOnce(ctx, input)
	}
	return payment, first, err
}

// consumePaymentCallbackOnce 执行一次回调消费事务，是 ConsumePaymentCallback 的"核"；
// 竞争失败时由外层壳以全新事务重放（见 errConcurrentCallbackLost）。
func (r *MySQLRepository) consumePaymentCallbackOnce(ctx context.Context, input ConsumeCallbackInput) (model.Payment, bool, error) {
	var consumed model.Payment
	firstEffect := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 定位支付单（回调来自渠道，无 user_id 上下文；越权风险由"只按 payment_no 推进自身状态、不暴露数据"控制）。
		var payment model.Payment
		if err := tx.Where("payment_no = ?", input.PaymentNo).Take(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrPaymentNotFound
			}
			return fmt.Errorf("查询支付单：%w", err)
		}
		// 2. 事件号已登记 → 本事件此前已完整处理，幂等返回首次结果。
		if payment.EventNo != nil && *payment.EventNo == input.EventNo {
			consumed = payment
			return nil
		}
		// 3. 落回调日志（无论后续成否都留痕；但注意它在事务内——业务失败会连日志一起回滚，
		//    排查"渠道说发了但我们没记"时需结合渠道侧对账，真实系统会把日志写在独立事务里。学习阶段先保简单一致。）
		callbackLog := model.PaymentCallbackLog{
			PaymentNo: input.PaymentNo, EventNo: input.EventNo, Result: input.Result,
			Payload: input.Payload, Processed: true,
		}
		if err := tx.Create(&callbackLog).Error; err != nil {
			return fmt.Errorf("记录支付回调日志：%w", err)
		}
		// 4. 条件推进支付单 pending→目标态；event_no 写入即登记幂等键。
		toStatus := model.PaymentFailed
		extra := map[string]any{"event_no": input.EventNo}
		if input.Result == "success" {
			toStatus = model.PaymentSuccess
			extra["paid_at"] = input.PaidAt
		}
		result := tx.Model(&model.Payment{}).
			Where("id = ? AND status = ?", payment.ID, model.PaymentPending).
			Updates(extraWithStatus(toStatus, extra))
		if result.Error != nil {
			var dup *mysql.MySQLError
			if errors.As(result.Error, &dup) && dup.Number == mysqlErrDupEntry {
				// ⚠️ InnoDB 死锁/冲突时可能已把本事务选为牺牲者回滚（1213），此后在旧 tx 里的一切读取都不可信；
				// 正确姿势是放弃当前事务、由外层以全新事务重试一次——重试进入第 2 步"事件号已登记"分支即幂等返回。
				return errConcurrentCallbackLost
			}
			return fmt.Errorf("推进支付单状态：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			// 支付单已非 pending。两种成因要区分开：
			//   a) 同一事件已被并发请求完整处理 → 幂等返回首次结果（PRD-007：重复回调确认成功、零副作用）；
			//   b) 不同事件抢先推进（如 success 后又来 failed）→ 真冲突，拒绝。
			var current model.Payment
			if probeErr := tx.Where("id = ?", payment.ID).Take(&current).Error; probeErr != nil {
				return fmt.Errorf("核对支付单状态：%w", probeErr)
			}
			if current.EventNo != nil && *current.EventNo == input.EventNo {
				consumed = current
				return nil // firstEffect=false：非首次，幂等确认
			}
			return model.ErrCallbackOrderStateConflict
		}
		firstEffect = true
		consumed = payment
		consumed.Status = toStatus
		consumed.EventNo = &input.EventNo
		// 5. 仅成功回调推进订单；订单不存在/状态冲突走异常路径（乱序防御）。
		if input.Result != "success" {
			return nil
		}
		orderResult := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", payment.OrderID, model.OrderStatusPendingPayment).
			Updates(map[string]any{"status": model.OrderStatusPaid, "paid_at": input.PaidAt})
		if orderResult.Error != nil {
			return fmt.Errorf("推进订单状态：%w", orderResult.Error)
		}
		if orderResult.RowsAffected == 0 {
			// 思考点：为什么这里不能静默跳过？钱收了货没了必须显式报错进人工通道（PRD-007 测试要求第 6 条）。
			return model.ErrCallbackOrderStateConflict
		}
		statusLog := model.OrderStatusLog{
			OrderID: payment.OrderID, FromStatus: model.OrderStatusPendingPayment, ToStatus: model.OrderStatusPaid,
			OperatorType: model.OperatorSystem, OperatorID: 0, Remark: "支付成功回调",
		}
		if err := tx.Create(&statusLog).Error; err != nil {
			return fmt.Errorf("写入订单状态日志：%w", err)
		}
		// ── 撤销闹钟：支付成功了，把"到期自动关单"任务作废 ──
		// 不做的后果：worker 到点照样触发关单流程——虽然 CancelOrderBySystem 的状态机会拒绝
		// （订单已是 paid），但让一个注定失败的任务空跑一轮不如当场注销。
		// JSON_EXTRACT 从 payload 里按 JSON 路径取 order_id 匹配——MySQL 原生 JSON 函数；
		// 学习期用它是合理的偷懒，量大后应给 background_tasks 加 related_order_id 索引列替代。
		if err := tx.Model(&model.BackgroundTask{}).
			Where("task_type = ? AND status = ? AND JSON_EXTRACT(payload, '$.order_id') = ?", "order_timeout_close", model.TaskPending, payment.OrderID).
			Update("status", model.TaskSucceeded).Error; err != nil {
			return fmt.Errorf("作废超时关单任务：%w", err)
		}
		// ── 埋新事件：登记"发支付成功站内信"任务（Outbox 生产端）──
		// 与上面的状态推进在同一个事务里：回调业务效果落库的瞬间，通知承诺也一定落库了——
		// 不存在"订单显示已支付但用户永远收不到通知"的缝隙。这就是 Outbox 模式的价值。
		var paidOrder model.Order
		if err := tx.Select("order_no").First(&paidOrder, payment.OrderID).Error; err != nil {
			return fmt.Errorf("查询已支付订单号：%w", err)
		}
		notifyTask := model.BackgroundTask{
			TaskType:    "notification_send",
			Payload:     map[string]any{"user_id": payment.UserID, "type": "payment", "title": "支付成功", "content": fmt.Sprintf("订单 %s 已支付成功", paidOrder.OrderNo)},
			Status:      model.TaskPending,
			MaxAttempts: 3,
			NextRunAt:   input.PaidAt, // 立即到期：下一个心跳（≤5秒）就该送达
		}
		if err := tx.Create(&notifyTask).Error; err != nil {
			return fmt.Errorf("登记支付通知任务：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.Payment{}, false, err
	}
	return consumed, firstEffect, nil
}

// extraWithStatus 把目标状态并入更新集合，纯语法糖。
func extraWithStatus(status model.PaymentStatus, extra map[string]any) map[string]any {
	updates := maps.Clone(extra)
	if updates == nil {
		updates = map[string]any{}
	}
	updates["status"] = status
	return updates
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ PaymentRepository = (*MySQLRepository)(nil)
