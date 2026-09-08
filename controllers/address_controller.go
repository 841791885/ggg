package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	model "ggg/models"
	"ggg/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type addressService interface {
	Create(ctx context.Context, input services.CreateAddressInput) (model.Address, error)
	List(ctx context.Context, userID uint64) ([]model.Address, error)
	Update(ctx context.Context, input services.UpdateAddressInput) (model.Address, error)
	SetDefault(ctx context.Context, userID, addressID uint64) (model.Address, error)
	Delete(ctx context.Context, userID, addressID uint64) error
}

// AddressController 负责收货地址相关 HTTP 请求。
type AddressController struct{ service addressService }

// NewAddressController 创建收货地址控制器并注入业务服务。
func NewAddressController(service addressService) *AddressController {
	return &AddressController{service: service}
}

// Create 处理新增收货地址请求。
func (c *AddressController) Create(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request CreateAddressRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	address, err := c.service.Create(ctx.Request.Context(), services.CreateAddressInput{
		UserID: userID, Recipient: request.Recipient, Phone: request.Phone,
		Province: request.Province, City: request.City, District: request.District, Detail: request.Detail,
	})
	if err != nil {
		respondAddressError(ctx, err, userID, 0)
		return
	}
	respondSuccess(ctx, http.StatusCreated, newAddressResponse(&address))
}

// List 处理查询当前用户地址列表请求。
func (c *AddressController) List(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	addresses, err := c.service.List(ctx.Request.Context(), userID)
	if err != nil {
		zap.L().Error("查询收货地址列表失败", zap.Uint64("user_id", userID), zap.Error(err))
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	items := make([]AddressResponse, len(addresses))
	for i := range addresses {
		items[i] = newAddressResponse(&addresses[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"items": items})
}

// Update 处理修改收货地址请求。
func (c *AddressController) Update(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	addressID, ok := parseAddressID(ctx)
	if !ok {
		return
	}
	var request UpdateAddressRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	address, err := c.service.Update(ctx.Request.Context(), services.UpdateAddressInput{
		UserID: userID, AddressID: addressID, Recipient: request.Recipient, Phone: request.Phone,
		Province: request.Province, City: request.City, District: request.District, Detail: request.Detail,
	})
	if err != nil {
		respondAddressError(ctx, err, userID, addressID)
		return
	}
	respondSuccess(ctx, http.StatusOK, newAddressResponse(&address))
}

// SetDefault 处理设置默认收货地址请求，切换在数据库事务内完成。
func (c *AddressController) SetDefault(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	addressID, ok := parseAddressID(ctx)
	if !ok {
		return
	}
	address, err := c.service.SetDefault(ctx.Request.Context(), userID, addressID)
	if err != nil {
		respondAddressError(ctx, err, userID, addressID)
		return
	}
	respondSuccess(ctx, http.StatusOK, newAddressResponse(&address))
}

// Delete 处理删除收货地址请求。
func (c *AddressController) Delete(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	addressID, ok := parseAddressID(ctx)
	if !ok {
		return
	}
	if err := c.service.Delete(ctx.Request.Context(), userID, addressID); err != nil {
		respondAddressError(ctx, err, userID, addressID)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// currentUserID 从 JWT 认证上下文读取当前用户 ID，失败时直接写 401 响应。
func currentUserID(ctx *gin.Context) (uint64, bool) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return 0, false
	}
	return userID, true
}

// parseAddressID 解析路径中的 :id 参数，失败时直接写 400 响应。
func parseAddressID(ctx *gin.Context) (uint64, bool) {
	addressID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || addressID == 0 {
		respondError(ctx, http.StatusBadRequest, "id 必须是大于 0 的整数")
		return 0, false
	}
	return addressID, true
}

// respondAddressError 把地址业务错误映射为 HTTP 响应；返回 false 表示未识别的错误，由调用方按 500 处理前先记录日志。
func respondAddressError(ctx *gin.Context, err error, userID, addressID uint64) bool {
	switch {
	case errors.Is(err, model.ErrInvalidRecipient), errors.Is(err, model.ErrInvalidPhone),
		errors.Is(err, model.ErrInvalidAddressRegion), errors.Is(err, model.ErrInvalidAddressDetail),
		errors.Is(err, model.ErrEmptyAddressUpdate), errors.Is(err, model.ErrAddressLimitExceeded):
		respondError(ctx, http.StatusBadRequest, err.Error())
	case errors.Is(err, model.ErrAddressNotFound):
		// 他人地址统一按不存在处理，不泄露资源是否存在。
		respondError(ctx, http.StatusNotFound, err.Error())
	default:
		zap.L().Error("收货地址操作失败", zap.Uint64("user_id", userID), zap.Uint64("address_id", addressID), zap.Error(err))
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
	}
	return true
}
