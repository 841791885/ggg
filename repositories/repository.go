package repositories

import (
	"context"

	model "ggg/models"
)

// ProductRepository 定义商品模块需要的数据持久化能力。
type ProductRepository interface {
	CreateProduct(context.Context, model.Product) (model.Product, error)
	ListProducts(context.Context, ListProductsQuery) (int64, []model.Product, error)
	GetProduct(context.Context, uint64) (*model.Product, error)
	UpdateProduct(context.Context, uint64, UpdateProductFields) (model.Product, error)
	DeleteProduct(context.Context, uint64) error
}

// SKURepository 定义 SKU 模块需要的数据持久化能力。
type SKURepository interface {
	CreateSKU(context.Context, model.SKU) (model.SKU, error)
	GetSKU(context.Context, uint64, uint64) (model.SKU, error)
	GetSKUByID(context.Context, uint64) (model.SKU, error)
	ListSKU(context.Context, uint64, ListSKUQuery) (int64, []model.SKU, error)
	UpdateSKU(context.Context, uint64, uint64, UpdateSKUFields) (model.SKU, error)
	DeleteSKU(context.Context, uint64, uint64) error
	UpdateSKUStatus(context.Context, uint64, uint64, model.SKUStatus) (model.SKU, error)
}

// CartRepository 定义购物车模块需要的数据持久化能力。
type CartRepository interface {
	GetOrCreateCart(context.Context, uint64) (model.Cart, error)
	GetCart(context.Context, uint64) (model.Cart, error)
	GetCartItem(context.Context, uint64, uint64) (model.CartItem, error)
	GetCartItemByID(context.Context, uint64, uint64) (model.CartItem, error)
	CreateCartItem(context.Context, model.CartItem) (model.CartItem, error)
	UpdateCartItemQuantity(context.Context, uint64, uint64, int64) (model.CartItem, error)
	DeleteCartItem(context.Context, uint64, uint64) error
}

// UserRepository 定义用户模块需要的数据持久化能力。
type UserRepository interface {
	GetUserByLogin(context.Context, string) (model.User, error)
	GetUserByUsername(context.Context, string) (model.User, error)
	GetUserByEmail(context.Context, string) (model.User, error)
	CreateUser(context.Context, model.User) (model.User, error)
}

// Repository 是当前 MySQL 仓库的完整能力集合，便于旧 Service 统一注入。
type Repository interface {
	ProductRepository
	SKURepository
	CartRepository
	UserRepository
}
