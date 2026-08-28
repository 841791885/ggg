package repositories

import (
	"context"
	"errors"
	"fmt"
	"log"

	model "ggg/models"

	"gorm.io/gorm"
)

// Repository 定义当前阶段业务层需要的数据持久化能力。
// 业务层依赖此接口，不直接依赖下面的 MySQLRepository 实现。
type Repository interface {
	CreateProduct(ctx context.Context, product model.Product) (model.Product, error)
	ListProducts(ctx context.Context, query ListProductsQuery) ([]model.Product, error)
	GetProduct(ctx context.Context, productID uint64) (*model.Product, error)
	UpdateProduct(ctx context.Context, productID uint64, fields UpdateProductFields) (model.Product, error)
	DeleteProduct(ctx context.Context, productID uint64) error
}

// UpdateProductFields 表示本次需要更新的商品字段，nil 表示不更新该字段。
type UpdateProductFields struct {
	Name        *string
	Description *string
}

// ListProductsQuery 是 Repository 查询商品列表所需的条件。
type ListProductsQuery struct {
	Page     int
	PageSize int
	Name     string
}

// MySQLRepository 使用 GORM 将商品保存到 MySQL。
type MySQLRepository struct {
	db *gorm.DB
}

// 编译期检查 MySQLRepository 是否完整实现了 Repository 接口。
var _ Repository = (*MySQLRepository)(nil)

// NewMySQLRepository 创建 MySQL 商品仓库。
func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// CreateProduct 创建商品，ID 和时间由 MySQL 生成。
func (r *MySQLRepository) CreateProduct(ctx context.Context, product model.Product) (model.Product, error) {
	// 对应 SQL（具体字段顺序由 GORM 生成）：
	// INSERT INTO products (name, description, status, created_at, updated_at, deleted_at)
	// VALUES (?, ?, ?, ?, ?, ?);
	// MySQL 生成自增 ID 后，GORM 会把 ID 回填到 product.ID。
	if err := r.db.WithContext(ctx).Create(&product).Error; err != nil {
		return model.Product{}, fmt.Errorf("创建商品：%w", err)
	}
	return product, nil
}

// ListProducts 按名称筛选并分页查询商品，Page 从 1 开始。
func (r *MySQLRepository) ListProducts(ctx context.Context, query ListProductsQuery) ([]model.Product, error) {
	offset := (query.Page - 1) * query.PageSize
	log.Printf(
		"[Repository] 开始查询商品列表: page=%d page_size=%d offset=%d name=%q",
		query.Page,
		query.PageSize,
		offset,
		query.Name,
	)

	databaseQuery := r.db.WithContext(ctx).Model(&model.Product{})
	if query.Name != "" {
		databaseQuery = databaseQuery.Where("name LIKE ?", "%"+query.Name+"%")
	}

	products := make([]model.Product, 0)
	// 对应 SQL；设置了 Name 时会多出 name LIKE ? 条件：
	// SELECT * FROM products
	// WHERE deleted_at IS NULL [AND name LIKE ?]
	// ORDER BY created_at DESC
	// LIMIT ? OFFSET ?;
	// DeletedAt 使用了 gorm.DeletedAt，因此 deleted_at IS NULL 由 GORM 自动添加。
	if err := databaseQuery.
		Order("created_at DESC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&products).Error; err != nil {
		log.Printf("[Repository] MySQL 查询商品列表失败: %v", err)
		return nil, fmt.Errorf("查询商品列表：%w", err)
	}
	log.Printf("[Repository] MySQL 查询商品列表完成：count=%d", len(products))
	log.Printf("[Repository] 查询到的商品：%+v", products)
	return products, nil
}

// GetProduct 根据商品 ID 查询单个商品。
func (r *MySQLRepository) GetProduct(ctx context.Context, productID uint64) (*model.Product, error) {
	var product model.Product
	// 对应 SQL：
	// SELECT * FROM products
	// WHERE id = ? AND deleted_at IS NULL
	// ORDER BY id LIMIT 1;
	// 第二个参数 productID 会绑定到主键条件中的 ?，查询结果写入 product。
	if err := r.db.WithContext(ctx).First(&product, productID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrProductNotFound
		}
		return nil, fmt.Errorf("查询商品：%w", err)
	}
	return &product, nil
}

func (r *MySQLRepository) UpdateProduct(ctx context.Context, productID uint64, fields UpdateProductFields) (model.Product, error) {
	updates := make(map[string]any, 2)
	if fields.Name != nil {
		updates["name"] = *fields.Name
	}
	if fields.Description != nil {
		updates["description"] = *fields.Description
	}
	if len(updates) == 0 {
		return model.Product{}, model.ErrEmptyProductUpdate
	}

	// 只传 name 时对应 SQL：
	// UPDATE products SET name = ?, updated_at = ?
	// WHERE id = ? AND deleted_at IS NULL;
	// description 同理；两个字段都传时会同时出现在 SET 中。
	result := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", productID).
		Updates(updates)
	if result.Error != nil {
		return model.Product{}, fmt.Errorf("更新商品：%w", result.Error)
	}

	// UPDATE 后查询一次：既返回最新数据，也能在目标不存在时得到 ErrProductNotFound。
	product, err := r.GetProduct(ctx, productID)
	if err != nil {
		return model.Product{}, err
	}
	return *product, nil
}

func (r *MySQLRepository) DeleteProduct(ctx context.Context, productID uint64) error {
	// 对应 SQL（Product 使用 gorm.DeletedAt，所以执行软删除）：
	// UPDATE products SET deleted_at = ?
	// WHERE id = ? AND deleted_at IS NULL;
	result := r.db.WithContext(ctx).Delete(&model.Product{}, productID)
	if result.Error != nil {
		return fmt.Errorf("删除商品：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrProductNotFound
	}
	return nil
}
