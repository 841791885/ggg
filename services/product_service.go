package services

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	model "ggg/models"
	"ggg/repositories"
)

const (
	minProductNameLength        = 2
	maxProductNameLength        = 100
	maxProductDescriptionLength = 2000
	minSKUCodeLength            = 3
	maxSKUCodeLength            = 32
	maxSKUSpecCount             = 5
	maxSKUSpecKeyLength         = 20
	maxSKUSpecValueLength       = 50
)

var skuCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]*$`)

// CreateProductInput 表示创建商品时业务层需要的数据。
//
// 它不是 HTTP Request。Controller 后续会把用户请求转换成这个结构，
// 因此 Service 不需要依赖 Gin。
type CreateProductInput struct {
	Name        string
	Description string
}

type UpdateProductInput struct {
	Name        *string
	Description *string
}

// ListProductsInput 表示查询商品列表时可选的筛选和分页条件。
type ListProductsInput struct {
	Page     int
	PageSize int
	Name     string
	Status   model.ProductStatus // 空值不过滤；消费者入口固定传 on_sale
}

type CreateSKUInput struct {
	ProductID uint64
	Code      string
	Specs     map[string]string
	PriceCent int64
	Stock     int64
}

type ListSKUInput struct {
	Page     int
	PageSize int
	Name     string
	Status   model.SKUStatus // 空值不过滤；公开入口传 active
}

type UpdateSKUInput struct {
	Code      string
	Specs     map[string]string
	PriceCent int64
	Stock     int64
}

// ProductService 负责商品和 SKU 的业务规则。
//
// Service 只依赖 Repository 接口，不关心底层是 MySQL、内存还是测试替身。
type ProductService struct {
	repository repositories.Repository
}

// NewProductService 创建商品业务服务。
func NewProductService(repository repositories.Repository) *ProductService {
	return &ProductService{repository: repository}
}

// CreateProduct 校验并创建商品。
// CreateProduct 校验并创建草稿商品。
func (s *ProductService) CreateProduct(ctx context.Context, input CreateProductInput) (model.Product, error) {
	name := strings.TrimSpace(input.Name)
	nameLength := utf8.RuneCountInString(name)
	if nameLength < minProductNameLength || nameLength > maxProductNameLength {
		return model.Product{}, model.ErrInvalidProductName
	}

	description := strings.TrimSpace(input.Description)
	if utf8.RuneCountInString(description) > maxProductDescriptionLength {
		return model.Product{}, model.ErrInvalidProductDescription
	}

	// 商品创建时只能是草稿，不能由调用方绕过业务规则直接上架。
	product := model.Product{
		Name:        name,
		Description: description,
		Status:      model.ProductStatusDraft,
	}

	createdProduct, err := s.repository.CreateProduct(ctx, product)
	if err != nil {
		return model.Product{}, fmt.Errorf("创建商品：%w", err)
	}
	return createdProduct, nil
}

// ListProducts 查询商品列表；input 为 nil 时使用全部默认值。
func (s *ProductService) ListProducts(ctx context.Context, input *ListProductsInput) (int64, []model.Product, error) {
	zap.S().Debugf("开始处理商品列表业务：input=%+v", input)

	page := 1
	pageSize := 20
	name := ""
	status := model.ProductStatus("") // 空值=不过滤（管理端看全部）；消费者入口固定 on_sale

	if input != nil {
		page = input.Page
		if input.PageSize != 0 {
			pageSize = input.PageSize
		}
		name = strings.TrimSpace(input.Name)
		status = input.Status
	}

	zap.S().Debugf("商品列表参数处理完成: page=%d page_size=%d name=%q", page, pageSize, name)
	if page < 1 || pageSize < 1 || pageSize > 100 {
		zap.S().Warnf("商品列表参数校验失败：page=%d page_size=%d", page, pageSize)
		return 0, nil, model.ErrInvalidProductQuery
	}

	total, products, err := s.repository.ListProducts(ctx, repositories.ListProductsQuery{
		Page:     page,
		PageSize: pageSize,
		Name:     name,
		Status:   status,
	})
	if err != nil {
		zap.S().Errorf("[Service] Repository 查询商品列表失败: %v", err)
		return 0, nil, fmt.Errorf("查询商品列表：%w", err)
	}
	zap.S().Debugf("商品列表业务处理完成：count=%d", len(products))
	return total, products, nil
}

// GetProduct 根据商品 ID 查询商品。
// GetProduct 根据商品 ID 查询单个商品。
func (s *ProductService) GetProduct(ctx context.Context, productID uint64) (*model.Product, error) {
	product, err := s.repository.GetProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("查询商品：%w", err)
	}
	return product, nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, productID uint64, info UpdateProductInput) (model.Product, error) {
	if info.Name == nil && info.Description == nil {
		return model.Product{}, model.ErrEmptyProductUpdate
	}

	fields := repositories.UpdateProductFields{}

	if info.Name != nil {
		name := strings.TrimSpace(*info.Name)
		nameLength := utf8.RuneCountInString(name)
		if nameLength < minProductNameLength || nameLength > maxProductNameLength {
			return model.Product{}, model.ErrInvalidProductName
		}
		fields.Name = &name
	}

	if info.Description != nil {
		description := strings.TrimSpace(*info.Description)
		if utf8.RuneCountInString(description) > maxProductDescriptionLength {
			return model.Product{}, model.ErrInvalidProductDescription
		}
		fields.Description = &description
	}

	updatedProduct, err := s.repository.UpdateProduct(ctx, productID, fields)
	if err != nil {
		return model.Product{}, fmt.Errorf("更新商品：%w", err)
	}
	return updatedProduct, nil
}

// DeleteProduct 软删除商品，并阻止删除在售商品。
func (s *ProductService) DeleteProduct(ctx context.Context, productID uint64) error {
	product, err := s.repository.GetProduct(ctx, productID)
	if err != nil {
		return fmt.Errorf("查询待删除商品：%w", err)
	}
	if product.Status == model.ProductStatusOnSale {
		return model.ErrProductOnSale
	}

	if err := s.repository.DeleteProduct(ctx, productID); err != nil {
		return fmt.Errorf("删除商品：%w", err)
	}
	return nil
}

func (s *ProductService) CreateSKU(ctx context.Context, input CreateSKUInput) (model.SKU, error) {
	if input.ProductID == 0 {
		return model.SKU{}, model.ErrInvalidProductID
	}

	code := strings.TrimSpace(input.Code)
	codeLength := utf8.RuneCountInString(code)
	if codeLength < minSKUCodeLength || codeLength > maxSKUCodeLength || !skuCodePattern.MatchString(code) {
		return model.SKU{}, model.ErrInvalidSKUCode
	}

	if len(input.Specs) < 1 || len(input.Specs) > maxSKUSpecCount {
		return model.SKU{}, model.ErrInvalidSKUSpecs
	}
	specs := make(map[string]string, len(input.Specs))
	for key, value := range input.Specs {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || utf8.RuneCountInString(key) > maxSKUSpecKeyLength ||
			value == "" || utf8.RuneCountInString(value) > maxSKUSpecValueLength {
			return model.SKU{}, model.ErrInvalidSKUSpecs
		}
		specs[key] = value
	}

	if input.PriceCent <= 0 {
		return model.SKU{}, model.ErrInvalidSKUPrice
	}
	if input.Stock < 0 {
		return model.SKU{}, model.ErrInvalidSKUStock
	}

	if _, err := s.repository.GetProduct(ctx, input.ProductID); err != nil {
		return model.SKU{}, fmt.Errorf("查询 SKU 所属商品：%w", err)
	}

	sku, err := s.repository.CreateSKU(ctx, model.SKU{
		ProductID: input.ProductID,
		Code:      code,
		Specs:     specs,
		PriceCent: input.PriceCent,
		Stock:     input.Stock,
		Status:    model.SKUStatusActive,
	})
	if err != nil {
		return model.SKU{}, fmt.Errorf("创建 SKU：%w", err)
	}
	return sku, nil
}

// GetSKU 查询指定商品下的 SKU。
func (s *ProductService) GetSKU(ctx context.Context, productID, skuID uint64) (model.SKU, error) {
	if productID == 0 || skuID == 0 {
		return model.SKU{}, model.ErrInvalidProductID
	}
	sku, err := s.repository.GetSKU(ctx, productID, skuID)
	if err != nil {
		return model.SKU{}, fmt.Errorf("查询 SKU：%w", err)
	}
	return sku, nil
}

func (s *ProductService) ListSKU(ctx context.Context, productID uint64, input *ListSKUInput) (int64, []model.SKU, error) {
	if productID == 0 {
		return 0, nil, model.ErrInvalidProductID
	}
	page, pageSize, name := 1, 20, ""
	status := model.SKUStatus("") // 空值=不过滤（管理端看全部）；公开入口传 active
	if input != nil {
		if input.Page != 0 {
			page = input.Page
		}
		if input.PageSize != 0 {
			pageSize = input.PageSize
		}
		name = strings.TrimSpace(input.Name)
		status = input.Status
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return 0, nil, model.ErrInvalidProductQuery
	}
	total, items, err := s.repository.ListSKU(ctx, productID, repositories.ListSKUQuery{Page: page, PageSize: pageSize, Name: name, Status: status})
	if err != nil {
		return 0, nil, fmt.Errorf("查询 SKU 列表：%w", err)
	}
	return total, items, nil
}

// UpdateSKU 校验并部分更新 SKU 信息。
func (s *ProductService) UpdateSKU(ctx context.Context, productID, skuID uint64, input *UpdateSKUInput) (model.SKU, error) {
	if productID == 0 || skuID == 0 || input == nil {
		return model.SKU{}, model.ErrInvalidProductID
	}
	fields := repositories.UpdateSKUFields{}
	if input.Code != "" {
		code := strings.TrimSpace(input.Code)
		if !skuCodePattern.MatchString(code) || utf8.RuneCountInString(code) < minSKUCodeLength || utf8.RuneCountInString(code) > maxSKUCodeLength {
			return model.SKU{}, model.ErrInvalidSKUCode
		}
		fields.Code = &code
	}
	if input.Specs != nil {
		if len(input.Specs) < 1 || len(input.Specs) > maxSKUSpecCount {
			return model.SKU{}, model.ErrInvalidSKUSpecs
		}
		specs := make(map[string]string, len(input.Specs))
		for key, value := range input.Specs {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || utf8.RuneCountInString(key) > maxSKUSpecKeyLength ||
				value == "" || utf8.RuneCountInString(value) > maxSKUSpecValueLength {
				return model.SKU{}, model.ErrInvalidSKUSpecs
			}
			specs[key] = value
		}
		fields.Specs = specs
	}
	if input.PriceCent != 0 {
		if input.PriceCent <= 0 {
			return model.SKU{}, model.ErrInvalidSKUPrice
		}
		fields.PriceCent = &input.PriceCent
	}
	if input.Stock != 0 {
		if input.Stock < 0 {
			return model.SKU{}, model.ErrInvalidSKUStock
		}
		fields.Stock = &input.Stock
	}
	item, err := s.repository.UpdateSKU(ctx, productID, skuID, fields)
	if err != nil {
		return model.SKU{}, fmt.Errorf("更新 SKU：%w", err)
	}
	return item, nil
}

func (s *ProductService) DeleteSKU(ctx context.Context, productID, skuID uint64) error {
	if productID == 0 || skuID == 0 {
		return model.ErrInvalidProductID
	}
	if err := s.repository.DeleteSKU(ctx, productID, skuID); err != nil {
		return fmt.Errorf("删除 SKU：%w", err)
	}
	return nil
}

// UpdateSKUStatus 修改 SKU 的启用状态。
func (s *ProductService) UpdateSKUStatus(ctx context.Context, productID, skuID uint64, status model.SKUStatus) (model.SKU, error) {
	if productID == 0 || skuID == 0 {
		return model.SKU{}, model.ErrInvalidProductID
	}
	if status != model.SKUStatusActive && status != model.SKUStatusInactive {
		return model.SKU{}, model.ErrInvalidSKUStatus
	}
	sku, err := s.repository.UpdateSKUStatus(ctx, productID, skuID, status)
	if err != nil {
		return model.SKU{}, fmt.Errorf("更新 SKU 状态：%w", err)
	}
	return sku, nil
}

// productStatusTransitions 商品状态机：草稿→在售、在售↔下架、任意→草稿。
// 为什么需要状态机而不是随便 set：草稿商品不该被消费者看见，下架再上架是常规运营动作，
// 但"下架→在售"必须允许（否则下架成了单向死刑），而"在售→草稿"要禁止（已有消费者看过，
// 回草稿会造成语义混乱）。规则集中在这里，controller 和 repository 都不许自行判断。
var productStatusTransitions = map[model.ProductStatus][]model.ProductStatus{
	model.ProductStatusDraft:   {model.ProductStatusOnSale},
	model.ProductStatusOnSale:  {model.ProductStatusOffSale, model.ProductStatusDraft},
	model.ProductStatusOffSale: {model.ProductStatusOnSale},
}

// UpdateStatus 推进商品销售状态，非法迁移返回 ErrInvalidProductQuery（复用 400 语义）。
func (s *ProductService) UpdateStatus(ctx context.Context, productID uint64, status model.ProductStatus) (model.Product, error) {
	if productID == 0 {
		return model.Product{}, model.ErrInvalidProductID
	}
	product, err := s.repository.GetProduct(ctx, productID)
	if err != nil {
		return model.Product{}, err
	}
	if !slices.Contains(productStatusTransitions[product.Status], status) {
		return model.Product{}, model.ErrInvalidProductTransition
	}
	updated, err := s.repository.UpdateProductStatus(ctx, productID, status)
	if err != nil {
		return model.Product{}, fmt.Errorf("更新商品状态：%w", err)
	}
	return updated, nil
}
