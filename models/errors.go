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
	ErrEmptyProductUpdate = errors.New("至少提供一个需要修改的商品字段")
)
