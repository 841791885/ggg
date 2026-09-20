package controllers

import (
	"net/http"

	model "ggg/models"
	"ggg/repositories"
	"ggg/services"

	"github.com/gin-gonic/gin"
)

// CartPreview 处理下单预览请求：服务端重算选中项的实时价格与可购买性。
func (c *TradeController) CartPreview(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	preview, err := c.orderService.Preview(ctx.Request.Context(), userID)
	if err != nil {
		respondTradeError(ctx, err, "下单预览")
		return
	}
	respondSuccess(ctx, http.StatusOK, preview)
}

// CreateOrder 处理创建订单请求。幂等键来自 Idempotency-Key 请求头，重试返回首次结果。
func (c *TradeController) CreateOrder(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request CreateOrderRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	order, err := c.orderService.Create(ctx.Request.Context(), services.CreateOrderInput{
		UserID: userID, AddressID: request.AddressID, IdempotencyKey: ctx.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		respondTradeError(ctx, err, "创建订单")
		return
	}
	respondSuccess(ctx, http.StatusCreated, newOrderResponse(&order))
}

// ListMyOrders 分页查询当前用户订单，支持 status 过滤。
func (c *TradeController) ListMyOrders(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	page, size := parsePageQuery(ctx)
	total, orders, err := c.orderService.ListByUser(ctx.Request.Context(), userID, repositories.ListOrdersQuery{
		Status: orderStatusParam(ctx), Page: page, PageSize: size,
	})
	if err != nil {
		respondTradeError(ctx, err, "查询订单列表")
		return
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": newOrderResponses(orders)})
}

// GetMyOrder 查询当前用户订单详情及状态历史。
func (c *TradeController) GetMyOrder(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	order, logs, err := c.orderService.Get(ctx.Request.Context(), userID, orderID)
	if err != nil {
		respondTradeError(ctx, err, "查询订单")
		return
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"order": newOrderResponse(&order), "status_logs": logs})
}

// CancelOrder 处理消费者取消待支付订单。
func (c *TradeController) CancelOrder(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	order, err := c.orderService.Cancel(ctx.Request.Context(), userID, orderID)
	if err != nil {
		respondTradeError(ctx, err, "取消订单")
		return
	}
	respondSuccess(ctx, http.StatusOK, newOrderResponse(&order))
}

// ConfirmReceipt 处理消费者确认收货。
func (c *TradeController) ConfirmReceipt(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	order, err := c.orderService.ConfirmReceipt(ctx.Request.Context(), userID, orderID)
	if err != nil {
		respondTradeError(ctx, err, "确认收货")
		return
	}
	respondSuccess(ctx, http.StatusOK, newOrderResponse(&order))
}

// AdminListOrders 管理端分页查询全部订单。
func (c *TradeController) AdminListOrders(ctx *gin.Context) {
	page, size := parsePageQuery(ctx)
	total, orders, err := c.orderService.AdminList(ctx.Request.Context(), repositories.ListOrdersQuery{
		Status: orderStatusParam(ctx), Page: page, PageSize: size,
	})
	if err != nil {
		respondTradeError(ctx, err, "查询订单列表")
		return
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"total": total, "list": newOrderResponses(orders)})
}

// AdminGetOrder 管理端查询任意订单详情。
func (c *TradeController) AdminGetOrder(ctx *gin.Context) {
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	order, logs, err := c.orderService.AdminGet(ctx.Request.Context(), orderID)
	if err != nil {
		respondTradeError(ctx, err, "查询订单")
		return
	}
	respondSuccess(ctx, http.StatusOK, gin.H{"order": newOrderResponse(&order), "status_logs": logs})
}

// AdminShipOrder 运营发货，仅允许已支付订单进入已发货。
func (c *TradeController) AdminShipOrder(ctx *gin.Context) {
	operatorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	orderID, ok := parseIDParam(ctx, "id")
	if !ok {
		return
	}
	var request ShipOrderRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	order, err := c.orderService.Ship(ctx.Request.Context(), services.ShipInput{
		OperatorID: operatorID, OrderID: orderID,
		ShippingKey: request.ShippingKey, Carrier: request.Carrier, TrackingNo: request.TrackingNo,
	})
	if err != nil {
		respondTradeError(ctx, err, "发货")
		return
	}
	respondSuccess(ctx, http.StatusOK, newOrderResponse(&order))
}

// orderStatusParam 读取可选的 status 查询参数并转换为订单状态枚举。
func orderStatusParam(ctx *gin.Context) model.OrderStatus {
	return model.OrderStatus(ctx.Query("status"))
}
