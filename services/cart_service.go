package services

import (
	"context"
	"fmt"

	model "ggg/models"
	"ggg/repositories"
)

type AddCartItemInput struct {
	UserID   uint64
	SKUID    uint64
	Quantity int64
}

// UpdateCartItemInput 表示修改购物车商品数量所需的业务参数。
type UpdateCartItemInput struct {
	UserID, ItemID uint64
	Quantity       int64
}

// GetCart 查询用户购物车及全部明细。
func (s *CartService) GetCart(ctx context.Context, userID uint64) (model.Cart, error) {
	if userID == 0 {
		return model.Cart{}, model.ErrInvalidUserID
	}
	cart, err := s.repository.GetCart(ctx, userID)
	if err != nil {
		return model.Cart{}, fmt.Errorf("查询购物车：%w", err)
	}
	// 补齐明细的展示信息（商品名/单价/库存/可购买性）。
	// 注意这里是"每条明细查一次"的写法，购物车通常就几条，够用；
	// 与下单预览同款的 N+1 优化见 docs/ADVANCED-TASKS.md A5。
	for i := range cart.Items {
		s.fillItemDisplay(ctx, &cart.Items[i])
	}
	return cart, nil
}

// fillItemDisplay 给一条购物车明细填充展示字段。任一环节查不到（商品/SKU 被删）
// 就标记为不可购买并给出原因——与下单预览的口径完全一致，保证前后端对"能不能买"的判断同源。
func (s *CartService) fillItemDisplay(ctx context.Context, item *model.CartItem) {
	sku, err := s.repository.GetSKUByID(ctx, item.SKUID)
	if err != nil {
		item.Reason = "SKU 不存在或已删除"
		return
	}
	item.SKUCode = sku.Code
	item.UnitPriceCent = sku.PriceCent
	item.Stock = sku.Stock
	product, err := s.repository.GetProduct(ctx, sku.ProductID)
	if err != nil {
		item.Reason = "商品不存在或已删除"
		return
	}
	item.ProductID = product.ID
	item.ProductName = product.Name
	switch {
	case product.Status != model.ProductStatusOnSale:
		item.Reason = "商品已下架"
	case sku.Status != model.SKUStatusActive:
		item.Reason = "SKU 已停用"
	case item.Quantity > sku.Stock:
		item.Reason = "库存不足"
	default:
		item.Purchasable = true
	}
}

type CartService struct{ repository repositories.CartStore }

// NewCartService 创建购物车业务服务。
func NewCartService(repository repositories.CartStore) *CartService {
	return &CartService{repository: repository}
}

// AddItem 将指定数量的 SKU 加入用户购物车。
func (s *CartService) AddItem(ctx context.Context, input AddCartItemInput) (model.CartItem, error) {
	// 第一步：校验用户、SKU 和购买数量，避免无效请求进入数据库层。
	if input.UserID == 0 {
		return model.CartItem{}, model.ErrInvalidUserID
	}
	if input.SKUID == 0 {
		return model.CartItem{}, model.ErrInvalidSKUID
	}
	if input.Quantity <= 0 {
		return model.CartItem{}, model.ErrInvalidCartQuantity
	}

	// 第二步：查询 SKU，确认它存在、已启用，并且本次购买数量不超过库存。
	sku, err := s.repository.GetSKUByID(ctx, input.SKUID)
	if err != nil {
		return model.CartItem{}, fmt.Errorf("查询 SKU：%w", err)
	}
	if sku.Status != model.SKUStatusActive {
		return model.CartItem{}, model.ErrSKUInactive
	}
	if input.Quantity > sku.Stock {
		return model.CartItem{}, model.ErrInsufficientStock
	}

	// 第三步：获取用户购物车；用户第一次加入商品时会自动创建购物车。
	cart, err := s.repository.GetOrCreateCart(ctx, input.UserID)
	if err != nil {
		return model.CartItem{}, fmt.Errorf("获取购物车：%w", err)
	}

	// 第四步：检查购物车中是否已经有这个 SKU。
	item, err := s.repository.GetCartItem(ctx, cart.ID, input.SKUID)
	if err == nil {
		// 已有明细时累加数量，并校验累加后的总数不能超过库存。
		newQuantity := item.Quantity + input.Quantity
		if newQuantity > sku.Stock {
			return model.CartItem{}, model.ErrInsufficientStock
		}
		return s.repository.UpdateCartItemQuantity(ctx, cart.ID, input.SKUID, newQuantity)
	}
	if err != model.ErrCartItemNotFound {
		return model.CartItem{}, fmt.Errorf("查询购物车商品：%w", err)
	}

	// 没有明细时创建一条新的购物车商品记录。
	return s.repository.CreateCartItem(ctx, model.CartItem{CartID: cart.ID, SKUID: input.SKUID, Quantity: input.Quantity})
}

// UpdateItemQuantity 校验归属、SKU 状态和库存后修改购物车商品数量。
func (s *CartService) UpdateItemQuantity(ctx context.Context, input UpdateCartItemInput) (model.CartItem, error) {
	if input.UserID == 0 {
		return model.CartItem{}, model.ErrInvalidUserID
	}
	if input.ItemID == 0 {
		return model.CartItem{}, model.ErrCartItemNotFound
	}
	if input.Quantity <= 0 {
		return model.CartItem{}, model.ErrInvalidCartQuantity
	}
	item, err := s.repository.GetCartItemByID(ctx, input.UserID, input.ItemID)
	if err != nil {
		return model.CartItem{}, fmt.Errorf("查询购物车商品：%w", err)
	}
	sku, err := s.repository.GetSKUByID(ctx, item.SKUID)
	if err != nil {
		return model.CartItem{}, fmt.Errorf("查询 SKU：%w", err)
	}
	if sku.Status != model.SKUStatusActive {
		return model.CartItem{}, model.ErrSKUInactive
	}
	if input.Quantity > sku.Stock {
		return model.CartItem{}, model.ErrInsufficientStock
	}
	updated, err := s.repository.UpdateCartItemQuantity(ctx, item.CartID, item.SKUID, input.Quantity)
	if err != nil {
		return model.CartItem{}, fmt.Errorf("更新购物车商品数量：%w", err)
	}
	return updated, nil
}

// SetSelection 整组替换购物车选中状态（PRD-004 PUT /cart/selection）。
// item_ids 为期望选中的明细集合，未列出的自动取消选中；空数组表示全部取消。
func (s *CartService) SetSelection(ctx context.Context, userID uint64, itemIDs []uint64) ([]model.CartItem, error) {
	if userID == 0 {
		return nil, model.ErrInvalidUserID
	}
	items, err := s.repository.SetCartSelections(ctx, userID, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("更新购物车选中状态：%w", err)
	}
	return items, nil
}

// RemoveItem 删除当前用户购物车中的指定明细。
func (s *CartService) RemoveItem(ctx context.Context, userID, itemID uint64) error {
	if userID == 0 {
		return model.ErrInvalidUserID
	}
	if itemID == 0 {
		return model.ErrCartItemNotFound
	}
	if err := s.repository.DeleteCartItem(ctx, userID, itemID); err != nil {
		return fmt.Errorf("移除购物车商品：%w", err)
	}
	return nil
}
