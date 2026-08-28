package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	model "ggg/models"
	"ggg/services"

	"github.com/gin-gonic/gin"
)

type productService interface {
	CreateProduct(ctx context.Context, input services.CreateProductInput) (model.Product, error)
	ListProducts(ctx context.Context, input *services.ListProductsInput) ([]model.Product, error)
	GetProduct(ctx context.Context, productID uint64) (*model.Product, error)
	UpdateProduct(ctx context.Context, productID uint64, input services.UpdateProductInput) (model.Product, error)
	DeleteProduct(ctx context.Context, productID uint64) error
}

// ProductController 负责商品 HTTP 请求和响应，不实现商品业务规则。
type ProductController struct {
	service productService
}

// NewProductController 创建商品控制器。
func NewProductController(service productService) *ProductController {
	return &ProductController{service: service}
}

// CreateProduct 处理 POST /api/v1/admin/products。
func (p *ProductController) CreateProduct(c *gin.Context) {
	var request CreateProductRequest
	// ShouldBindJSON 是 Gin 提供的参数绑定方法，会把请求体中的 JSON 解析到 request。
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"请求 JSON 格式不正确",
		)
		return
	}

	// Gin Context 只留在 Controller，向业务层传递标准库 Request Context。
	product, err := p.service.CreateProduct(c.Request.Context(), services.CreateProductInput{
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductName) ||
			errors.Is(err, model.ErrInvalidProductDescription) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("创建商品失败: %v", err)
		respondError(
			c,
			http.StatusInternalServerError,
			"服务器内部错误",
		)
		return
	}
	log.Printf("[Controller] 商品列表参数绑定失败: %v", product)

	respondSuccess(c, http.StatusCreated, newProductResponse(&product))
}

// ListProducts 处理 GET /api/v1/admin/products。
func (p *ProductController) ListProducts(c *gin.Context) {
	log.Printf("[Controller] 开始处理商品列表请求: query=%q", c.Request.URL.RawQuery)

	var query ListProductsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		log.Printf("[Controller] 商品列表参数绑定失败: %v", err)
		respondError(c, http.StatusBadRequest, "查询参数格式不正确")
		return
	}

	// 没有传任何查询参数时传 nil，让 Service 统一使用默认查询条件。
	var input *services.ListProductsInput
	if query.Page != nil || query.PageSize != nil || query.Name != nil {
		input = &services.ListProductsInput{}
		if query.Page != nil {
			input.Page = *query.Page
		}
		if query.PageSize != nil {
			input.PageSize = *query.PageSize
		}
		if query.Name != nil {
			input.Name = *query.Name
		}
	}

	log.Printf("[Controller] 调用 ProductService.ListProducts: input=%+v", input)
	products, err := p.service.ListProducts(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductQuery) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("查询商品列表失败: %v", err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	// 已知最终长度时一次性分配，按索引转换，避免 append 扩容和 range 元素拷贝。
	response := make([]ProductResponse, len(products))
	for i := range products {
		response[i] = newProductResponse(&products[i])
	}
	log.Printf("[Controller] 商品列表请求处理完成：count=%d", len(response))
	respondSuccess(c, http.StatusOK, products)
}

// GetProduct 处理 GET /api/v1/admin/products/:product_id。
func (p *ProductController) GetProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}

	product, err := p.service.GetProduct(c.Request.Context(), productID)
	if err != nil {
		if errors.Is(err, model.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}

		log.Printf("查询商品失败: product_id=%d err=%v", productID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, newProductResponse(product))
}

// UpdateProduct 处理 PATCH /api/v1/admin/products/:product_id。
func (p *ProductController) UpdateProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}

	var request UpdateProductRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}

	product, err := p.service.UpdateProduct(c.Request.Context(), productID, services.UpdateProductInput{
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		if errors.Is(err, model.ErrEmptyProductUpdate) ||
			errors.Is(err, model.ErrInvalidProductName) ||
			errors.Is(err, model.ErrInvalidProductDescription) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, model.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}

		log.Printf("更新商品失败: product_id=%d err=%v", productID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	respondSuccess(c, http.StatusOK, newProductResponse(&product))
}

func (p *ProductController) DeleteProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}

	if err := p.service.DeleteProduct(c.Request.Context(), productID); err != nil {
		if errors.Is(err, model.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, model.ErrProductOnSale) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}

		log.Printf("删除商品失败: product_id=%d err=%v", productID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	c.Status(http.StatusNoContent)
}
