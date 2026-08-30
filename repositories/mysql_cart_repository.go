package repositories

import (
	"context"
	"errors"
	"fmt"

	model "ggg/models"
	"gorm.io/gorm"
)

// GetOrCreateCart 获取用户购物车；用户首次操作时自动创建。
func (r *MySQLRepository) GetOrCreateCart(ctx context.Context, userID uint64) (model.Cart, error) {
	var cart model.Cart
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&cart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cart = model.Cart{UserID: userID}
		if err := r.db.WithContext(ctx).Create(&cart).Error; err != nil {
			return model.Cart{}, fmt.Errorf("创建购物车：%w", err)
		}
		return cart, nil
	}
	if err != nil {
		return model.Cart{}, fmt.Errorf("查询购物车：%w", err)
	}
	return cart, nil
}

// GetCart 查询用户购物车及其中的商品明细。
func (r *MySQLRepository) GetCart(ctx context.Context, userID uint64) (model.Cart, error) {
	var cart model.Cart
	err := r.db.WithContext(ctx).Preload("Items").Where("user_id = ?", userID).First(&cart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Cart{UserID: userID, Items: []model.CartItem{}}, nil
	}
	if err != nil {
		return model.Cart{}, fmt.Errorf("查询购物车：%w", err)
	}
	return cart, nil
}

// GetCartItem 查询购物车中指定 SKU 的明细。
func (r *MySQLRepository) GetCartItem(ctx context.Context, cartID, skuID uint64) (model.CartItem, error) {
	var item model.CartItem
	err := r.db.WithContext(ctx).Where("cart_id = ? AND sku_id = ?", cartID, skuID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CartItem{}, model.ErrCartItemNotFound
	}
	if err != nil {
		return model.CartItem{}, fmt.Errorf("查询购物车商品：%w", err)
	}
	return item, nil
}

// CreateCartItem 新增购物车明细。
func (r *MySQLRepository) CreateCartItem(ctx context.Context, item model.CartItem) (model.CartItem, error) {
	if err := r.db.WithContext(ctx).Create(&item).Error; err != nil {
		return model.CartItem{}, fmt.Errorf("创建购物车商品：%w", err)
	}
	return item, nil
}

// UpdateCartItemQuantity 更新购物车明细数量。
func (r *MySQLRepository) UpdateCartItemQuantity(ctx context.Context, cartID, skuID uint64, quantity int64) (model.CartItem, error) {
	result := r.db.WithContext(ctx).Model(&model.CartItem{}).Where("cart_id = ? AND sku_id = ?", cartID, skuID).Update("quantity", quantity)
	if result.Error != nil {
		return model.CartItem{}, fmt.Errorf("更新购物车商品数量：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.CartItem{}, model.ErrCartItemNotFound
	}
	return r.GetCartItem(ctx, cartID, skuID)
}
