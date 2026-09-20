package repositories

import (
	"context"
	"errors"
	"fmt"

	model "ggg/models"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// CreateShipment 插入一条发货记录。
//
// ⚠️ 唯一键冲突（order_id + shipping_key 重复）不是故障，而是【幂等重试的信号】：
// 说明有另一个请求（或本请求的上一次投递）已经把这批发货记下来了。
// 这里翻译成 ErrShipmentDuplicate 上抛，由 service 决定怎么处理（返回首次结果）。
// 参考 order_repository.go 的 classifyCreateOrderError —— 同一套模式。
func (r *MySQLRepository) CreateShipment(ctx context.Context, shipment model.Shipment) (model.Shipment, error) {
	if err := r.db.WithContext(ctx).Create(&shipment).Error; err != nil {
		var dup *mysql.MySQLError
		if errors.As(err, &dup) && dup.Number == mysqlErrDupEntry {
			return model.Shipment{}, fmt.Errorf("%w：订单 %d", model.ErrShipmentDuplicate, shipment.OrderID)
		}
		return model.Shipment{}, fmt.Errorf("创建发货记录：%w", err)
	}
	return shipment, nil
}

// GetShipmentByOrderAndKey 按（订单 + 幂等键）查发货记录，用于幂等判断与重试返回首次结果。
// 查不到返回 ErrShipmentNotFound —— 调用方据此区分"这是重试"还是"想改物流信息"。
func (r *MySQLRepository) GetShipmentByOrderAndKey(ctx context.Context, orderID uint64, shippingKey string) (model.Shipment, error) {
	var shipment model.Shipment
	err := r.db.WithContext(ctx).
		Where("order_id = ? AND shipping_key = ?", orderID, shippingKey).
		Take(&shipment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Shipment{}, model.ErrShipmentNotFound
	}
	if err != nil {
		return model.Shipment{}, fmt.Errorf("查询发货记录：%w", err)
	}
	return shipment, nil
}

// ListShipmentsByOrder 查询订单的全部发货记录（买家看物流、运营核对都用它）。
// 当前业务一单只发一次，但按一对多设计——将来部分发货时前端无需改动。
func (r *MySQLRepository) ListShipmentsByOrder(ctx context.Context, orderID uint64) ([]model.Shipment, error) {
	shipments := make([]model.Shipment, 0)
	if err := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Order("id ASC").
		Find(&shipments).Error; err != nil {
		return nil, fmt.Errorf("查询发货列表：%w", err)
	}
	return shipments, nil
}

// ShipOrderTx 在一个事务内完成发货三件套（PRD-009 A7）：
//
//	① 条件更新订单 paid → shipped（WHERE 带前态，防并发重复发货）
//	② 插入 shipment 发货记录（shipping_key 唯一键承担幂等去重）
//	③ 写订单状态日志
//
// ── 为什么必须同事务 ──
// 拆开写的两种坏结果：
//
//	· 订单改了、shipment 没插：买家看到"已发货"却查不到任何物流单号
//	· shipment 插了、订单没改：出现幽灵发货记录，且订单还停在待发货可被再次发货
//
// 同事务保证【状态推进 ⇔ 发货凭证存在】同生共死——和 A1 扣库存、A2 支付回调同一个道理。
//
// ── 并发下的裁决顺序 ──
// 步骤①在前：两个相同请求同时到达时，只有一个能抢到"paid→shipped"的状态变更
// （行锁 + WHERE status=paid），另一个 RowsAffected=0 直接返回 ErrInvalidOrderTransition，
// service 层据此走幂等分支。步骤②的唯一键是第二道防线（防止跨请求的 key 竞争）。
func (r *MySQLRepository) ShipOrderTx(ctx context.Context, orderID, operatorID uint64, shipment model.Shipment) (model.Order, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusPaid).
			Updates(map[string]any{"status": model.OrderStatusShipped, "shipped_at": shipment.ShippedAt})
		if result.Error != nil {
			return fmt.Errorf("推进订单为已发货：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			// 已被并发请求推进，或状态本就不对：交回 service 判断是幂等还是非法迁移。
			return model.ErrInvalidOrderTransition
		}
		shipment.OrderID = orderID
		if err := tx.Create(&shipment).Error; err != nil {
			var dup *mysql.MySQLError
			if errors.As(err, &dup) && dup.Number == mysqlErrDupEntry {
				return fmt.Errorf("%w：订单 %d", model.ErrShipmentDuplicate, orderID)
			}
			return fmt.Errorf("写入发货记录：%w", err)
		}
		statusLog := model.OrderStatusLog{
			OrderID: orderID, FromStatus: model.OrderStatusPaid, ToStatus: model.OrderStatusShipped,
			OperatorType: model.OperatorAdmin, OperatorID: operatorID, Remark: "运营发货",
		}
		if err := tx.Create(&statusLog).Error; err != nil {
			return fmt.Errorf("写入订单状态日志：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.Order{}, err
	}
	// 事务提交后重读返回最新订单（复用既有查询，保持 repository 各方法返回形状一致）。
	return r.AdminGetOrderByID(ctx, orderID)
}
