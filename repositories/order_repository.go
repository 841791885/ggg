package repositories

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	model "ggg/models"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// mysqlErrDupEntry 是 MySQL 的重复键错误号（1062 Duplicate entry）。
const mysqlErrDupEntry = 1062

// mysqlErrDeadlock 是 InnoDB 死锁错误号（1213 Deadlock found when trying to get lock）。
const mysqlErrDeadlock = 1213

// isDeadlockError 判断错误链里是否为可重试的死锁。GORM 会把驱动错误包在中间层里，必须用 errors.As 穿透。
func isDeadlockError(err error) bool {
	var dup *mysql.MySQLError
	return errors.As(err, &dup) && dup.Number == mysqlErrDeadlock
}

// maxDeadlockRetries 死锁重试次数上限。死锁是瞬态冲突（两个事务互相等对方的行锁），
// MySQL 检测到环后会主动杀掉代价小的那个（errno 1213）并重放整笔事务即可自愈；
// 限次防止病态热竞争下无限循环烧 CPU。
const maxDeadlockRetries = 3

// CreateOrder 在一个数据库事务内完成：预占库存 → 写订单与订单项 → 记状态日志。
// 任一步失败整体回滚，保证"订单存在"与"库存已扣"永远同生共死（PRD-005 进阶 A1）。
// 遇 InnoDB 死锁自动重放整个事务（最多 3 次、指数退避）；幂等键冲突不重试——那是"已被人抢先建成"，
// 交给 service 层重新查询返回首次结果，重试只会白扣一轮库存。
func (r *MySQLRepository) CreateOrder(ctx context.Context, order model.Order) (model.Order, error) {
	var lastErr error
	for attempt := 0; attempt <= maxDeadlockRetries; attempt++ {
		created, err := r.createOrderOnce(ctx, order)
		switch {
		case err == nil:
			return created, nil
		case errors.Is(err, model.ErrIdempotencyConflict):
			return model.Order{}, err // 语义性冲突非瞬态故障，立即上抛
		case isDeadlockError(err):
			lastErr = err
			backoff := time.Duration(1<<attempt) * 5 * time.Millisecond // 5ms→10ms→20ms，给对方让路
			select {
			case <-ctx.Done():
				return model.Order{}, ctx.Err()
			case <-time.After(backoff):
			}
			continue
		default:
			return model.Order{}, err
		}
	}
	return model.Order{}, fmt.Errorf("创建订单死锁重试%d次仍失败：%w", maxDeadlockRetries, lastErr)
}

func (r *MySQLRepository) createOrderOnce(ctx context.Context, order model.Order) (model.Order, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		items := order.Items
		order.Items = nil // Omit("Items") 已阻止级联；置空双保险，确保订单主表按无关联创建
		if err := reserveStockTx(tx, items, order.OrderNo); err != nil {
			return err
		}
		if err := tx.Omit("Items").Create(&order).Error; err != nil {
			return classifyCreateOrderError(err)
		}
		for i := range items {
			items[i].OrderID = order.ID // 订单项外键需手工回填
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("写入订单项：%w", err)
			}
		}
		order.Items = items // 返回给调用方的完整订单仍带明细
		statusLog := model.OrderStatusLog{
			OrderID: order.ID, FromStatus: "", ToStatus: order.Status,
			OperatorType: model.OperatorUser, OperatorID: order.UserID, Remark: "创建订单",
		}
		if err := tx.Create(&statusLog).Error; err != nil {
			return fmt.Errorf("写入订单状态日志：%w", err)
		}
		// ── Outbox 模式（PRD-008 A3）：给这张订单埋一颗"超时自动关单"的种子 ──
		// 为什么写在同一个事务里？如果拆成"提交订单后再插任务"，两步之间进程崩溃，
		// 订单就永远没人关、库存被永久占用——Outbox 保证【订单存在 ⇔ 关单承诺存在】同生共死。
		// NextRunAt = *order.ExpiresAt：任务的"闹钟时间"就是订单的死刑时间，
		// worker 的通用扫描条件（next_run_at <= now）不需要为超时关单写任何特殊调度逻辑。
		if order.ExpiresAt != nil {
			timeoutTask := model.BackgroundTask{
				TaskType:    "order_timeout_close",
				Payload:     map[string]any{"order_id": order.ID, "reason": "payment_timeout"},
				Status:      model.TaskPending,
				MaxAttempts: 5,
				NextRunAt:   *order.ExpiresAt,
			}
			if err := tx.Create(&timeoutTask).Error; err != nil {
				return fmt.Errorf("登记超时关单任务：%w", err)
			}
		}
		return nil
	})
	if err != nil {
		return model.Order{}, err
	}
	return order, nil
}

// CancelOrder 在一个数据库事务内完成：条件推进状态 → 释放预占库存 → 记状态日志（PRD-005 进阶 A1）。
// 关键约束：订单的读取、状态判断、更新必须全在事务内——事务外读到的状态是过期快照，
// 拿它做"能否取消"的判断等于把超卖时间差请回来。
func (r *MySQLRepository) CancelOrder(ctx context.Context, userID, orderID uint64) (model.Order, error) {
	return r.cancelOrderInner(ctx, userID, orderID, model.OperatorUser, "用户取消订单")
}

// cancelOrderInner 是消费者取消与系统关单共享的事务主体（两个公开入口 CancelOrder / CancelOrderBySystem 都转发到这里）。
// 设计意图：取消动作的业务规则（前态校验、释放库存、写日志）对用户和系统必须【完全一致】——
// 复制两份实现迟早漂移出 bug（比如系统版忘了释放库存），所以只留一份闭包，用参数表达差异：
//   - operatorType/remark：状态日志的署名不同，审计时能区分"用户主动取消"和"超时被关"；
//   - userID：用户路径用它做所有权过滤（WHERE user_id），系统路径传 0 并在 probe 分支跳过越权检查。
func (r *MySQLRepository) cancelOrderInner(ctx context.Context, userID, orderID uint64, operatorType model.OperatorType, remark string) (model.Order, error) {
	var cancelled model.Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 思考题①的答案：防"两个 cancel 都成功"不靠先查后判，靠 WHERE status='pending_payment'
		// 的条件更新——第一个事务提交后行状态已变，第二个事务的 WHERE 匹配不到，RowsAffected=0。
		// 并发时第二个请求会阻塞在行锁上，等第一个提交后拿到新值再判定失败。
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", orderID, model.OrderStatusPendingPayment).
			Updates(map[string]any{"status": model.OrderStatusCancelled, "cancelled_at": time.Now().UTC()})
		if result.Error != nil {
			return fmt.Errorf("取消订单：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			// 一行都没改到 → 三种可能：单不存在 / 是别人的单 / 状态不对。
			// 但 UPDATE 的 WHERE 把三者混在一起了，API 需要不同的错误码（404 vs 409）——
			// 所以补一条只读查询"验尸"，分辨到底死因是哪个（仍在同一事务内，读到的一定是当前真相）。
			var probe model.Order
			probeErr := tx.Select("id", "user_id", "status").Take(&probe, orderID).Error
			if errors.Is(probeErr, gorm.ErrRecordNotFound) {
				return model.ErrOrderNotFound
			}
			if probeErr != nil {
				return fmt.Errorf("核对订单状态：%w", probeErr)
			}
			if operatorType == model.OperatorUser && probe.UserID != userID {
				return model.ErrOrderNotFound // 越权按不存在处理，不泄露资源存在性（系统路径跳过此检查：它没有用户身份）
			}
			return model.ErrInvalidOrderTransition // 存在但非待支付（如已支付）
		}
		// 事务内重读完整订单（含明细），既用于释放库存，也作为返回值与日志前态来源。
		if err := tx.Preload("Items").First(&cancelled, orderID).Error; err != nil {
			return fmt.Errorf("查询已取消订单：%w", err)
		}
		// 思考题②的答案：释放方向不需要 stock >= ? 条件。扣减怕的是"减成负数"（物理不可能发生的事），
		// 而加回只会让库存变大，恒合法；且数量来自该订单自己的预占记录，还多还少由业务不变量保证。
		if err := releaseStockTx(tx, cancelled.Items, cancelled.OrderNo); err != nil {
			return err
		}
		statusLog := model.OrderStatusLog{
			OrderID: orderID, FromStatus: model.OrderStatusPendingPayment, ToStatus: model.OrderStatusCancelled,
			OperatorType: operatorType, OperatorID: userID, Remark: remark,
		}
		if err := tx.Create(&statusLog).Error; err != nil {
			return fmt.Errorf("写入订单状态日志：%w", err)
		}
		return nil
	})
	if err != nil {
		return model.Order{}, err
	}
	return cancelled, nil
}

// CancelOrderBySystem 供 worker 超时关单使用：与 CancelOrder 同一套事务逻辑，
// 差别仅在身份——无 user_id 过滤（系统发起）、状态日志 OperatorType=system。
// 复用而非复制：把 CancelOrder 的闭包主体提取为 cancelOrderTx，两个入口只差外壳参数。
func (r *MySQLRepository) CancelOrderBySystem(ctx context.Context, orderID uint64) (model.Order, error) {
	return r.cancelOrderInner(ctx, 0, orderID, model.OperatorSystem, "超时自动关单")
}

// releaseStockTx 按订单明细反向归还预占库存：UPDATE skus SET stock = stock + n WHERE id = ?。
// 与 reserveStockTx 对称，同样按 sku_id 升序申请行锁，避免与其他事务形成等待环。
func releaseStockTx(tx *gorm.DB, items []model.OrderItem, orderNo string) error {
	qtyBySKU := aggregateBySKU(items)
	skuIDs := sortedUniqueSKUIDs(items)
	for _, skuID := range skuIDs {
		result := tx.Model(&model.SKU{}).
			Where("id = ?", skuID).
			UpdateColumn("stock", gorm.Expr("stock + ?", qtyBySKU[skuID]))
		if result.Error != nil {
			return fmt.Errorf("释放库存（SKU %d）：%w", skuID, result.Error)
		}
		if result.RowsAffected == 0 {
			// SKU 行已被硬删除才可能为 0；正常业务下订单存续期间 SKU 必然存在。
			return fmt.Errorf("释放库存失败：SKU %d 不存在", skuID)
		}
		if err := recordStockMovementTx(tx, skuID, qtyBySKU[skuID], model.StockMovementRelease, orderNo); err != nil {
			return err
		}
	}
	return nil
}

// aggregateBySKU 把订单项按 SKU 汇总数量（同一 SKU 多行合并为一笔），供扣减/释放取用。
func aggregateBySKU(items []model.OrderItem) map[uint64]int64 {
	qtyBySKU := make(map[uint64]int64, len(items))
	for _, item := range items {
		qtyBySKU[item.SKUID] += item.Quantity
	}
	return qtyBySKU
}

// sortedUniqueSKUIDs 返回去重后按 id 升序的 SKU 列表：去重保证每个 SKU 只处理一次，
// 升序保证所有事务以相同顺序申请行锁，避免交叉等待形成死锁。
func sortedUniqueSKUIDs(items []model.OrderItem) []uint64 {
	skuIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		skuIDs = append(skuIDs, item.SKUID)
	}
	slices.Sort(skuIDs)
	return slices.Compact(skuIDs) // Compact 去掉相邻重复项（排序后重复必然相邻）
}

// reserveStockTx 对订单内每个 SKU 执行条件原子扣减：UPDATE ... SET stock = stock - n WHERE id = ? AND stock >= n。
// 比较与扣减在同一条 SQL 内完成，杜绝"先查后改"的超卖窗口；RowsAffected=0 即库存不足，返回错误触发整单回滚。
// 多 SKU 按 id 升序加锁：并发事务以相同顺序申请行锁，不会形成等待环（死锁预防）。
// orderNo 由调用方传入用于流水凭证署名；创建事务里订单尚未插入，order_no 已在内存中生成可直接取用。
func reserveStockTx(tx *gorm.DB, items []model.OrderItem, orderNo string) error {
	qtyBySKU := aggregateBySKU(items)
	skuIDs := sortedUniqueSKUIDs(items)
	for _, skuID := range skuIDs {
		result := tx.Model(&model.SKU{}).
			Where("id = ? AND stock >= ?", skuID, qtyBySKU[skuID]).
			UpdateColumn("stock", gorm.Expr("stock - ?", qtyBySKU[skuID]))
		if result.Error != nil {
			return fmt.Errorf("预占库存（SKU %d）：%w", skuID, result.Error)
		}
		if result.RowsAffected == 0 {
			return model.ErrInsufficientStock
		}
		if err := recordStockMovementTx(tx, skuID, -qtyBySKU[skuID], model.StockMovementReserve, orderNo); err != nil {
			return err
		}
	}
	return nil
}

// recordStockMovementTx 写一条库存流水。balance_stock 在同一事务内回读——读到的是本事务
// 刚扣减/释放后的值（未提交），其他会话看不到，正好作为"变动后余额"快照。
// ⚠️ 前提：该 SKU 的锁必须已持有且流水紧随其后，否则并发事务的写入会污染回读结果。
func recordStockMovementTx(tx *gorm.DB, skuID uint64, changeQty int64, movementType model.StockMovementType, orderNo string) error {
	var sku model.SKU
	if err := tx.Select("stock").First(&sku, skuID).Error; err != nil {
		return fmt.Errorf("回读库存余额（SKU %d）：%w", skuID, err)
	}
	movement := model.StockMovement{
		SKUID: skuID, ChangeQty: changeQty, BalanceStock: sku.Stock, Type: movementType, OrderNo: orderNo,
	}
	if err := tx.Create(&movement).Error; err != nil {
		return fmt.Errorf("写入库存流水（SKU %d）：%w", skuID, err)
	}
	return nil
}

// classifyCreateOrderError 把唯一键冲突翻译为业务错误：幂等键或订单号重复说明有并发请求抢先创建了同一意图。
// service 层捕获 ErrIdempotencyConflict 后应重新查询并返回首次结果（A1 幂等兜底）。
func classifyCreateOrderError(err error) error {
	var dup *mysql.MySQLError
	if errors.As(err, &dup) && dup.Number == mysqlErrDupEntry {
		return fmt.Errorf("%w：%s", model.ErrIdempotencyConflict, err)
	}
	return fmt.Errorf("创建订单：%w", err)
}

// GetOrderByID 查询当前用户自己的订单及明细；他人订单统一按不存在处理。
func (r *MySQLRepository) GetOrderByID(ctx context.Context, userID, orderID uint64) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	return order, nil
}

// GetOrderByIdempotencyKey 按用户 + 幂等键查询已有订单，用于重试返回首次结果。
func (r *MySQLRepository) GetOrderByIdempotencyKey(ctx context.Context, userID uint64, key string) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").Where("user_id = ? AND idempotency_key = ?", userID, key).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询幂等订单：%w", err)
	}
	return order, nil
}

// ListOrdersByUser 分页查询当前用户的订单。
func (r *MySQLRepository) ListOrdersByUser(ctx context.Context, userID uint64, query ListOrdersQuery) (int64, []model.Order, error) {
	return r.listOrders(ctx, r.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID), query)
}

// UpdateOrderStatus 在消费者视角下推进状态：条件里带上当前状态做乐观校验，
// 状态被并发抢先变更时 RowsAffected 为 0，调用方重新读取后按非法迁移处理。
// extra 允许附带时间戳字段（paid_at 等）。
func (r *MySQLRepository) UpdateOrderStatus(ctx context.Context, userID, orderID uint64, to model.OrderStatus, extra map[string]any) (model.Order, error) {
	order, err := r.GetOrderByID(ctx, userID, orderID)
	if err != nil {
		return model.Order{}, err
	}
	updates := map[string]any{"status": to}
	maps.Copy(updates, extra)
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND user_id = ? AND status = ?", orderID, userID, order.Status).
		Updates(updates)
	if result.Error != nil {
		return model.Order{}, fmt.Errorf("更新订单状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	return r.GetOrderByID(ctx, userID, orderID)
}

// CreateOrderStatusLog 追加一条状态变更历史。
func (r *MySQLRepository) CreateOrderStatusLog(ctx context.Context, log model.OrderStatusLog) error {
	if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
		return fmt.Errorf("记录订单状态日志：%w", err)
	}
	return nil
}

// ListOrderStatusLogs 查询订单的状态变更历史。
func (r *MySQLRepository) ListOrderStatusLogs(ctx context.Context, orderID uint64) ([]model.OrderStatusLog, error) {
	var logs []model.OrderStatusLog
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id ASC").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("查询订单状态日志：%w", err)
	}
	return logs, nil
}

// AdminGetOrderByID 管理端查询任意订单及明细。
func (r *MySQLRepository) AdminGetOrderByID(ctx context.Context, orderID uint64) (model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).Preload("Items").First(&order, orderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrOrderNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	return order, nil
}

// AdminListOrders 管理端分页查询全部订单。
func (r *MySQLRepository) AdminListOrders(ctx context.Context, query ListOrdersQuery) (int64, []model.Order, error) {
	return r.listOrders(ctx, r.db.WithContext(ctx).Model(&model.Order{}), query)
}

// AdminUpdateOrderStatus 管理端推进状态，from→to 双条件保证只从合法前态迁移。
func (r *MySQLRepository) AdminUpdateOrderStatus(ctx context.Context, orderID uint64, from, to model.OrderStatus, extra map[string]any) (model.Order, error) {
	updates := map[string]any{"status": to}
	maps.Copy(updates, extra)
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, from).
		Updates(updates)
	if result.Error != nil {
		return model.Order{}, fmt.Errorf("更新订单状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	return r.AdminGetOrderByID(ctx, orderID)
}

// listOrders 是消费者与管理端共用的分页查询主体。
func (r *MySQLRepository) listOrders(ctx context.Context, tx *gorm.DB, query ListOrdersQuery) (int64, []model.Order, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	if query.Status != "" {
		tx = tx.Where("status = ?", query.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计订单数量：%w", err)
	}
	var orders []model.Order
	if err := tx.Preload("Items").Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&orders).Error; err != nil {
		return 0, nil, fmt.Errorf("查询订单列表：%w", err)
	}
	return total, orders, nil
}

// normalizePage 把非法分页参数收敛到安全范围。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ OrderRepository = (*MySQLRepository)(nil)
