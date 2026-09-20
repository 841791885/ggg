package repositories

import (
	"context"
	"time"

	model "ggg/models"
)

// ConsumeCallbackInput 是一次支付回调事务消费所需的全部参数（PRD-007 进阶 A2）。
// 验签与报文校验在 service 层完成后才进入这里——repository 只信任"已通过安全校验"的事实。
type ConsumeCallbackInput struct {
	PaymentNo  string         // 支付单号，定位支付单
	EventNo    string         // 渠道事件号，幂等唯一键
	Result     string         // success / failed
	AmountCent int64          // 已交叉核对过的渠道金额（仅存档用）
	PaidAt     time.Time      // 完成时间
	Payload    map[string]any // 回调原始报文，落 payment_callback_logs 供对账
}

// TaskContext 随任务一起传给 handler：任务本体 + 仓库句柄（handler 需要跨模块读写订单/通知）。
type TaskContext struct {
	Task       model.BackgroundTask
	Repository TaskHandlerStore // 刻意小于全量 Repository：handler 能用哪些域由这里声明
}

// ListOrdersQuery 是订单列表查询条件；Admin=true 时忽略 UserID 查全量。
type ListOrdersQuery struct {
	UserID   uint64
	Status   model.OrderStatus
	Page     int
	PageSize int
}

// ListRefundsQuery 是退款列表查询条件。
type ListRefundsQuery struct {
	Status   model.RefundStatus
	Page     int
	PageSize int
}

// ListCouponTemplatesQuery 是优惠券模板列表查询条件。
type ListCouponTemplatesQuery struct {
	Status   model.CouponTemplateStatus
	Page     int
	PageSize int
}

// UpdateCouponTemplateFields 表示本次需要更新的优惠券模板字段，nil 表示不更新。
type UpdateCouponTemplateFields struct {
	Name          *string
	ThresholdCent *int64
	DiscountCent  *int64
	TotalCount    *int
	Remaining     *int
	StartsAt      *time.Time
	EndsAt        *time.Time
	Status        *model.CouponTemplateStatus
}

// ListTasksQuery 是后台任务列表查询条件。
type ListTasksQuery struct {
	Status   model.TaskStatus
	TaskType string
	Page     int
	PageSize int
}

// ProductRepository 定义商品模块需要的数据持久化能力。
type ProductRepository interface {
	CreateProduct(context.Context, model.Product) (model.Product, error)
	ListProducts(context.Context, ListProductsQuery) (int64, []model.Product, error)
	GetProduct(context.Context, uint64) (*model.Product, error)
	GetProductByIDs(context.Context, []uint64) ([]model.Product, error)
	UpdateProduct(context.Context, uint64, UpdateProductFields) (model.Product, error)
	UpdateProductStatus(context.Context, uint64, model.ProductStatus) (model.Product, error) // 上架/下架/转草稿
	DeleteProduct(context.Context, uint64) error
}

// SKURepository 定义 SKU 模块需要的数据持久化能力。
type SKURepository interface {
	CreateSKU(context.Context, model.SKU) (model.SKU, error)
	GetSKU(context.Context, uint64, uint64) (model.SKU, error)
	GetSKUByID(context.Context, uint64) (model.SKU, error)
	GetSKUByIDs(context.Context, []uint64) ([]model.SKU, error)
	ListSKU(context.Context, uint64, ListSKUQuery) (int64, []model.SKU, error)
	UpdateSKU(context.Context, uint64, uint64, UpdateSKUFields) (model.SKU, error)
	DeleteSKU(context.Context, uint64, uint64) error
	UpdateSKUStatus(context.Context, uint64, uint64, model.SKUStatus) (model.SKU, error)
}

// CartRepository 定义购物车模块需要的数据持久化能力。
type CartRepository interface {
	GetOrCreateCart(context.Context, uint64) (model.Cart, error)
	GetCart(context.Context, uint64) (model.Cart, error)
	GetCartItem(context.Context, uint64, uint64) (model.CartItem, error)
	GetCartItemByID(context.Context, uint64, uint64) (model.CartItem, error)
	CreateCartItem(context.Context, model.CartItem) (model.CartItem, error)
	UpdateCartItemQuantity(context.Context, uint64, uint64, int64) (model.CartItem, error)
	DeleteCartItem(context.Context, uint64, uint64) error
	// SetCartSelections 整组替换当前用户购物车的选中状态，返回更新后的明细。
	SetCartSelections(context.Context, uint64, []uint64) ([]model.CartItem, error)
}

// AddressRepository 定义收货地址模块需要的数据持久化能力。
// 所有方法都要求传入 userID 做所有权过滤，访问他人资源统一返回 ErrAddressNotFound。
type AddressRepository interface {
	CreateAddress(context.Context, model.Address) (model.Address, error)
	ListAddresses(context.Context, uint64) ([]model.Address, error)
	GetAddress(context.Context, uint64, uint64) (model.Address, error)
	CountAddresses(context.Context, uint64) (int64, error)
	UpdateAddress(context.Context, uint64, uint64, UpdateAddressFields) (model.Address, error)
	DeleteAddress(context.Context, uint64, uint64) error
	SetDefaultAddress(context.Context, uint64, uint64) (model.Address, error)
}

// UserRepository 定义用户模块需要的数据持久化能力。
type UserRepository interface {
	GetUserByLogin(context.Context, string) (model.User, error)
	GetUserByUsername(context.Context, string) (model.User, error)
	GetUserByEmail(context.Context, string) (model.User, error)
	CreateUser(context.Context, model.User) (model.User, error)
}

// OrderRepository 定义订单模块需要的数据持久化能力。
// 所有按 ID 访问的方法都带 userID 做所有权过滤；管理端方法以 admin 前缀区分。
type OrderRepository interface {
	CreateOrder(context.Context, model.Order) (model.Order, error)
	GetOrderByID(context.Context, uint64, uint64) (model.Order, error)             // userID, orderID（含订单项）
	GetOrderByIdempotencyKey(context.Context, uint64, string) (model.Order, error) // 幂等重放查询
	ListOrdersByUser(context.Context, uint64, ListOrdersQuery) (int64, []model.Order, error)
	UpdateOrderStatus(context.Context, uint64, uint64, model.OrderStatus, map[string]any) (model.Order, error) // userID, orderID, to；条件含当前状态
	CreateOrderStatusLog(context.Context, model.OrderStatusLog) error
	CancelOrder(context.Context, uint64, uint64) (model.Order, error) // userID, orderID
	CancelOrderBySystem(context.Context, uint64) (model.Order, error) // worker 超时关单：无用户身份、操作者记 system

	ListOrderStatusLogs(context.Context, uint64) ([]model.OrderStatusLog, error)

	AdminGetOrderByID(context.Context, uint64) (model.Order, error)
	AdminListOrders(context.Context, ListOrdersQuery) (int64, []model.Order, error)
	AdminUpdateOrderStatus(context.Context, uint64, model.OrderStatus, model.OrderStatus, map[string]any) (model.Order, error) // from, to
}

// PaymentRepository 定义支付单模块需要的数据持久化能力。
type PaymentRepository interface {
	CreatePayment(context.Context, model.Payment) (model.Payment, error)
	GetPaymentByNo(context.Context, uint64, string) (model.Payment, error) // userID, paymentNo
	GetActivePaymentByOrder(context.Context, uint64, uint64) (model.Payment, error)
	UpdatePaymentResult(context.Context, string, model.PaymentStatus, *string, *time.Time) (model.Payment, error) // paymentNo, from→success, eventNo, paidAt
	CreateCallbackLog(context.Context, model.PaymentCallbackLog) error
	ConsumePaymentCallback(context.Context, ConsumeCallbackInput) (model.Payment, bool, error) // 事务内幂等消费回调；bool=是否首次生效
	GetPaymentByNoAnyUser(context.Context, string) (model.Payment, error)                      // 回调侧按支付单号定位（无用户上下文）
}

// RefundRepository 定义退款模块需要的数据持久化能力。
type RefundRepository interface {
	CreateRefund(context.Context, model.Refund) (model.Refund, error)
	GetRefundByID(context.Context, uint64, uint64) (model.Refund, error) // userID, refundID
	ListRefundsByUser(context.Context, uint64) ([]model.Refund, error)
	AdminListRefunds(context.Context, ListRefundsQuery) (int64, []model.Refund, error)
	AdminReviewRefund(context.Context, uint64, model.RefundStatus, uint64) (model.Refund, error) // pending→approved/rejected
	UpdateOrderItemRefundStatus(context.Context, uint64, model.ItemRefundStatus) error
	GetOrderItemForUser(context.Context, uint64, uint64) (model.OrderItem, model.Order, error) // userID, itemID → 项 + 订单
}

// CouponRepository 定义优惠券模块需要的数据持久化能力。
type CouponRepository interface {
	CreateCouponTemplate(context.Context, model.CouponTemplate) (model.CouponTemplate, error)
	GetCouponTemplate(context.Context, uint64) (model.CouponTemplate, error)
	ListCouponTemplates(context.Context, ListCouponTemplatesQuery) (int64, []model.CouponTemplate, error)
	UpdateCouponTemplate(context.Context, uint64, UpdateCouponTemplateFields) (model.CouponTemplate, error)
	DeleteCouponTemplate(context.Context, uint64) error
	CreateUserCoupon(context.Context, model.UserCoupon) (model.UserCoupon, error)
	GetUserCouponByTemplate(context.Context, uint64, uint64) (model.UserCoupon, error)
	ListUserCoupons(context.Context, uint64) ([]model.UserCoupon, error)
	// ClaimCouponTx 事务内完成"扣发行量 + 插持有记录"（PRD-009 A4 防超发核心入口）。
	ClaimCouponTx(context.Context, uint64, uint64, int) (model.UserCoupon, error) // userID, templateID, perUserLimit
	CountUserCouponsByTemplate(context.Context, uint64, uint64) (int64, error)    // userID, templateID → 已领张数
}

// ReviewRepository 定义评价模块需要的数据持久化能力。
type ReviewRepository interface {
	CreateReview(context.Context, model.Review) (model.Review, error)
	GetReviewByOrderItem(context.Context, uint64) (model.Review, error)
	ListVisibleReviewsByProduct(context.Context, uint64, int, int) (int64, []model.Review, error)
	AdminListAllReviews(context.Context, uint64, int, int) (int64, []model.Review, error)
	SetReviewVisibility(context.Context, uint64, bool, uint64) (model.Review, error)
}

// NotificationRepository 定义站内通知模块需要的数据持久化能力。
type NotificationRepository interface {
	CreateNotification(context.Context, model.Notification) (model.Notification, error)
	ListNotifications(context.Context, uint64, int, int) (int64, []model.Notification, error)
	MarkNotificationRead(context.Context, uint64, uint64) error
	MarkAllNotificationsRead(context.Context, uint64) error
	HasNotificationLike(context.Context, uint64, string, string) (bool, error) // worker 通知幂等判重：user+type+title
}

// ShipmentRepository 定义发货记录的数据持久化能力（PRD-009 A7）。
type ShipmentRepository interface {
	CreateShipment(context.Context, model.Shipment) (model.Shipment, error)           // 撞唯一键翻译为 ErrShipmentDuplicate（幂等信号）
	GetShipmentByOrderAndKey(context.Context, uint64, string) (model.Shipment, error) // orderID, shippingKey → 幂等查询
	ListShipmentsByOrder(context.Context, uint64) ([]model.Shipment, error)           // 买家看物流 / 运营核对
	ShipOrderTx(context.Context, uint64, uint64, model.Shipment) (model.Order, error) // orderID, operatorID → 事务内改状态+插记录+写日志
}

// TaskRepository 定义后台任务模块需要的数据持久化能力。
type TaskRepository interface {
	CreateTask(context.Context, model.BackgroundTask) (model.BackgroundTask, error)
	ListTasks(context.Context, ListTasksQuery) (int64, []model.BackgroundTask, error)
	GetTaskByID(context.Context, uint64) (model.BackgroundTask, error)
	RetryTask(context.Context, uint64, uint64) (model.BackgroundTask, error)     // taskID, operatorID；仅 failed
	ClaimDueTask(context.Context, time.Time) (model.BackgroundTask, bool, error) // worker 领取到期任务（条件更新即锁）
	MarkTaskSucceeded(context.Context, uint64) error
	MarkTaskFailedOrRetry(context.Context, uint64, string, time.Duration) error // 未达上限退避回 pending，达到定格 failed
	RevertRunningTasks(context.Context) (int64, error)                          // 启动时回收崩溃遗留的 running 任务
}

// Repository 是当前 MySQL 仓库的完整能力集合，便于旧 Service 统一注入。
type Repository interface {
	ProductRepository
	SKURepository
	CartRepository
	AddressRepository
	OrderRepository
	PaymentRepository
	RefundRepository
	CouponRepository
	ReviewRepository
	NotificationRepository
	TaskRepository
	UserRepository
	ShipmentRepository
}

/* ═══════════════ Service 专用窄接口 ═══════════════
 * 每个 service 只声明它真正需要的那几个领域接口。收益：
 *   · 编译期挡住越权调用（cartService 拿不到 CancelOrder）
 *   · 读代码时一眼看出"这个 service 依赖哪些数据域"
 * Go 接口是隐式实现的，注入时仍传同一个 MySQLRepository，零行为变化。
 */

// ProductStore 是商品服务的数据依赖：商品与 SKU 两个域。
type ProductStore interface {
	ProductRepository
	SKURepository
}

// CartStore 是购物车服务的数据依赖：读写购物车外还需查 SKU/商品做库存与上下架校验。
type CartStore interface {
	CartRepository
	SKURepository
	ProductRepository
}

// OrderStore 是订单服务的数据依赖：下单要跨地址、购物车、商品/SKU（预览与扣库存），
// 发货要写 shipments（A7 幂等发货）。
type OrderStore interface {
	OrderRepository
	AddressRepository
	CartRepository
	SKURepository
	ProductRepository
	ShipmentRepository
}

// PaymentStore 是支付服务的数据依赖：回调推进订单状态，需读订单。
type PaymentStore interface {
	PaymentRepository
	OrderRepository
}

// ReviewStore 是评价服务的数据依赖：评价必须绑定真实订单项，
// 故还需退款域的"按用户取订单项+订单"能力（该查询本就归属订单项校验职责）。
type ReviewStore interface {
	ReviewRepository
	SKURepository
	RefundRepository
}

// WorkerStore 是 worker 主循环的数据依赖：只做"领任务 + 回写状态"，不越界。
type WorkerStore interface {
	TaskRepository
}

// TaskHandlerStore 是【任务处理器】的数据依赖：handler 天生要跨模块干活
// （超时关单要改订单、发通知要写站内信），所以它比 WorkerStore 宽。
// 单独命名而非复用胖 Repository，是为了让"handler 能碰哪些域"这件事显式可见——
// 新增 handler 时若用到清单外的能力，编译器会提醒你先来此处登记。
type TaskHandlerStore interface {
	OrderRepository        // CancelOrderBySystem：超时关单并释放库存
	NotificationRepository // CreateNotification / HasNotificationLike：站内信投递与判重
}
