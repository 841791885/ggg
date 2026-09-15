package controllers

import (
	"errors"
	"net/http"
	"strconv"

	model "ggg/models"
	"ggg/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TradeController 负责订单、支付、退款、优惠券、评价、通知和任务的 HTTP 请求。
// 各领域的请求处理方法按业务拆分在同包文件中（order_controller.go、payment_controller.go 等），
// 此处只持有依赖与公共工具。命名统一为 *_controller.go：controller 是"HTTP 层"在本项目的唯一叫法，
// 不再混用 handler（两者同义，混用只会让读代码的人以为存在两种分层）。
type TradeController struct {
	orderService        *services.OrderService
	paymentService      *services.PaymentService
	refundService       *services.RefundService
	couponService       *services.CouponService
	reviewService       *services.ReviewService
	notificationService *services.NotificationService
	taskService         *services.TaskService
}

// NewTradeController 创建交易控制器并注入全部业务服务。
func NewTradeController(
	orderService *services.OrderService,
	paymentService *services.PaymentService,
	refundService *services.RefundService,
	couponService *services.CouponService,
	reviewService *services.ReviewService,
	notificationService *services.NotificationService,
	taskService *services.TaskService,
) *TradeController {
	return &TradeController{
		orderService: orderService, paymentService: paymentService, refundService: refundService,
		couponService: couponService, reviewService: reviewService,
		notificationService: notificationService, taskService: taskService,
	}
}

// parseIDParam 解析路径中的数字 ID 参数，失败时写 400 并返回 false。
func parseIDParam(ctx *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(ctx.Param(name), 10, 64)
	if err != nil || value == 0 {
		respondError(ctx, http.StatusBadRequest, name+" 必须是大于 0 的整数")
		return 0, false
	}
	return value, true
}

// parsePageQuery 读取分页参数，缺省或非法时回退为第 1 页 20 条。
func parsePageQuery(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

// respondTradeError 把交易域业务错误统一映射为 HTTP 响应。
// 目的：集中维护"错误 → 状态码"契约，避免每个 controller 方法重复 switch 造成行为漂移。
func respondTradeError(ctx *gin.Context, err error, resource string) {
	switch {
	case errors.Is(err, model.ErrOrderNotFound), errors.Is(err, model.ErrPaymentNotFound),
		errors.Is(err, model.ErrRefundNotFound), errors.Is(err, model.ErrCouponNotFound),
		errors.Is(err, model.ErrReviewNotFound), errors.Is(err, model.ErrNotificationNotFound),
		errors.Is(err, model.ErrTaskNotFound), errors.Is(err, model.ErrAddressNotFound):
		// 他人资源统一按不存在处理（404），不泄露资源是否存在。
		respondError(ctx, http.StatusNotFound, "资源不存在")
	case errors.Is(err, model.ErrInvalidIdempotencyKey), errors.Is(err, model.ErrEmptyCartSelection),
		errors.Is(err, model.ErrInvalidRefundAmount), errors.Is(err, model.ErrInvalidCouponInput),
		errors.Is(err, model.ErrInvalidUserID), errors.Is(err, model.ErrInvalidCallbackSignature),
		errors.Is(err, model.ErrCallbackAmountMismatch):
		// 400：报文本身有问题（签名伪造、金额对不上）——调用方（渠道模拟器）应修正后重新发起。
		respondError(ctx, http.StatusBadRequest, err.Error())
	case errors.Is(err, model.ErrIdempotencyConflict), errors.Is(err, model.ErrInvalidOrderTransition),
		errors.Is(err, model.ErrPaymentAlreadyExists), errors.Is(err, model.ErrRefundAlreadyPending),
		errors.Is(err, model.ErrInsufficientStock),          // 下单事务内预占失败：合法请求撞上库存现状，可改数量后重试
		errors.Is(err, model.ErrCallbackOrderStateConflict), // 乱序回调（已取消订单收到成功）：挂起进人工通道
		errors.Is(err, model.ErrCouponNotClaimable), errors.Is(err, model.ErrCouponAlreadyClaimed),
		errors.Is(err, model.ErrReviewDuplicate), errors.Is(err, model.ErrReviewNotEligible),
		errors.Is(err, model.ErrTaskNotRetryable):
		// 409：请求本身合法但与资源当前状态冲突，客户端可依据提示修正后重试。
		respondError(ctx, http.StatusConflict, err.Error())
	default:
		zap.L().Error(resource+"操作失败", zap.Error(err))
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
	}
}
