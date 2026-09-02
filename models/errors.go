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
)
