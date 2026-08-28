package controllers

import "github.com/gin-gonic/gin"

// SuccessResponse 是所有成功 JSON 响应使用的全局外层结构。
type SuccessResponse[T any] struct {
	Code int `json:"code"`
	Data T   `json:"data"`
}

// FailureResponse 是所有失败 JSON 响应使用的全局外层结构。
type FailureResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// respondSuccess 使用统一格式返回成功数据。
func respondSuccess[T any](c *gin.Context, httpStatus int, data T) {
	c.JSON(httpStatus, SuccessResponse[T]{
		Code: httpStatus,
		Data: data,
	})
}

// respondError 使用统一格式返回业务或系统错误。
func respondError(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, FailureResponse{
		Code:    httpStatus,
		Message: message,
	})
}
