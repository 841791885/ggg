package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "ggg/models"
	"ggg/repositories"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// LoginInput 表示用户登录参数。
type LoginInput struct{ Login, Password string }

// LoginResponse 表示登录成功后返回的 JWT 信息。
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// AuthService 负责登录和身份凭证生成。
type AuthService struct {
	repository repositories.UserRepository
	jwtSecret  []byte
	tokenTTL   time.Duration
}

// NewAuthService 创建认证服务。
func NewAuthService(repository repositories.UserRepository, secret string, tokenTTL time.Duration) *AuthService {
	return &AuthService{repository: repository, jwtSecret: []byte(secret), tokenTTL: tokenTTL}
}

// Login 验证用户凭据并生成 JWT。
func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginResponse, error) {
	login := strings.TrimSpace(strings.ToLower(input.Login))
	if login == "" || input.Password == "" {
		return LoginResponse{}, model.ErrInvalidLogin
	}
	user, err := s.repository.GetUserByLogin(ctx, login)
	if err != nil {
		if err == model.ErrUserNotFound {
			return LoginResponse{}, model.ErrInvalidCredentials
		}
		return LoginResponse{}, fmt.Errorf("查询登录用户：%w", err)
	}
	if user.Status != model.UserStatusActive {
		return LoginResponse{}, model.ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return LoginResponse{}, model.ErrInvalidCredentials
	}
	expires := time.Now().Add(s.tokenTTL)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID, "role": user.Role, "exp": expires.Unix(), "iat": time.Now().Unix()}).SignedString(s.jwtSecret)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("生成登录令牌：%w", err)
	}
	return LoginResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: int64(s.tokenTTL.Seconds())}, nil
}
