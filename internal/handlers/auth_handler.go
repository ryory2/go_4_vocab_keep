package handlers

import (
	"errors"
	"net/http"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/service"
	"go_4_vocab_keep/internal/webutil"

	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	service service.AuthService
}

func NewAuthHandler(s service.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

// Register は新規ユーザーを登録し、有効化メールの送信をトリガーします
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	var req model.RegisterRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed for registration", "errors", validationErrors.Error())
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			logger.Error("Unexpected error during validation for registration", "error", err)
			webutil.HandleError(w, logger, err)
		}
		return
	}

	_, err := h.service.RegisterTenant(r.Context(), &req)
	if err != nil {
		logger.Error("Registration process failed in service", "error", err)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Registration request successful. Verification email sent.")
	webutil.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "確認メールを送信しました。メールボックスをご確認の上、アカウントを有効化してください。",
	}, logger)
}

// VerifyAccount は提供されたトークンでアカウントを有効化します
func (h *AuthHandler) VerifyAccount(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Warn("Verification attempt with no token")
		appErr := model.NewAppError("INVALID_REQUEST", "有効化トークンが必要です。", "token", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With("token_prefix", token[:min(8, len(token))]) // トークンの先頭だけログに残す

	logger.Info("Attempting to verify account")
	if err := h.service.VerifyAccount(r.Context(), token); err != nil {
		logger.Error("Account verification failed in service", "error", err)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Account successfully verified")
	webutil.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "アカウントが正常に有効化されました。ログインしてください。",
	}, logger)
}

// Login はユーザーを認証し、JWTを返します
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	var req model.LoginRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode login request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed for login", "errors", validationErrors.Error())
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			logger.Error("Unexpected error during validation for login", "error", err)
			webutil.HandleError(w, logger, err)
		}
		return
	}

	loginResponse, err := h.service.Login(r.Context(), &req)
	if err != nil {
		// サービス層でログは出力済みなので、ここではエラー処理に専念
		webutil.HandleError(w, logger, err)
		return
	}

	webutil.RespondWithJSON(w, http.StatusOK, loginResponse, logger)
}

// GetMe は認証済みユーザー自身の情報を返します
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}

	tenant, err := h.service.GetTenant(r.Context(), userID)
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}

	response := &model.TenantResponse{
		TenantID:  tenant.TenantID,
		Name:      tenant.Name,
		Email:     tenant.Email,
		IsActive:  tenant.IsActive,
		CreatedAt: tenant.CreatedAt,
	}

	webutil.RespondWithJSON(w, http.StatusOK, response, logger)
}

func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	// 1. リクエストボディをデコード
	var req model.ForgotPasswordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode forgot-password request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	// 2. バリデーション
	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			webutil.HandleError(w, logger, err)
		}
		return
	}

	// 3. サービス層の呼び出し
	if err := h.service.RequestPasswordReset(r.Context(), req.Email); err != nil {
		// サービス層でエラーが発生した場合 (通常は内部エラーかメール送信エラー)
		webutil.HandleError(w, logger, err)
		return
	}

	// 4. 成功レスポンス
	// ユーザーが存在しない場合でも、セキュリティのために同じ成功メッセージを返す
	webutil.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "ご入力のメールアドレスにパスワード再設定用のリンクを送信しました。メールが届かない場合は、迷惑メールフォルダもご確認ください。",
	}, logger)
}

// ResetPassword は新しいパスワードへのリセットを実行します
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	// 1. リクエストボディをデコード
	var req model.ResetPasswordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode reset-password request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	// 2. バリデーション
	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			webutil.HandleError(w, logger, err)
		}
		return
	}

	// 3. サービス層の呼び出し
	if err := h.service.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		// サービス層から返されたエラー (無効なトークンなど) を処理
		webutil.HandleError(w, logger, err)
		return
	}

	// 4. 成功レスポンス
	webutil.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "パスワードが正常に更新されました。",
	}, logger)
}
