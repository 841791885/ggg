package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	model "ggg/models"
	"ggg/repositories"
)

const (
	minProductNameLength        = 2
	maxProductNameLength        = 100
	maxProductDescriptionLength = 2000
)

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
func (s *ProductService) ListProducts(ctx context.Context, input *ListProductsInput) ([]model.Product, error) {
	log.Printf("[Service] 开始处理商品列表业务：input=%+v", input)

	page := 1
	pageSize := 20
	name := ""

	if input != nil {
		page = input.Page
		if input.PageSize != 0 {
			pageSize = input.PageSize
		}
		name = strings.TrimSpace(input.Name)
	}

	log.Printf("[Service] 商品列表参数处理完成: page=%d page_size=%d name=%q", page, pageSize, name)
	if page < 1 || pageSize < 1 || pageSize > 100 {
		log.Printf("[Service] 商品列表参数校验失败：page=%d page_size=%d", page, pageSize)
		return nil, model.ErrInvalidProductQuery
	}

	products, err := s.repository.ListProducts(ctx, repositories.ListProductsQuery{
		Page:     page,
		PageSize: pageSize,
		Name:     name,
	})
	if err != nil {
		log.Printf("[Service] Repository 查询商品列表失败: %v", err)
		return nil, fmt.Errorf("查询商品列表：%w", err)
	}
	log.Printf("[Service] 商品列表业务处理完成：count=%d", len(products))
	return products, nil
}

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
