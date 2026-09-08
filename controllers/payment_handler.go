package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreatePayment 为待支付订单创建模拟渠道支付单。
func (c *TradeController) CreatePayment(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	payment, err := c.paymentService.CreateForOrder(ctx.Request.Context(), userID, orderID)
	if err != nil {
		respondTradeError(ctx, err, "创建支付单")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newPaymentResponse(&payment))
}

// GetMyPayment 按支付单号查询当前用户的支付单。
func (c *TradeController) GetMyPayment(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	payment, err := c.paymentService.GetByPaymentNo(ctx.Request.Context(), userID, ctx.Param("payment_no"))
	if err != nil {
		respondTradeError(ctx, err, "查询支付单")
		return
	}
	respondSuccess(ctx, http.StatusOK, newPaymentResponse(&payment))
}
