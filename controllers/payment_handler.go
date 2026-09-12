package controllers

import (
	"net/http"

	"ggg/services"

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

// MockPaymentCallback 接收模拟支付渠道的异步回调（PRD-007 进阶 A2）。
// 公开端点、无登录态——安全完全依赖 HMAC 验签；响应只回 {received:true}，
// 不泄露任何业务数据（真实渠道对接里这就是"应答报文"格式约定）。
func (c *TradeController) MockPaymentCallback(ctx *gin.Context) {
	var callback services.MockPaymentCallback
	if err := ctx.ShouldBindJSON(&callback); err != nil {
		respondError(ctx, http.StatusBadRequest, "回调 JSON 格式不正确")
		return
	}
	if _, err := c.paymentService.HandleCallback(ctx.Request.Context(), callback); err != nil {
		respondTradeError(ctx, err, "支付回调")
		return
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"received": true})
}
