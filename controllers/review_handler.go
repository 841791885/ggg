package controllers

import (
	"net/http"
	"strconv"

	"ggg/services"

	"github.com/gin-gonic/gin"
)

// CreateReview 消费者对已完成订单项提交评价。
func (c *TradeController) CreateReview(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	itemID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var request struct {
		Rating  int    `json:"rating"`
		Content string `json:"content"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	review, err := c.reviewService.Create(ctx.Request.Context(), services.CreateReviewInput{
		UserID: userID, OrderItemID: itemID, Rating: request.Rating, Content: request.Content,
	})
	if err != nil {
		respondTradeError(ctx, err, "创建评价")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newReviewResponse(&review))
}

// ListProductReviews 消费者分页查看商品的可见评价（公开接口挂在商品详情下）。
func (c *TradeController) ListProductReviews(ctx *gin.Context) {
	productID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	page, size := parsePageQuery(ctx)
	total, reviews, err := c.reviewService.ListVisibleByProduct(ctx.Request.Context(), productID, page, size)
	if err != nil {
		respondTradeError(ctx, err, "查询商品评价")
		return
	}
	items := make([]ReviewResponse, len(reviews))
	for i := range reviews {
		items[i] = newReviewResponse(&reviews[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// AdminListReviews 管理端分页查看全部评价，product_id 可选过滤。
func (c *TradeController) AdminListReviews(ctx *gin.Context) {
	page, size := parsePageQuery(ctx)
	productID, _ := strconv.ParseUint(ctx.Query("product_id"), 10, 64)
	total, reviews, err := c.reviewService.AdminList(ctx.Request.Context(), productID, page, size)
	if err != nil {
		respondTradeError(ctx, err, "查询评价列表")
		return
	}
	items := make([]ReviewResponse, len(reviews))
	for i := range reviews {
		items[i] = newReviewResponse(&reviews[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// SetReviewVisibility 运营设置评价显隐；隐藏记录操作者用于审计。
func (c *TradeController) SetReviewVisibility(ctx *gin.Context) {
	operatorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	reviewID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var request SetReviewVisibilityRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	review, err := c.reviewService.SetVisibility(ctx.Request.Context(), operatorID, reviewID, request.Visible)
	if err != nil {
		respondTradeError(ctx, err, "更新评价显示状态")
		return
	}
	respondSuccess(ctx, http.StatusOK, newReviewResponse(&review))
}
