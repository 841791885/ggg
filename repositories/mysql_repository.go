package repositories

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"

	model "ggg/models"

	"gorm.io/gorm"
)

// UpdateProductFields 表示本次需要更新的商品字段，nil 表示不更新该字段。
type UpdateProductFields struct {
	Name        *string
	Description *string
}

type UpdateSKUFields struct {
	Code      *string
	Specs     map[string]string
	PriceCent *int64
	Stock     *int64
}

// ListProductsQuery 是 Repository 查询商品列表所需的条件。
type ListProductsQuery struct {
	Page     int
	PageSize int
	Name     string
}

type ListSKUQuery struct {
	Page, PageSize int
	Name           string
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

// CreateProduct 将商品写入 products 表，并返回数据库生成的字段。
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
func (r *MySQLRepository) ListProducts(ctx context.Context, query ListProductsQuery) (int64, []model.Product, error) {
	offset := (query.Page - 1) * query.PageSize
	zap.S().Debugf(
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
	var total int64
	if err := databaseQuery.Model(&model.Product{}).Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计商品：%w", err)
	}
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
		zap.S().Errorf("[Repository] MySQL 查询商品列表失败: %v", err)
		return 0, nil, fmt.Errorf("查询商品列表：%w", err)
	}
	zap.S().Debugf("MySQL 查询商品列表完成：count=%d", len(products))
	zap.S().Debugf("查询到的商品：%+v", products)
	return total, products, nil
}

// GetProduct 从数据库读取一个未软删除的商品。
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

// DeleteProduct 对商品执行软删除。
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

// CreateSKU 使用 GORM 创建 SKU。
func (r *MySQLRepository) CreateSKU(ctx context.Context, sku model.SKU) (model.SKU, error) {
	// 对应 SQL：
	// INSERT INTO skus (
	//     product_id, code, specs, price_cent, stock, status,
	//     created_at, updated_at, deleted_at
	// ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	// MySQL 生成自增 ID 后，GORM 会把 ID 回填到 sku.ID。
	if err := r.db.WithContext(ctx).Create(&sku).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.SKU{}, model.ErrSKUCodeConflict
		}
		return model.SKU{}, fmt.Errorf("创建 SKU: %w", err)
	}
	return sku, nil
}

// GetSKU 查询指定商品下的单个 SKU。
func (r *MySQLRepository) GetSKU(ctx context.Context, productID, skuID uint64) (model.SKU, error) {
	var sku model.SKU
	err := r.db.WithContext(ctx).Where("id = ? AND product_id = ?", skuID, productID).First(&sku).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SKU{}, model.ErrSKUNotFound
	}
	if err != nil {
		return model.SKU{}, fmt.Errorf("查询 SKU：%w", err)
	}
	return sku, nil
}

func (r *MySQLRepository) GetSKUByID(ctx context.Context, skuID uint64) (model.SKU, error) {
	var sku model.SKU
	err := r.db.WithContext(ctx).First(&sku, skuID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SKU{}, model.ErrSKUNotFound
	}
	if err != nil {
		return model.SKU{}, fmt.Errorf("查询 SKU：%w", err)
	}
	return sku, nil
}

// ListSKU 查询商品下的 SKU 列表并返回总数。
func (r *MySQLRepository) ListSKU(ctx context.Context, productID uint64, query ListSKUQuery) (int64, []model.SKU, error) {
	items := make([]model.SKU, 0)
	db := r.db.WithContext(ctx).Where("product_id = ?", productID)
	if query.Name != "" {
		db = db.Where("code LIKE ?", "%"+query.Name+"%")
	}
	var total int64
	if err := db.Model(&model.SKU{}).Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计 SKU：%w", err)
	}
	err := db.Order("created_at DESC").Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Find(&items).Error
	if err != nil {
		return 0, nil, fmt.Errorf("查询 SKU 列表：%w", err)
	}
	return total, items, nil
}

func (r *MySQLRepository) UpdateSKU(ctx context.Context, productID, skuID uint64, fields UpdateSKUFields) (model.SKU, error) {
	updates := make(map[string]any)
	if fields.Code != nil {
		updates["code"] = *fields.Code
	}
	if fields.Specs != nil {
		updates["specs"] = fields.Specs
	}
	if fields.PriceCent != nil {
		updates["price_cent"] = *fields.PriceCent
	}
	if fields.Stock != nil {
		updates["stock"] = *fields.Stock
	}
	if len(updates) == 0 {
		return model.SKU{}, model.ErrEmptySKUUpdate
	}
	result := r.db.WithContext(ctx).Model(&model.SKU{}).Where("id = ? AND product_id = ?", skuID, productID).Updates(updates)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return model.SKU{}, model.ErrSKUCodeConflict
		}
		return model.SKU{}, fmt.Errorf("更新 SKU：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.SKU{}, model.ErrSKUNotFound
	}
	return r.GetSKU(ctx, productID, skuID)
}

// DeleteSKU 软删除指定商品下的 SKU。
func (r *MySQLRepository) DeleteSKU(ctx context.Context, productID, skuID uint64) error {
	result := r.db.WithContext(ctx).Where("id = ? AND product_id = ?", skuID, productID).Delete(&model.SKU{})
	if result.Error != nil {
		return fmt.Errorf("删除 SKU：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ErrSKUNotFound
	}
	return nil
}

func (r *MySQLRepository) UpdateSKUStatus(ctx context.Context, productID, skuID uint64, status model.SKUStatus) (model.SKU, error) {
	result := r.db.WithContext(ctx).Model(&model.SKU{}).
		Where("id = ? AND product_id = ?", skuID, productID).
		Update("status", status)
	if result.Error != nil {
		return model.SKU{}, fmt.Errorf("更新 SKU 状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.SKU{}, model.ErrSKUNotFound
	}
	return r.GetSKU(ctx, productID, skuID)
}
