package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	model "ggg/models"
	"ggg/repositories"
)

// orderPaymentTTL 是待支付订单的有效期；超时自动关单由 PRD-008 后台任务执行，本阶段只记录到期时间。
const orderPaymentTTL = 30 * time.Minute

// CartPreviewItem 表示下单预览中的一个商品行：服务端实时价格 + 可购买性判断。
type CartPreviewItem struct {
	ItemID        uint64 `json:"item_id"`
	SKUID         uint64 `json:"sku_id"`
	ProductName   string `json:"product_name"`
	SKUCode       string `json:"sku_code"`
	UnitPriceCent int64  `json:"unit_price_cent"`
	Quantity      int64  `json:"quantity"`
	SubtotalCent  int64  `json:"subtotal_cent"`
	Stock         int64  `json:"stock"`
	Purchasable   bool   `json:"purchasable"`
	Reason        string `json:"reason,omitempty"` // 不可购买原因：商品下架、SKU 停用或库存不足
}

// CartPreview 是下单预览结果，金额全部由服务端重新计算。
type CartPreview struct {
	Items            []CartPreviewItem `json:"items"`
	TotalCent        int64             `json:"total_cent"`
	PurchasableCount int               `json:"purchasable_count"`
}

// CreateOrderInput 表示创建订单所需的业务参数。金额与商品信息一律由服务端从购物车和 SKU 重新读取。
type CreateOrderInput struct {
	UserID         uint64
	AddressID      uint64
	IdempotencyKey string
}

// OrderService 负责购物车预览、订单创建与状态机流转。
type OrderService struct{ repository repositories.Repository }

// NewOrderService 创建订单业务服务。
func NewOrderService(repository repositories.Repository) *OrderService {
	return &OrderService{repository: repository}
}

// Preview 重新读取选中购物车项的实时价格与库存，生成下单预览；不锁库存、不固定价格。
func (s *OrderService) Preview(ctx context.Context, userID uint64) (CartPreview, error) {
	if userID == 0 {
		return CartPreview{}, model.ErrInvalidUserID
	}
	cart, err := s.repository.GetCart(ctx, userID)
	if err != nil {
		return CartPreview{}, fmt.Errorf("查询购物车：%w", err)
	}
	preview := CartPreview{Items: []CartPreviewItem{}}
	for i := range cart.Items {
		item := &cart.Items[i]
		if !item.Selected {
			continue // 未选中的明细不参与预览
		}
		row, err := s.buildPreviewRow(ctx, item)
		if err != nil {
			return CartPreview{}, err
		}
		preview.Items = append(preview.Items, row)
		if row.Purchasable {
			preview.TotalCent += row.SubtotalCent
			preview.PurchasableCount++
		}
	}
	return preview, nil
}

// buildPreviewRow 逐条读取 SKU 与商品状态，标记可购买性和不可购买原因。
// TODO(PRD-004 进阶): 当前每条明细一次查询存在 N+1，应改为收集 sku_id 后批量 IN 查询（见 docs/ADVANCED-TASKS.md A5）。
func (s *OrderService) buildPreviewRow(ctx context.Context, item *model.CartItem) (CartPreviewItem, error) {
	row := CartPreviewItem{ItemID: item.ID, SKUID: item.SKUID, Quantity: item.Quantity, UnitPriceCent: 0, SubtotalCent: 0}
	sku, err := s.repository.GetSKUByID(ctx, item.SKUID)
	if err != nil {
		if isModelNotFound(err) {
			row.Reason = "SKU 不存在或已删除"
			return row, nil
		}
		return row, fmt.Errorf("查询 SKU：%w", err)
	}
	// ⚠️ GetProduct 返回 *model.Product：not found 时是 (nil, ErrProductNotFound)，
	// 指针接收者调方法不 panic、解引用字段才 panic——所以必须先判错再取 Name。
	product, err := s.repository.GetProduct(ctx, sku.ProductID)
	if err != nil {
		if isModelNotFound(err) {
			row.Reason = "商品不存在或已删除"
			return row, nil // 与 SKU 同款处理：标记不可购买而非中断整个预览
		}
		return row, fmt.Errorf("查询商品：%w", err)
	}
	row.ProductName = product.Name
	row.SKUCode = sku.Code
	row.UnitPriceCent = sku.PriceCent
	row.SubtotalCent = sku.PriceCent * item.Quantity
	row.Stock = sku.Stock
	switch {
	case product.Status != model.ProductStatusOnSale:
		row.Reason = "商品已下架"
	case sku.Status != model.SKUStatusActive:
		row.Reason = "SKU 已停用"
	case item.Quantity > sku.Stock:
		row.Reason = "库存不足"
	default:
		row.Purchasable = true
	}
	return row, nil
}

// Create 从购物车选中项创建订单：校验幂等键 → 重算预览 → 固化快照 → 事务内预占库存并落库。
func (s *OrderService) Create(ctx context.Context, input CreateOrderInput) (model.Order, error) {
	if input.UserID == 0 {
		return model.Order{}, model.ErrInvalidUserID
	}
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 64 {
		return model.Order{}, model.ErrInvalidIdempotencyKey
	}
	// 目的：客户端可能传任意字符串做 key，统一哈希成定长值存储，避免长度和字符集问题。
	keyHash := sha256.Sum256([]byte(input.IdempotencyKey))
	shortKey := hex.EncodeToString(keyHash[:])[:32]

	address, err := s.repository.GetAddress(ctx, input.UserID, input.AddressID)
	if err != nil {
		if isModelNotFound(err) {
			return model.Order{}, model.ErrAddressNotFound // 透传 sentinel，让 API 层映射为 404（他人/不存在资源不泄露细节）
		}
		return model.Order{}, fmt.Errorf("查询收货地址：%w", err)
	}

	preview, err := s.Preview(ctx, input.UserID)
	if err != nil {
		return model.Order{}, err
	}
	if preview.PurchasableCount == 0 {
		return model.Order{}, model.ErrEmptyCartSelection
	}
	// 先算本次请求内容指纹，用于识别"相同幂等键但请求不同"。
	fingerprint := requestFingerprint(preview, address)

	// 幂等重放：同一用户同一 key 命中已有订单时，指纹一致返回首次结果，不一致说明复用了 key 提交不同内容。
	if existing, err := s.repository.GetOrderByIdempotencyKey(ctx, input.UserID, shortKey); err == nil {
		if existing.RequestHash != fingerprint {
			return model.Order{}, model.ErrIdempotencyConflict
		}
		return existing, nil
	} else if !isModelNotFound(err) {
		return model.Order{}, fmt.Errorf("查询幂等订单：%w", err)
	}

	now := time.Now().UTC()
	expires := now.Add(orderPaymentTTL)
	order := model.Order{
		OrderNo:        newBizNo(),
		UserID:         input.UserID,
		IdempotencyKey: shortKey,
		RequestHash:    fingerprint,
		Status:         model.OrderStatusPendingPayment,
		TotalCent:      preview.TotalCent,
		PayCent:        preview.TotalCent,
		Address:        toSnapshot(address),
		ExpiresAt:      &expires,
	}
	for _, row := range preview.Items {
		if !row.Purchasable {
			continue // 不可购买项不进订单，原因已在预览中反馈
		}
		order.Items = append(order.Items, model.OrderItem{
			SKUID: row.SKUID, ProductName: row.ProductName, SKUCode: row.SKUCode,
			UnitPriceCent: row.UnitPriceCent, Quantity: row.Quantity, SubtotalCent: row.SubtotalCent,
		})
	}
	// 落库：repository 在单事务内完成 预占库存→写订单/订单项/状态日志；任一步失败整单回滚。
	created, err := s.repository.CreateOrder(ctx, order)
	if err != nil {
		// 并发兜底：两个相同幂等键的请求同时通过上面的重放查询时，后到的那个会撞唯一键
		// uk_orders_user_idempotency 并被翻译为 ErrIdempotencyConflict——此时首次请求已建单成功，
		// 重新查询返回那份结果即可（本次事务已整体回滚，我们的扣减未生效，不会重复占库存）。
		if errors.Is(err, model.ErrIdempotencyConflict) {
			existing, qErr := s.repository.GetOrderByIdempotencyKey(ctx, input.UserID, shortKey)
			if qErr == nil && existing.RequestHash == fingerprint {
				return existing, nil
			}
			if qErr != nil && !isModelNotFound(qErr) {
				return model.Order{}, fmt.Errorf("查询幂等订单：%w", qErr)
			}
			return model.Order{}, err // 查不到或内容不符：交回冲突错误由 API 层报 409
		}
		return model.Order{}, err
	}
	return created, nil
}

// ListByUser 分页查询当前用户的订单。
func (s *OrderService) ListByUser(ctx context.Context, userID uint64, query repositories.ListOrdersQuery) (int64, []model.Order, error) {
	if userID == 0 {
		return 0, nil, model.ErrInvalidUserID
	}
	total, orders, err := s.repository.ListOrdersByUser(ctx, userID, query)
	if err != nil {
		return 0, nil, fmt.Errorf("查询订单列表：%w", err)
	}
	return total, orders, nil
}

// Get 查询当前用户自己的订单详情及状态历史。
func (s *OrderService) Get(ctx context.Context, userID, orderID uint64) (model.Order, []model.OrderStatusLog, error) {
	if userID == 0 {
		return model.Order{}, nil, model.ErrInvalidUserID
	}
	order, err := s.repository.GetOrderByID(ctx, userID, orderID)
	if err != nil {
		return model.Order{}, nil, fmt.Errorf("查询订单：%w", err)
	}
	logs, err := s.repository.ListOrderStatusLogs(ctx, orderID)
	if err != nil {
		return model.Order{}, nil, err
	}
	return order, logs, nil
}

// Cancel 消费者取消待支付订单：repository 在单事务内完成 条件推进状态→释放预占库存→记日志。
// 注意"能否取消"的判断已下沉为 SQL 前态条件，service 不再事务外预读状态——过期快照上的判断不可信。
func (s *OrderService) Cancel(ctx context.Context, userID, orderID uint64) (model.Order, error) {
	if userID == 0 {
		return model.Order{}, model.ErrInvalidUserID
	}
	return s.repository.CancelOrder(ctx, userID, orderID)
}

// ConfirmReceipt 消费者确认收货，仅允许已发货订单。
func (s *OrderService) ConfirmReceipt(ctx context.Context, userID, orderID uint64) (model.Order, error) {
	order, err := s.repository.GetOrderByID(ctx, userID, orderID)
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	if !order.Status.CanTransitionTo(model.OrderStatusCompleted) {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	now := time.Now().UTC()
	updated, err := s.repository.UpdateOrderStatus(ctx, userID, orderID, model.OrderStatusCompleted, map[string]any{"completed_at": &now})
	if err != nil {
		return model.Order{}, fmt.Errorf("确认收货：%w", err)
	}
	if err := s.repository.CreateOrderStatusLog(ctx, model.OrderStatusLog{
		OrderID: orderID, FromStatus: order.Status, ToStatus: updated.Status,
		OperatorType: model.OperatorUser, OperatorID: userID, Remark: "确认收货",
	}); err != nil {
		return model.Order{}, err
	}
	return updated, nil
}

// Ship 运营发货，仅允许已支付订单。
// TODO(PRD-009 进阶): 相同物流信息重复提交应幂等返回首次结果而非 409（见 docs/ADVANCED-TASKS.md A7）。
func (s *OrderService) Ship(ctx context.Context, operatorID, orderID uint64) (model.Order, error) {
	order, err := s.repository.AdminGetOrderByID(ctx, orderID)
	if err != nil {
		return model.Order{}, fmt.Errorf("查询订单：%w", err)
	}
	if !order.Status.CanTransitionTo(model.OrderStatusShipped) {
		return model.Order{}, model.ErrInvalidOrderTransition
	}
	now := time.Now().UTC()
	updated, err := s.repository.AdminUpdateOrderStatus(ctx, orderID, order.Status, model.OrderStatusShipped, map[string]any{"shipped_at": &now})
	if err != nil {
		return model.Order{}, fmt.Errorf("发货：%w", err)
	}
	if err := s.repository.CreateOrderStatusLog(ctx, model.OrderStatusLog{
		OrderID: orderID, FromStatus: order.Status, ToStatus: updated.Status,
		OperatorType: model.OperatorAdmin, OperatorID: operatorID, Remark: "运营发货",
	}); err != nil {
		return model.Order{}, err
	}
	return updated, nil
}

// AdminList 管理端分页查询全部订单。
func (s *OrderService) AdminList(ctx context.Context, query repositories.ListOrdersQuery) (int64, []model.Order, error) {
	total, orders, err := s.repository.AdminListOrders(ctx, query)
	if err != nil {
		return 0, nil, fmt.Errorf("查询订单列表：%w", err)
	}
	return total, orders, nil
}

// AdminGet 管理端查询任意订单详情。
func (s *OrderService) AdminGet(ctx context.Context, orderID uint64) (model.Order, []model.OrderStatusLog, error) {
	order, err := s.repository.AdminGetOrderByID(ctx, orderID)
	if err != nil {
		return model.Order{}, nil, fmt.Errorf("查询订单：%w", err)
	}
	logs, err := s.repository.ListOrderStatusLogs(ctx, orderID)
	if err != nil {
		return model.Order{}, nil, err
	}
	return order, logs, nil
}

// toSnapshot 把地址模型转换为订单内嵌快照。
func toSnapshot(a model.Address) model.AddressSnapshot {
	return model.AddressSnapshot{Recipient: a.Recipient, Phone: a.Phone, Province: a.Province, City: a.City, District: a.District, Detail: a.Detail}
}

// requestFingerprint 对影响订单内容的字段做指纹，用于识别"相同幂等键但请求不同"。
func requestFingerprint(preview CartPreview, address model.Address) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%d|%d|%d|", preview.TotalCent, preview.PurchasableCount, address.ID)
	for _, row := range preview.Items {
		fmt.Fprintf(sum, "%d:%d;", row.SKUID, row.Quantity)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// newBizNo 生成"时间戳 + 随机后缀"的业务单号，冲突由数据库唯一键兜底。
func newBizNo() string {
	buf := make([]byte, 4)
	rand.Read(buf)
	return fmt.Sprintf("%s%04d%s", time.Now().UTC().Format("20060102150405"), time.Now().Nanosecond()/100000%10000, hex.EncodeToString(buf))
}

// isModelNotFound 判断错误链中是否为项目 sentinel"不存在"类错误。
// 目的：repository 已把 gorm.ErrRecordNotFound 统一转换为业务错误，service 层只需识别这些语义错误。
// 新增"不存在"类错误时必须同步登记到这里，否则调用方会把误判为数据库故障。
func isModelNotFound(err error) bool {
	return errors.Is(err, model.ErrSKUNotFound) || errors.Is(err, model.ErrProductNotFound) ||
		errors.Is(err, model.ErrOrderNotFound) || errors.Is(err, model.ErrAddressNotFound) ||
		errors.Is(err, model.ErrCouponNotFound) || errors.Is(err, model.ErrPaymentNotFound)
}
