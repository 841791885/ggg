package controllers

import (
	"net/http"

	model "ggg/models"
	"ggg/repositories"
	"ggg/services"

	"github.com/gin-gonic/gin"
)

// CreateCouponTemplate 运营创建满减券模板。
func (c *TradeController) CreateCouponTemplate(ctx *gin.Context) {
	var request CreateCouponTemplateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	template, err := c.couponService.CreateTemplate(ctx.Request.Context(), services.CreateTemplateInput{
		Name: request.Name, ThresholdCent: request.ThresholdCent, DiscountCent: request.DiscountCent,
		TotalCount: request.TotalCount, StartsAt: request.StartsAt.UTC(), EndsAt: request.EndsAt.UTC(),
	})
	if err != nil {
		respondTradeError(ctx, err, "创建优惠券模板")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newCouponTemplateResponse(&template))
}

// ListCouponTemplates 管理端分页查询优惠券模板，支持 status 过滤。
func (c *TradeController) ListCouponTemplates(ctx *gin.Context) {
	page, size := parsePageQuery(ctx)
	total, templates, err := c.couponService.ListTemplates(ctx.Request.Context(), repositories.ListCouponTemplatesQuery{
		Status: model.CouponTemplateStatus(ctx.Query("status")), Page: page, PageSize: size,
	})
	if err != nil {
		respondTradeError(ctx, err, "查询优惠券模板列表")
		return
	}
	items := make([]CouponTemplateResponse, len(templates))
	for i := range templates {
		items[i] = newCouponTemplateResponse(&templates[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// UpdateCouponTemplate 运营局部更新优惠券模板（含上下架）。
func (c *TradeController) UpdateCouponTemplate(ctx *gin.Context) {
	templateID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var request UpdateCouponTemplateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	input := services.UpdateTemplateInput{
		TemplateID: templateID, Name: request.Name, ThresholdCent: request.ThresholdCent,
		DiscountCent: request.DiscountCent, TotalCount: request.TotalCount, Remaining: request.Remaining,
		StartsAt: request.StartsAt, EndsAt: request.EndsAt,
	}
	if request.Status != nil {
		status := model.CouponTemplateStatus(*request.Status)
		input.Status = &status
	}
	template, err := c.couponService.UpdateTemplate(ctx.Request.Context(), input)
	if err != nil {
		respondTradeError(ctx, err, "更新优惠券模板")
		return
	}
	respondSuccess(ctx, http.StatusOK, newCouponTemplateResponse(&template))
}

// DeleteCouponTemplate 运营软删除优惠券模板。
func (c *TradeController) DeleteCouponTemplate(ctx *gin.Context) {
	templateID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	if err := c.couponService.DeleteTemplate(ctx.Request.Context(), templateID); err != nil {
		respondTradeError(ctx, err, "删除优惠券模板")
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ClaimCoupon 用户领取优惠券，重复领取返回 409。
func (c *TradeController) ClaimCoupon(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	templateID, ok := parseIDParam(ctx, "template_id")
	if !ok {
		return
	}
	coupon, err := c.couponService.Claim(ctx.Request.Context(), userID, templateID)
	if err != nil {
		respondTradeError(ctx, err, "领取优惠券")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newUserCouponResponse(&coupon))
}

// ListMyCoupons 查询当前用户持有的全部券。
func (c *TradeController) ListMyCoupons(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	coupons, err := c.couponService.ListMine(ctx.Request.Context(), userID)
	if err != nil {
		respondTradeError(ctx, err, "查询用户优惠券")
		return
	}
	items := make([]UserCouponResponse, len(coupons))
	for i := range coupons {
		items[i] = newUserCouponResponse(&coupons[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"items": items})
}
