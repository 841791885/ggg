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

// Preview 生成"下单预览"：把购物车里勾选的商品，按【此刻】的真实价格与库存重算一遍。
//
// ── 它是干什么的（一句话）──
// 用户在结算页看到的那个清单：每件商品多少钱、能不能买、一共多少 —— 全部由这里算出来，
// 前端传来的任何金额都不作数（防篡改的第一道闸门）。
//
// ── 前端类比 ──
// 相当于服务端的 "computed 属性"：输入是购物车（可能已过期），输出是一份【当下有效】的结算清单。
// 前端当然也能算，但那份计算基于可能过期的数据，且用户能改 —— 所以真算必须在这里做。
//
// ── 关键设计：它"不确定"任何东西 ──
// 预览展示的库存和价格，到你真正下单那一刻可能已经变了（别人抢走了最后一件）。
// 所以它返回的是"此刻看起来能买"，不是承诺。真正的一致性由 Create 里的事务扣减保证：
// 预览 → 下单之间只要有变化，下单就会因库存不足而失败并提示用户。
// ⚠️ 这条"预览不锁资源"的边界很重要：如果这里锁库存，用户打开结算页就会占用库存，
//
//	而多数人看完就走了，库存会被白白占死。
//
// 关于 err 的处理：单条明细查询失败（数据库抖动）会中断整个预览（fail fast）；
// 而"商品下架/库存不足"这类业务态不算 error，会被标记进 row.Reason 照常返回。
// ⚠️ 版本说明：这是【逐条查询版】（N+1），与下方 BatchPreview【批量版】并存对比。
// 每条明细各查一次 SKU + 商品，10 条明细 = 21 次数据库往返。保留用于学习对照。
func (s *OrderService) Preview(ctx context.Context, userID uint64) (CartPreview, error) {
	// 实现已切换为批量查询版（A5）：N+1 → 固定 3 次查询，对外行为完全不变。
	// 保留单一入口而非让调用方直接调 BatchPreview，是为了让"预览"只有一条生产路径——
	// 下单（Create）与展示（API）调同一个函数，杜绝两套算法漂移。
	//
	// 📚 学习对照：下方 buildPreviewRow 是重构前的逐条查询实现（N+1 版），
	// 已不再被调用，保留用于对比理解"批量化 + 索引化"带来的差异。
	return s.BatchPreview(ctx, userID)
}

// BatchPreview 是下单预览的正式实现（A5 重构后由 Preview 统一委托调用）。\n// 核心价值：把 N+1 次查询压成 3 次（读购物车 + 批量查 SKU + 批量查商品）。
//
// ─ 什么是 N+1，这里怎么消除的 ──
// 重构前：对每条明细单独查 1 次 SKU + 1 次商品（已删除的逐条实现）：
//
//	10 条明细 = 1(读购物车) + 10(查 SKU) + 10(查商品) = 21 次数据库往返 ❌
//
// 本版本改成"先收集 id，再批量查"：
//
//	10 条明细 = 1(读购物车) + 1(批量查 SKU) + 1(批量查商品) = 3 次 ✅
//
// 数据库往返是慢操作（网络 + 解析 + 连接开销），条数越多差距越大。
//
// ─ 内存组装：为什么先转成 map ──
// IN 查询返回的是切片，要按 sku_id 找到对应 SKU 得每次遍历一遍（O(n²)）。
// 先建成 map[id] 对象，查一条就是 O(1)——这是"批量查询"的标准后半段：
// 查询批量化 + 索引内存化，两者缺一不可。
// ✅ 版本说明：这是【批量查询版】（A5 成果），10 条明细只需 3 次数据库往返。
// 与上方 Preview（逐条版）行为完全一致，差异仅在查询次数。
func (s *OrderService) BatchPreview(ctx context.Context, userID uint64) (CartPreview, error) {
	if userID == 0 {
		return CartPreview{}, model.ErrInvalidUserID
	}
	cart, err := s.repository.GetCart(ctx, userID)
	if err != nil {
		return CartPreview{}, fmt.Errorf("查询购物车：%w", err)
	}

	// ── 第一步：只挑勾选项，收集需要查询的 ID ──
	// 注意这里不查商品：SKU 表里已经有 product_id，查回 SKU 后再收集商品 id 即可，
	// 比依赖 cartItem.ProductID 可靠（后者是展示字段，可能没填充）。
	selected := make([]*model.CartItem, 0, len(cart.Items))
	skuIDs := make([]uint64, 0, len(cart.Items))
	for i := range cart.Items {
		item := &cart.Items[i]
		if !item.Selected {
			continue
		}
		selected = append(selected, item)
		skuIDs = append(skuIDs, item.SKUID)
	}
	if len(selected) == 0 {
		return CartPreview{Items: []CartPreviewItem{}}, nil // 没有勾选项：直接返回空预览
	}

	// ─ 第二步：一次查回所有 SKU（1 次往返）──
	skus, err := s.repository.GetSKUByIDs(ctx, skuIDs)
	if err != nil {
		return CartPreview{}, fmt.Errorf("批量查询 SKU：%w", err)
	}
	skuByID := make(map[uint64]model.SKU, len(skus))
	productIDs := make([]uint64, 0, len(skus))
	for _, sku := range skus {
		skuByID[sku.ID] = sku
		productIDs = append(productIDs, sku.ProductID) // 商品 id 从 SKU 里取，来源唯一
	}

	// ─ 第三步：一次查回所有商品（1 次往返）──
	products, err := s.repository.GetProductByIDs(ctx, productIDs)
	if err != nil {
		return CartPreview{}, fmt.Errorf("批量查询商品：%w", err)
	}
	productByID := make(map[uint64]model.Product, len(products))
	for _, product := range products {
		productByID[product.ID] = product
	}

	// ── 第四步：纯内存组装，零数据库调用 ──
	// 判断口径：商品下架 / SKU 停用 / 库存不足 —— 与购物车页 fillItemDisplay 完全一致，
	// 保证"购物车里的灰项"和"预览里被剔除的项"永远是同一批。
	preview := CartPreview{Items: make([]CartPreviewItem, 0, len(selected))}
	for _, item := range selected {
		row := CartPreviewItem{ItemID: item.ID, SKUID: item.SKUID, Quantity: item.Quantity}

		// 用 `sku, ok :=` 双返回值判断"map 里有没有"——IN 查询查不到的 id 不会出现在结果里，
		// 这正是"缺失项"要显式处理的地方（原版是靠 error 判断，这里靠 ok）。
		sku, ok := skuByID[item.SKUID]
		if !ok {
			row.Reason = "SKU 不存在或已删除"
			preview.Items = append(preview.Items, row)
			continue
		}
		product, ok := productByID[sku.ProductID]
		if !ok {
			row.Reason = "商品不存在或已删除"
			preview.Items = append(preview.Items, row)
			continue
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
			preview.TotalCent += row.SubtotalCent // 只有可购项计入总额，与 Preview 口径一致
			preview.PurchasableCount++
		}
		preview.Items = append(preview.Items, row)
	}
	return preview, nil
}

// ─────────────────────────────────────────────────────────────
// 以下 buildPreviewRow 属于【逐条查询版】Preview，与 BatchPreview（批量版）并存，
// 用于对比学习 N+1 问题。生产路径应使用批量版；此版本保留作教学对照。
// ─────────────────────────────────────────────────────────────
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
