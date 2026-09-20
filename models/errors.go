package models

import "errors"

var (
	// ErrInvalidProductName 表示商品名称去除首尾空白后不符合长度规则。
	ErrInvalidProductName = errors.New("商品名称必须为 2 到 100 个字符")
	// ErrInvalidProductDescription 表示商品描述超过允许的最大长度。
	ErrInvalidProductDescription = errors.New("商品描述不能超过 2000 个字符")
	// ErrInvalidProductQuery 表示商品列表的分页条件不合法。
	ErrInvalidProductQuery = errors.New("page 必须大于等于 1，page_size 必须在 1 到 100 之间")
	// ErrProductNotFound 表示指定商品不存在。
	ErrProductNotFound = errors.New("商品不存在")
	// ErrInvalidProductTransition 表示商品状态迁移不合法（如草稿重复上架、在售回退草稿）。
	ErrInvalidProductTransition = errors.New("商品当前状态不允许该操作")
	// ErrProductOnSale 表示在售商品不能直接删除。
	ErrProductOnSale = errors.New("在售商品不能删除")
	// ErrEmptyProductUpdate 表示修改商品时没有提供任何可修改字段。
	ErrEmptyProductUpdate  = errors.New("至少提供一个需要修改的商品字段")
	ErrInvalidProductID    = errors.New("商品 ID 必须大于 0")
	ErrInvalidSKUCode      = errors.New("SKU 编码必须为 3 到 32 位，只能使用大写字母、数字、下划线和连字符")
	ErrInvalidSKUSpecs     = errors.New("SKU 规格必须为 1 到 5 项，且 key 和 value 不能为空")
	ErrInvalidSKUPrice     = errors.New("SKU 价格必须大于 0")
	ErrInvalidSKUStock     = errors.New("SKU 库存不能小于 0")
	ErrSKUCodeConflict     = errors.New("SKU 编码已存在")
	ErrSKUNotFound         = errors.New("SKU 不存在")
	ErrEmptySKUUpdate      = errors.New("至少提供一个需要修改的 SKU 字段")
	ErrInvalidSKUStatus    = errors.New("SKU 状态只能是 active 或 inactive")
	ErrCartItemNotFound    = errors.New("购物车商品不存在")
	ErrInvalidUserID       = errors.New("用户 ID 必须大于 0")
	ErrInvalidSKUID        = errors.New("SKU ID 必须大于 0")
	ErrInvalidCartQuantity = errors.New("购物车数量必须大于 0")
	ErrSKUInactive         = errors.New("SKU 未启用")
	ErrInsufficientStock   = errors.New("SKU 库存不足")
	ErrUserNotFound        = errors.New("用户不存在")
	ErrUsernameConflict    = errors.New("用户名已存在")
	ErrEmailConflict       = errors.New("邮箱已存在")
	ErrInvalidUsername     = errors.New("用户名必须为 3 到 50 个字符")
	ErrInvalidEmail        = errors.New("邮箱格式不正确")
	ErrInvalidPassword     = errors.New("密码必须为 8 到 72 个字符")
	ErrInvalidLogin        = errors.New("用户名或邮箱不能为空")
	ErrInvalidCredentials  = errors.New("用户名、邮箱或密码错误")
	ErrUserDisabled        = errors.New("用户已被禁用")
	// ErrAddressNotFound 表示地址不存在；他人地址统一按不存在处理，避免泄露资源是否存在。
	ErrAddressNotFound = errors.New("收货地址不存在")
	// ErrInvalidRecipient 表示收件人姓名去除首尾空白后不符合长度规则。
	ErrInvalidRecipient = errors.New("收件人必须为 2 到 30 个字符")
	// ErrInvalidPhone 表示手机号不符合大陆 11 位号码格式。
	ErrInvalidPhone = errors.New("手机号格式不正确")
	// ErrInvalidAddressRegion 表示省、市或区名称为空或超长。
	ErrInvalidAddressRegion = errors.New("省、市、区名称必须为 1 到 50 个字符")
	// ErrInvalidAddressDetail 表示详细地址不符合长度规则。
	ErrInvalidAddressDetail = errors.New("详细地址必须为 5 到 200 个字符")
	// ErrEmptyAddressUpdate 表示修改地址时没有提供任何可修改字段。
	ErrEmptyAddressUpdate = errors.New("至少提供一个需要修改的地址字段")
	// ErrAddressLimitExceeded 表示用户有效地址数量已达到上限。
	ErrAddressLimitExceeded = errors.New("收货地址数量不能超过 20 个")
	// ErrOrderNotFound 表示订单不存在；他人订单一律按不存在处理。
	ErrOrderNotFound = errors.New("订单不存在")
	// ErrInvalidIdempotencyKey 表示创建订单缺少或非法的幂等键。
	ErrInvalidIdempotencyKey = errors.New("必须携带 8 到 64 位的 Idempotency-Key 请求头")
	// ErrIdempotencyConflict 表示相同幂等键提交了不同的请求内容。
	ErrIdempotencyConflict = errors.New("相同幂等键的请求内容不一致")
	// ErrEmptyCartSelection 表示下单预览没有选中任何可购买的商品。
	ErrEmptyCartSelection = errors.New("请至少选择一件可购买的商品")
	// ErrInvalidOrderTransition 表示订单状态迁移不合法。
	ErrInvalidOrderTransition = errors.New("订单当前状态不允许该操作")
	// ErrPaymentNotFound 表示支付单不存在。
	ErrPaymentNotFound = errors.New("支付单不存在")
	// ErrPaymentAlreadyExists 表示订单已有进行中的支付单。
	ErrPaymentAlreadyExists = errors.New("订单已存在待支付支付单")
	// ErrInvalidCallbackSignature 表示回调 HMAC 签名校验失败（伪造或密钥不一致）。
	ErrInvalidCallbackSignature = errors.New("回调签名校验失败")
	// ErrCallbackAmountMismatch 表示回调金额与支付单不一致，拒绝推进。
	ErrCallbackAmountMismatch = errors.New("回调金额与支付单不一致")
	// ErrCallbackOrderStateConflict 表示支付结果与订单当前状态冲突（如已取消订单收到成功回调），需人工处理。
	ErrCallbackOrderStateConflict = errors.New("支付结果与订单状态冲突，请联系客服处理")
	// ErrRefundNotFound 表示退款单不存在。
	ErrRefundNotFound = errors.New("退款单不存在")
	// ErrRefundAlreadyPending 表示该订单项已有待处理退款。
	ErrRefundAlreadyPending = errors.New("该商品已有退款申请正在处理中")
	// ErrInvalidRefundAmount 表示退款金额超过订单项实付小计。
	ErrInvalidRefundAmount = errors.New("退款金额不能超过该商品实付金额")
	// ErrCouponNotFound 表示优惠券模板或用户的券不存在。
	ErrCouponNotFound = errors.New("优惠券不存在")
	// ErrCouponNotClaimable 表示优惠券当前不可领取（下架、未开始、已结束或库存不足）。
	ErrCouponNotClaimable = errors.New("优惠券当前不可领取")
	// ErrCouponAlreadyClaimed 表示用户已领取过该优惠券。
	ErrCouponAlreadyClaimed = errors.New("已领取过该优惠券")
	// ErrReviewNotFound 表示评价不存在。
	ErrReviewNotFound = errors.New("评价不存在")
	// ErrReviewNotEligible 表示订单项不满足评价条件（非本人、未完成或已评价）。
	ErrReviewNotEligible = errors.New("该商品暂不可评价")
	// ErrReviewDuplicate 表示订单项已经评价过。
	ErrReviewDuplicate = errors.New("每个商品只能评价一次")
	// ErrNotificationNotFound 表示通知不存在。
	ErrNotificationNotFound = errors.New("通知不存在")
	// ErrTaskNotFound 表示后台任务不存在。
	ErrTaskNotFound = errors.New("任务不存在")
	// ErrTaskNotRetryable 表示任务不处于失败状态，不能重试。
	ErrTaskNotRetryable = errors.New("只有失败的任务可以重试")
	// ErrShipmentDuplicate 表示同一批发货（同订单 + 同 shipping_key）已存在。
	// 它是幂等重试的【信号】而非错误：service 捕获后应返回首次发货结果，不是报错给运营。
	ErrShipmentDuplicate = errors.New("该批发货已记录")
	// ErrShipmentNotFound 表示发货记录不存在。
	ErrShipmentNotFound = errors.New("发货记录不存在")
	// ErrInvalidShippingKey 表示发货幂等键缺失或长度不合规。
	ErrInvalidShippingKey = errors.New("shipping_key 长度必须在 8 到 64 之间")
	// ErrInvalidOrderQuery 表示订单列表查询条件不合法。
	ErrInvalidOrderQuery = errors.New("page 必须大于等于 1，page_size 必须在 1 到 100 之间")
	// ErrInvalidCouponInput 表示优惠券模板参数不合法。
	ErrInvalidCouponInput = errors.New("优惠券参数不合法：名称 2-50 字，门槛和面额必须为正且结束时间晚于开始时间")
)
