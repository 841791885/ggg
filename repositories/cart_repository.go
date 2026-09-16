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

// GetCartItemByID 按明细 ID 查询当前用户自己的购物车商品。
func (r *MySQLRepository) GetCartItemByID(ctx context.Context, userID, itemID uint64) (model.CartItem, error) {
	var item model.CartItem
	err := r.db.WithContext(ctx).
		Joins("JOIN carts ON carts.id = cart_items.cart_id AND carts.deleted_at IS NULL").
		Where("cart_items.id = ? AND carts.user_id = ?", itemID, userID).
		First(&item).Error
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

// SetCartSelections 整组替换购物车选中状态：传入的 item_id 置为选中，其余全部取消。
// 目的：选择/取消是"最终集合"语义而非增量开关，两次 UPDATE 覆盖所有行，天然幂等。
func (r *MySQLRepository) SetCartSelections(ctx context.Context, userID uint64, selectedIDs []uint64) ([]model.CartItem, error) {
	cart, err := r.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取购物车：%w", err)
	}
	selected := map[uint64]struct{}{}
	for _, id := range selectedIDs {
		selected[id] = struct{}{}
	}
	items, err := r.GetCartItemsByCart(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		wantSelected := false
		if _, ok := selected[items[i].ID]; ok {
			wantSelected = true
		}
		if items[i].Selected == wantSelected {
			continue // 状态未变化的行跳过更新，减少写放大
		}
		if err := r.db.WithContext(ctx).Model(&model.CartItem{}).Where("id = ?", items[i].ID).Update("selected", wantSelected).Error; err != nil {
			return nil, fmt.Errorf("更新购物车选中状态：%w", err)
		}
		items[i].Selected = wantSelected
	}
	return items, nil
}

// GetCartItemsByCart 查询购物车全部明细。
func (r *MySQLRepository) GetCartItemsByCart(ctx context.Context, cartID uint64) ([]model.CartItem, error) {
	var items []model.CartItem
	if err := r.db.WithContext(ctx).Where("cart_id = ?", cartID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询购物车商品列表：%w", err)
	}
	return items, nil
}

// DeleteCartItem 物理删除当前用户购物车中的指定明细。
// 购物车是临时购买意向，物理删除后同一 SKU 才能再次加入；订单负责保留交易历史。
// 用户条件放进同一条删除语句，避免客户端通过 item_id 删除其他用户的数据。
func (r *MySQLRepository) DeleteCartItem(ctx context.Context, userID, itemID uint64) error {
	userCartIDs := r.db.WithContext(ctx).
		Model(&model.Cart{}).
		Select("id").
		Where("user_id = ?", userID)

	result := r.db.WithContext(ctx).
		Unscoped().
		Where("id = ? AND cart_id IN (?)", itemID, userCartIDs).
		Delete(&model.CartItem{})
	if result.Error != nil {
		return fmt.Errorf("删除购物车商品：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrCartItemNotFound
	}
	return nil
}
