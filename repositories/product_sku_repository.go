package repositories

// 商品与 SKU 的数据访问实现（两者强关联：SKU 必属于某商品，一起放便于对照阅读）。
// 基座类型 MySQLRepository 与其构造见 mysql_repository.go。

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

// ListProductsQuery 是 Repository 查询商品列表所需的条件。
type ListProductsQuery struct {
	Page     int
	PageSize int
	Name     string
	Status   model.ProductStatus // 空值不过滤；商城页传 on_sale 只看在售（PRD-001"仅上架商品可被消费者查询"）
}

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
	if query.Status != "" {
		databaseQuery = databaseQuery.Where("status = ?", query.Status)
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

// GetProductByIDs 批量按 ID 查询商品 —— 与 GetSKUByIDs 同款套路，见那里的详细注释。
func (r *MySQLRepository) GetProductByIDs(ctx context.Context, productIDs []uint64) ([]model.Product, error) {
	// ① 空输入必须短路：GORM 会把空切片展开成 IN ()，那是非法 SQL，MySQL 直接报语法错。
	if len(productIDs) == 0 {
		return []model.Product{}, nil
	}
	// ② 一次 IN 查询取回全部：SQL 大致是 SELECT * FROM products WHERE id IN (1,2,3)
	//    对比"循环里逐条 First"：N 次网络往返 → 1 次。这就是消除 N+1 的全部秘密。
	products := make([]model.Product, 0, len(productIDs))
	if err := r.db.WithContext(ctx).Where("id IN ?", productIDs).Find(&products).Error; err != nil {
		return nil, fmt.Errorf("批量查询商品：%w", err)
	}
	// ③ 查不到的 id 不会出现在结果里（可能被软删），调用方需处理"缺项"——
	//    repository 不做兜底填充，因为"找不到该算不存在还是该报错"是业务决策。
	return products, nil
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

type UpdateSKUFields struct {
	Code      *string
	Specs     map[string]string
	PriceCent *int64
	Stock     *int64
}

type ListSKUQuery struct {
	Page, PageSize int
	Name           string
	Status         model.SKUStatus // 空值不过滤；公开入口传 active（消费者只看可售规格）
}

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

// GetSKUByIDs 批量按 ID 查询 SKU —— A5 消除 N+1 的核心武器。
//
// ── 它解决什么问题 ──
// 下单预览原本对购物车每条明细都调一次 GetSKUByID：10 条明细 = 10 次数据库往返。
// 这就是经典的 N+1 查询问题（1 次列表查询 + N 次逐条补充查询）。
// 改成本方法后：收集所有 sku_id → 一次 IN 查回 → 内存里按 id 匹配。10 次变 1 次。
//
// ── GORM 写法要点 ──
//   - Where("id IN ?", ids)：注意是 "IN ?"（问号），不是 "IN (?)"——GORM 见到切片会自动展开成
//     IN (1,2,3) 并做参数绑定（防 SQL 注入）。写成 IN (?) 反而会出错。
//   - 参数名用复数 skuIDs：它接收的是一个"ID 列表"，不是单个 ID（原签名写成了单数，已修正）。
//
// ── 返回值为什么是切片而不是 map ──
// 由接口声明决定（repositories 里写的是 []model.SKU）。"转成 map 方便按 id 取用"是调用方的自由度：
// 想让 service 层决定怎么索引，repository 就只负责取数据——职责边界清晰。
func (r *MySQLRepository) GetSKUByIDs(ctx context.Context, skuIDs []uint64) ([]model.SKU, error) {
	if len(skuIDs) == 0 {
		return []model.SKU{}, nil // 空输入短路，避免生成非法的 IN ()
	}
	skus := make([]model.SKU, 0, len(skuIDs)) // 预分配容量：已知最多这么多条，省去 append 扩容
	if err := r.db.WithContext(ctx).Where("id IN ?", skuIDs).Find(&skus).Error; err != nil {
		return nil, fmt.Errorf("批量查询 SKU：%w", err)
	}
	return skus, nil
}

// ListSKU 查询商品下的 SKU 列表并返回总数。
func (r *MySQLRepository) ListSKU(ctx context.Context, productID uint64, query ListSKUQuery) (int64, []model.SKU, error) {
	items := make([]model.SKU, 0)
	db := r.db.WithContext(ctx).Where("product_id = ?", productID)
	if query.Name != "" {
		db = db.Where("code LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
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

// UpdateProductStatus 推进商品销售状态（草稿↔在售↔下架）。
// 状态合法性由 service 层的状态机把关，这里只负责落库并回读最新值。
func (r *MySQLRepository) UpdateProductStatus(ctx context.Context, productID uint64, status model.ProductStatus) (model.Product, error) {
	result := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ?", productID).
		Update("status", status)
	if result.Error != nil {
		return model.Product{}, fmt.Errorf("更新商品状态：%w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.Product{}, model.ErrProductNotFound
	}
	product, err := r.GetProduct(ctx, productID)
	if err != nil {
		return model.Product{}, err
	}
	return *product, nil
}

// 编译期检查：本文件负责的领域接口是否都实现了。
// 少写方法时错误直接指向这里，而不是 mysql_repository.go 里的全量断言。
var _ ProductRepository = (*MySQLRepository)(nil)
var _ SKURepository = (*MySQLRepository)(nil)
