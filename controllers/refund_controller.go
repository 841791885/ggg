package controllers

import (
	"net/http"

	model "ggg/models"
	"ggg/repositories"
	"ggg/services"

	"github.com/gin-gonic/gin"
)

// ApplyRefund 消费者对订单项申请退款。
func (c *TradeController) ApplyRefund(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	itemID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var request ApplyRefundRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	refund, err := c.refundService.Apply(ctx.Request.Context(), services.ApplyRefundInput{
		UserID: userID, OrderItemID: itemID, AmountCent: request.AmountCent, Reason: request.Reason,
	})
	if err != nil {
		respondTradeError(ctx, err, "申请退款")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newRefundResponse(&refund))
}

// ListMyRefunds 查询当前用户的退款单列表。
func (c *TradeController) ListMyRefunds(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	refunds, err := c.refundService.ListByUser(ctx.Request.Context(), userID)
	if err != nil {
		respondTradeError(ctx, err, "查询退款列表")
		return
	}
	items := make([]RefundResponse, len(refunds))
	for i := range refunds {
		items[i] = newRefundResponse(&refunds[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"items": items})
}

// AdminListRefunds 管理端分页查询全部退款单，支持 status 过滤。
func (c *TradeController) AdminListRefunds(ctx *gin.Context) {
	page, size := parsePageQuery(ctx)
	total, refunds, err := c.refundService.AdminList(ctx.Request.Context(), repositories.ListRefundsQuery{
		Status: model.RefundStatus(ctx.Query("status")), Page: page, PageSize: size,
	})
	if err != nil {
		respondTradeError(ctx, err, "查询退款列表")
		return
	}
	items := make([]RefundResponse, len(refunds))
	for i := range refunds {
		items[i] = newRefundResponse(&refunds[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// AdminApproveRefund 运营审核通过并模拟退款成功。
func (c *TradeController) AdminApproveRefund(ctx *gin.Context) {
	c.reviewRefund(ctx, true)
}

// AdminRejectRefund 运营驳回退款申请。
func (c *TradeController) AdminRejectRefund(ctx *gin.Context) {
	c.reviewRefund(ctx, false)
}

// reviewRefund 是审核的公共入口；approve=true 通过，否则驳回。
// 目的：两条审核路径共用归属校验、错误映射和响应转换，保证行为一致。
func (c *TradeController) reviewRefund(ctx *gin.Context, approve bool) {
	operatorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	refundID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var refund model.Refund
	var err error
	if approve {
		refund, err = c.refundService.Approve(ctx.Request.Context(), operatorID, refundID)
	} else {
		refund, err = c.refundService.Reject(ctx.Request.Context(), operatorID, refundID)
	}
	if err != nil {
		respondTradeError(ctx, err, "审核退款")
		return
	}
	respondSuccess(ctx, http.StatusOK, newRefundResponse(&refund))
}
