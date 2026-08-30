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

type cartService interface {
	AddItem(ctx context.Context, input services.AddCartItemInput) (model.CartItem, error)
	GetCart(ctx context.Context, userID uint64) (model.Cart, error)
}

// GetCart 处理查询用户购物车请求。
func (c *CartController) GetCart(ctx *gin.Context) {
	userID, err := strconv.ParseUint(ctx.Query("user_id"), 10, 64)
	if err != nil || userID == 0 {
		respondError(ctx, http.StatusBadRequest, "user_id 必须是大于 0 的整数")
		return
	}
	cart, err := c.service.GetCart(ctx.Request.Context(), userID)
	if err != nil {
		log.Printf("查询购物车失败: user_id=%d err=%v", userID, err)
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
	var request AddCartItemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	item, err := c.service.AddItem(ctx.Request.Context(), services.AddCartItemInput{
		UserID: request.UserID, SKUID: request.SKUID, Quantity: request.Quantity,
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
			log.Printf("加入购物车失败: %v", err)
			respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	respondSuccess(ctx, http.StatusCreated, newCartItemResponse(&item))
}
