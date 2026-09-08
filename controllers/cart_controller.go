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

type cartService interface {
	AddItem(ctx context.Context, input services.AddCartItemInput) (model.CartItem, error)
	GetCart(ctx context.Context, userID uint64) (model.Cart, error)
	UpdateItemQuantity(ctx context.Context, input services.UpdateCartItemInput) (model.CartItem, error)
	RemoveItem(ctx context.Context, userID, itemID uint64) error
	SetSelection(ctx context.Context, userID uint64, itemIDs []uint64) ([]model.CartItem, error)
}

// SetSelection 处理整组替换购物车选中状态的请求（PUT /cart/selection）。
func (c *CartController) SetSelection(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return
	}
	var request UpdateCartSelectionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	items, err := c.service.SetSelection(ctx.Request.Context(), userID, request.ItemIDs)
	if err != nil {
		zap.L().Error("更新购物车选中状态失败", zap.Uint64("user_id", userID), zap.Error(err))
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	responses := make([]CartItemResponse, len(items))
	for i := range items {
		responses[i] = newCartItemResponse(&items[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"items": responses})
}

// UpdateItemQuantity 处理修改当前用户购物车商品数量的请求。
func (c *CartController) UpdateItemQuantity(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return
	}
	itemID, err := strconv.ParseUint(ctx.Param("item_id"), 10, 64)
	if err != nil || itemID == 0 {
		respondError(ctx, http.StatusBadRequest, "item_id 必须是大于 0 的整数")
		return
	}
	var request UpdateCartItemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	item, err := c.service.UpdateItemQuantity(ctx.Request.Context(), services.UpdateCartItemInput{UserID: userID, ItemID: itemID, Quantity: request.Quantity})
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidCartQuantity):
			respondError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, model.ErrCartItemNotFound), errors.Is(err, model.ErrSKUNotFound):
			respondError(ctx, http.StatusNotFound, err.Error())
		case errors.Is(err, model.ErrSKUInactive), errors.Is(err, model.ErrInsufficientStock):
			respondError(ctx, http.StatusConflict, err.Error())
		default:
			zap.L().Error("修改购物车商品数量失败", zap.Uint64("user_id", userID), zap.Uint64("item_id", itemID), zap.Error(err))
			respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	respondSuccess(ctx, http.StatusOK, newCartItemResponse(&item))
}

// GetCart 处理查询用户购物车请求。
func (c *CartController) GetCart(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return
	}
	cart, err := c.service.GetCart(ctx.Request.Context(), userID)
	if err != nil {
		zap.S().Errorf("查询购物车失败: user_id=%d err=%v", userID, err)
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	items := make([]CartItemResponse, len(cart.Items))
	for i := range cart.Items {
		items[i] = newCartItemResponse(&cart.Items[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"id": cart.ID, "user_id": cart.UserID, "items": items})
}

// CartController 负责购物车相关 HTTP 请求。
type CartController struct{ service cartService }

// NewCartController 创建购物车控制器并注入业务服务。
func NewCartController(service cartService) *CartController { return &CartController{service: service} }

// AddItem 处理将 SKU 加入购物车的请求。
func (c *CartController) AddItem(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return
	}
	var request AddCartItemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	item, err := c.service.AddItem(ctx.Request.Context(), services.AddCartItemInput{
		UserID: userID, SKUID: request.SKUID, Quantity: request.Quantity,
	})
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidUserID), errors.Is(err, model.ErrInvalidSKUID), errors.Is(err, model.ErrInvalidCartQuantity):
			respondError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, model.ErrSKUNotFound):
			respondError(ctx, http.StatusNotFound, err.Error())
		case errors.Is(err, model.ErrSKUInactive), errors.Is(err, model.ErrInsufficientStock):
			respondError(ctx, http.StatusConflict, err.Error())
		default:
			zap.S().Errorf("加入购物车失败: %v", err)
			respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	respondSuccess(ctx, http.StatusCreated, newCartItemResponse(&item))
}

// RemoveItem 处理删除当前用户购物车明细的请求。
func (c *CartController) RemoveItem(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("user_id")
	userID, ok := userIDValue.(uint64)
	if !exists || !ok || userID == 0 {
		respondError(ctx, http.StatusUnauthorized, "用户身份不存在")
		return
	}

	itemID, err := strconv.ParseUint(ctx.Param("item_id"), 10, 64)
	if err != nil || itemID == 0 {
		respondError(ctx, http.StatusBadRequest, "item_id 必须是大于 0 的整数")
		return
	}

	if err := c.service.RemoveItem(ctx.Request.Context(), userID, itemID); err != nil {
		if errors.Is(err, model.ErrCartItemNotFound) {
			respondError(ctx, http.StatusNotFound, err.Error())
			return
		}
		zap.L().Error("删除购物车商品失败", zap.Uint64("user_id", userID), zap.Uint64("item_id", itemID), zap.Error(err))
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	ctx.Status(http.StatusNoContent)
}
