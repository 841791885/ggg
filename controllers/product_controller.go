package controllers

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"net/http"
	"strconv"

	model "ggg/models"
	"ggg/services"

	"github.com/gin-gonic/gin"
)

type productService interface {
	CreateProduct(ctx context.Context, input services.CreateProductInput) (model.Product, error)
	ListProducts(ctx context.Context, input *services.ListProductsInput) (int64, []model.Product, error)
	GetProduct(ctx context.Context, productID uint64) (*model.Product, error)
	UpdateProduct(ctx context.Context, productID uint64, input services.UpdateProductInput) (model.Product, error)
	DeleteProduct(ctx context.Context, productID uint64) error
	UpdateStatus(ctx context.Context, productID uint64, status model.ProductStatus) (model.Product, error)

	CreateSKU(ctx context.Context, input services.CreateSKUInput) (model.SKU, error)
	GetSKU(ctx context.Context, productID, skuID uint64) (model.SKU, error)
	ListSKU(ctx context.Context, productID uint64, input *services.ListSKUInput) (int64, []model.SKU, error)
	UpdateSKU(ctx context.Context, productID, skuID uint64, input *services.UpdateSKUInput) (model.SKU, error)
	DeleteSKU(ctx context.Context, productID, skuID uint64) error
	UpdateSKUStatus(ctx context.Context, productID, skuID uint64, status model.SKUStatus) (model.SKU, error)
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

		zap.S().Errorf("创建商品失败: %v", err)
		respondError(
			c,
			http.StatusInternalServerError,
			"服务器内部错误",
		)
		return
	}
	zap.S().Debugf("商品创建完成: product_id=%d", product.ID)

	respondSuccess(c, http.StatusCreated, newProductResponse(&product))
}

// ListProducts 处理 GET /api/v1/admin/products。
func (p *ProductController) ListProducts(c *gin.Context) {
	zap.S().Debugf("开始处理商品列表请求: query=%q", c.Request.URL.RawQuery)

	var query ListProductsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		zap.S().Errorf("[Controller] 商品列表参数绑定失败: %v", err)
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

	zap.S().Debugf("调用 ProductService.ListProducts: input=%+v", input)
	total, products, err := p.service.ListProducts(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductQuery) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		zap.S().Errorf("查询商品列表失败: %v", err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	// 已知最终长度时一次性分配，按索引转换，避免 append 扩容和 range 元素拷贝。
	response := make([]ProductResponse, len(products))
	for i := range products {
		response[i] = newProductResponse(&products[i])
	}
	zap.S().Debugf("商品列表请求处理完成: count=%d", len(response))
	respondSuccess(c, http.StatusOK, gin.H{"total": total, "list": response})
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

		zap.S().Errorf("查询商品失败: product_id=%d err=%v", productID, err)
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

		zap.S().Errorf("更新商品失败: product_id=%d err=%v", productID, err)
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

		zap.S().Errorf("删除商品失败: product_id=%d err=%v", productID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	c.Status(http.StatusNoContent)
}

// CreateSKU 处理 POST /api/v1/admin/products/:product_id/skus。
func (p *ProductController) CreateSKU(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}

	var request CreateSKURequest
	// ShouldBindJSON 是 Gin 提供的参数绑定方法，会把请求体中的 JSON 解析到 request。
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"请求 JSON 格式不正确",
		)
		return
	}

	sku, err := p.service.CreateSKU(c.Request.Context(), services.CreateSKUInput{
		ProductID: productID,
		Code:      request.Code,
		Specs:     request.Specs,
		PriceCent: request.PriceCent,
		Stock:     request.Stock,
	})
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductID) ||
			errors.Is(err, model.ErrInvalidSKUCode) ||
			errors.Is(err, model.ErrInvalidSKUSpecs) ||
			errors.Is(err, model.ErrInvalidSKUPrice) ||
			errors.Is(err, model.ErrInvalidSKUStock) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, model.ErrSKUCodeConflict) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}

		zap.S().Errorf("创建 SKU 失败: %v", err)
		respondError(
			c,
			http.StatusInternalServerError,
			"服务器内部错误",
		)
		return
	}

	respondSuccess(c, http.StatusCreated, newSKUResponse(&sku))
}

func (p *ProductController) GetSKU(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "sku_id 必须是大于 0 的整数")
		return
	}
	skuID, err := strconv.ParseUint(c.Param("sku_id"), 10, 64)
	if err != nil || skuID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	sku, err := p.service.GetSKU(c.Request.Context(), productID, skuID)
	if err != nil {
		if errors.Is(err, model.ErrSKUNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}
		zap.S().Errorf("查询 SKU 失败: product_id=%d sku_id=%d err=%v", productID, skuID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, newSKUResponse(&sku))
}

func (p *ProductController) ListSKU(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	var query ListSKUQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		respondError(c, http.StatusBadRequest, "查询参数格式错误")
		return
	}
	var input *services.ListSKUInput
	if query.Page != nil || query.PageSize != nil || query.Name != nil {
		input = &services.ListSKUInput{}
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

	total, skus, err := p.service.ListSKU(c.Request.Context(), productID, input)

	if err != nil {
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	response := make([]SKUResponse, len(skus))
	for i := range skus {
		response[i] = newSKUResponse(&skus[i])
	}
	respondSuccess(c, http.StatusOK, gin.H{"total": total, "list": response})

}
func (p *ProductController) UpdateSKU(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	skuID, err := strconv.ParseUint(c.Param("sku_id"), 10, 64)
	if err != nil || skuID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}

	var request UpdateSKURequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "请求 JSON 格式不正确")
	}
	sku, err := p.service.UpdateSKU(c.Request.Context(), productID, skuID, &services.UpdateSKUInput{
		Code:      request.Code,
		Specs:     request.Specs,
		PriceCent: request.PriceCent,
		Stock:     request.Stock,
	})
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"服务器内部错误",
		)
		return
	}
	respondSuccess(c, http.StatusCreated, newSKUResponse(&sku))

}
func (p *ProductController) DeleteSKU(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	skuID, err := strconv.ParseUint(c.Param("sku_id"), 10, 64)
	if err != nil || skuID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	if err := p.service.DeleteSKU(c.Request.Context(), productID, skuID); err != nil {
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}

	c.Status(http.StatusNoContent)

}

func (p *ProductController) UpdateSKUStatus(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	skuID, err := strconv.ParseUint(c.Param("sku_id"), 10, 64)
	if err != nil || skuID == 0 {
		respondError(c, http.StatusBadRequest, "sku_id 必须是大于 0 的整数")
		return
	}
	var request UpdateSKUStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	sku, err := p.service.UpdateSKUStatus(c.Request.Context(), productID, skuID, request.Status)
	if err != nil {
		if errors.Is(err, model.ErrInvalidSKUStatus) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, model.ErrSKUNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}
		zap.S().Errorf("更新 SKU 状态失败: product_id=%d sku_id=%d err=%v", productID, skuID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, newSKUResponse(&sku))
}

// ListPublicProducts 处理 GET /api/v1/products —— 消费者"逛商城"入口。
// 与运营列表的唯一差异：强制 status=on_sale（PRD-001 规则"只有上架商品可被消费者查询"），
// 不接受客户端传 status 参数——过滤条件绝不能交给调用方，这是服务端说了算的边界。
func (p *ProductController) ListPublicProducts(c *gin.Context) {
	var query ListProductsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		respondError(c, http.StatusBadRequest, "查询参数格式不正确")
		return
	}
	input := &services.ListProductsInput{Status: model.ProductStatusOnSale}
	if query.Page != nil {
		input.Page = *query.Page
	}
	if query.PageSize != nil {
		input.PageSize = *query.PageSize
	}
	if query.Name != nil {
		input.Name = *query.Name
	}
	total, products, err := p.service.ListProducts(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductQuery) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		zap.S().Errorf("查询在售商品失败: %v", err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	response := make([]ProductResponse, len(products))
	for i := range products {
		response[i] = newProductResponse(&products[i])
	}
	respondSuccess(c, http.StatusOK, gin.H{"total": total, "list": response})
}

// GetPublicSKUPage 处理 GET /api/v1/products/:product_id/skus —— 商城页按商品展开规格。
// 简化策略（学习项目）：返回该商品全部 SKU，仅 active 且所属商品在售；
// 商品下架时整卡不可见由列表接口的 on_sale 过滤天然保证（进不来这个 id 的合法页面）。
func (p *ProductController) GetPublicSKUs(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	product, err := p.service.GetProduct(c.Request.Context(), productID)
	if err != nil {
		if errors.Is(err, model.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, "资源不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if product.Status != model.ProductStatusOnSale {
		respondError(c, http.StatusNotFound, "资源不存在") // 未上架商品对消费者按不存在处理
		return
	}
	// 状态过滤下沉到查询层（SQL WHERE status=active），controller 不再内存二次过滤。
	total, skus, err := p.service.ListSKU(c.Request.Context(), productID, &services.ListSKUInput{Page: 1, PageSize: 100, Status: model.SKUStatusActive})
	if err != nil {
		zap.S().Errorf("查询商品规格失败: %v", err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	response := make([]SKUResponse, len(skus))
	for i := range skus {
		response[i] = newSKUResponse(&skus[i])
	}
	respondSuccess(c, http.StatusOK, gin.H{"total": total, "list": response})
}

// UpdateProductStatus 处理 PATCH /api/v1/admin/products/:product_id/status。
// 请求体只带 status；合法性（状态机）由 service 判断，controller 不重复规则。
func (p *ProductController) UpdateProductStatus(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil || productID == 0 {
		respondError(c, http.StatusBadRequest, "product_id 必须是大于 0 的整数")
		return
	}
	var request UpdateProductStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	product, err := p.service.UpdateStatus(c.Request.Context(), productID, request.Status)
	if err != nil {
		if errors.Is(err, model.ErrInvalidProductTransition) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, model.ErrProductNotFound) {
			respondError(c, http.StatusNotFound, err.Error())
			return
		}
		zap.S().Errorf("更新商品状态失败: product_id=%d err=%v", productID, err)
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, newProductResponse(&product))
}
