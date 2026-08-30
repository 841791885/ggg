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

// GetCart 查询用户购物车及全部明细。
func (s *CartService) GetCart(ctx context.Context, userID uint64) (model.Cart, error) {
	if userID == 0 {
		return model.Cart{}, model.ErrInvalidUserID
	}
	cart, err := s.repository.GetCart(ctx, userID)
	if err != nil {
		return model.Cart{}, fmt.Errorf("查询购物车：%w", err)
	}
	return cart, nil
}

type CartService struct{ repository repositories.Repository }

// NewCartService 创建购物车业务服务。
func NewCartService(repository repositories.Repository) *CartService {
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
