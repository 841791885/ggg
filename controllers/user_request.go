package controllers

// RegisterUserRequest 表示用户注册请求体。
type RegisterUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest 表示用户登录请求体。
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
