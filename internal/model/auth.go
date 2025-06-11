package model

import (
	"github.com/golang-jwt/jwt/v5"
)

// LoginRequest はログインAPIのリクエストボディ
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse はログイン成功時のレスポンス
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	// RefreshToken string `json:"refresh_token,omitempty"` // リフレッシュトークンを実装する場合
}

// JWTCustomClaims はJWTに含めるカスタムクレーム（ペイロード）
type JWTCustomClaims struct {
	// ここにJWTに含めたい情報を定義する
	// 例: Name string, Role string など
	jwt.RegisteredClaims // 標準クレーム (iss, sub, exp など) を埋め込む
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}
type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}
