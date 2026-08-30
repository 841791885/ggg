package controllers

import (
	"context"
	"errors"
	model "ggg/models"
	"ggg/services"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type userService interface {
	Register(context.Context, services.RegisterUserInput) (model.User, error)
	Login(context.Context, services.LoginInput) (services.LoginResponse, error)
}

// Login 处理用户登录请求并返回 JWT。
func (c *UserController) Login(ctx *gin.Context) {
	var request LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	result, err := c.service.Login(ctx.Request.Context(), services.LoginInput{Login: request.Login, Password: request.Password})
	if err != nil {
		if errors.Is(err, model.ErrInvalidLogin) {
			respondError(ctx, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, model.ErrInvalidCredentials) || errors.Is(err, model.ErrUserDisabled) {
			respondError(ctx, http.StatusUnauthorized, err.Error())
			return
		}
		log.Printf("用户登录失败: %v", err)
		respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	respondSuccess(ctx, http.StatusOK, result)
}

// UserController 负责用户相关 HTTP 请求。
type UserController struct{ service userService }

// NewUserController 创建用户控制器并注入业务服务。
func NewUserController(service userService) *UserController { return &UserController{service: service} }

// Register 处理用户注册请求。
func (c *UserController) Register(ctx *gin.Context) {
	var request RegisterUserRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		respondError(ctx, http.StatusBadRequest, "请求 JSON 格式不正确")
		return
	}
	user, err := c.service.Register(ctx.Request.Context(), services.RegisterUserInput{Username: request.Username, Email: request.Email, Password: request.Password})
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidUsername), errors.Is(err, model.ErrInvalidEmail), errors.Is(err, model.ErrInvalidPassword):
			respondError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, model.ErrUsernameConflict), errors.Is(err, model.ErrEmailConflict):
			respondError(ctx, http.StatusConflict, err.Error())
		default:
			log.Printf("用户注册失败: %v", err)
			respondError(ctx, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	respondSuccess(ctx, http.StatusCreated, newUserResponse(&user))
}
