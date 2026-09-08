package controllers

import (
	"net/http"

	"ggg/services"

	"github.com/gin-gonic/gin"
)

// ListNotifications 分页查询当前用户通知，未读在前。
func (c *TradeController) ListNotifications(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	page, size := parsePageQuery(ctx)
	total, notifications, err := c.notificationService.List(ctx.Request.Context(), userID, page, size)
	if err != nil {
		respondTradeError(ctx, err, "查询通知列表")
		return
	}
	items := make([]NotificationResponse, len(notifications))
	for i := range notifications {
		items[i] = newNotificationResponse(&notifications[i])
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": items})
}

// MarkNotificationRead 标记单条通知已读。
func (c *TradeController) MarkNotificationRead(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	notificationID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	if err := c.notificationService.MarkRead(ctx.Request.Context(), userID, notificationID); err != nil {
		respondTradeError(ctx, err, "标记通知已读")
		return
	}
	ctx.Status(http.StatusNoContent)
}

// MarkAllNotificationsRead 一键已读全部未读通知。
func (c *TradeController) MarkAllNotificationsRead(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	if err := c.notificationService.MarkAllRead(ctx.Request.Context(), userID); err != nil {
		respondTradeError(ctx, err, "批量标记通知已读")
		return
	}
	ctx.Status(http.StatusNoContent)
}

// CreateNotification 生成站内通知（本阶段供联调造数；事件自动触发待 PRD-008 worker）。
func (c *TradeController) CreateNotification(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request CreateNotificationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	notification, err := c.notificationService.Create(ctx.Request.Context(), services.CreateNotificationInput{
		UserID: userID, Type: request.Type, Title: request.Title, Content: request.Content,
	})
	if err != nil {
		respondTradeError(ctx, err, "创建通知")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newNotificationResponse(&notification))
}
