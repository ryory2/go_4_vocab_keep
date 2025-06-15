package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// AuthService インターフェースに VerifyAccount を追加
type AuthService interface {
	RegisterTenant(ctx context.Context, req *model.RegisterRequest) (*model.Tenant, error)
	VerifyAccount(ctx context.Context, tokenString string) error
	Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error)
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	HandleGoogleLogin(ctx context.Context, code string) (*model.LoginResponse, error)
}

// authService 構造体に依存関係を追加
type authService struct {
	db                *gorm.DB
	tenantRepo        repository.TenantRepository
	identityRepo      repository.IdentityRepository
	tokenRepo         repository.TokenRepository
	mailer            Mailer
	cfg               *config.Config
	googleOAuthConfig *oauth2.Config
}

func NewAuthService(db *gorm.DB, tenantRepo repository.TenantRepository, identityRepo repository.IdentityRepository, tokenRepo repository.TokenRepository, mailer Mailer, cfg *config.Config) AuthService {
	gOAuthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleOAuth.ClientID,
		ClientSecret: cfg.GoogleOAuth.ClientSecret,
		RedirectURL:  cfg.GoogleOAuth.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &authService{
		db:                db,
		tenantRepo:        tenantRepo,
		identityRepo:      identityRepo,
		tokenRepo:         tokenRepo,
		mailer:            mailer,
		cfg:               cfg,
		googleOAuthConfig: gOAuthConfig,
	}
}

func (s *authService) RegisterTenant(ctx context.Context, req *model.RegisterRequest) (*model.Tenant, error) {
	logger := middleware.GetLogger(ctx)
	var newTenant *model.Tenant

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Emailでの重複チェック
		_, err := s.tenantRepo.FindByEmail(ctx, tx, req.Email)
		if err == nil {
			logger.Warn("Email already exists", "email", req.Email)
			return model.NewAppError("DUPLICATE_EMAIL", "このメールアドレスは既に使用されています。", "email", model.ErrConflict)
		}
		if !errors.Is(err, model.ErrNotFound) {
			logger.Error("Failed to check email existence", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", err)
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの処理中にエラーが発生しました。", "", err)
		}
		hashedPasswordStr := string(hashedPassword)

		tenant := &model.Tenant{
			TenantID: uuid.New(),
			Name:     req.Name,
			Email:    req.Email,
			IsActive: false,
		}
		if err := s.tenantRepo.Create(ctx, tx, tenant); err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "ユーザーの作成に失敗しました。", "", err)
		}
		newTenant = tenant

		identity := &model.Identity{
			TenantID:     newTenant.TenantID,
			AuthProvider: model.AuthProviderLocal,
			ProviderID:   req.Email,
			PasswordHash: &hashedPasswordStr,
		}
		if err := s.identityRepo.Create(ctx, tx, identity); err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "認証情報の作成に失敗しました。", "", err)
		}

		tokenString, err := s.generateAndSaveVerificationToken(ctx, tx, newTenant.TenantID)
		if err != nil {
			return err
		}

		if err := s.sendVerificationEmail(ctx, newTenant.Email, tokenString); err != nil {
			return model.NewAppError("EMAIL_SEND_FAILED", "確認メールの送信に失敗しました。", "", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.Info("Tenant registered and verification email sent", "tenant_id", newTenant.TenantID, "email", newTenant.Email)
	return newTenant, nil
}

// VerifyAccount は提供されたトークンを検証し、アカウントを有効化します
func (s *authService) VerifyAccount(ctx context.Context, tokenString string) error {
	logger := middleware.GetLogger(ctx)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// トークンをDBから検索
		token, err := s.tokenRepo.FindVerificationToken(ctx, tx, tokenString)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				logger.Warn("Verification token not found", "token", tokenString)
				return model.NewAppError("INVALID_TOKEN", "このリンクは無効か、既に使用されています。", "token", model.ErrInvalidInput)
			}
			logger.Error("Error finding verification token", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "エラーが発生しました。", "", err)
		}

		// 有効期限をチェック
		if time.Now().After(token.ExpiresAt) {
			logger.Warn("Verification token expired", "token", tokenString, "expires_at", token.ExpiresAt)
			_ = s.tokenRepo.DeleteVerificationToken(ctx, tx, tokenString) // 期限切れトークンは削除
			return model.NewAppError("INVALID_TOKEN", "このリンクの有効期限が切れています。", "token", model.ErrInvalidInput)
		}

		// ユーザーを有効化
		updateResult := tx.Model(&model.Tenant{}).Where("tenant_id = ?", token.TenantID).Update("is_active", true)
		if updateResult.Error != nil {
			logger.Error("Failed to activate tenant account", "error", updateResult.Error, "tenant_id", token.TenantID)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "アカウントの有効化に失敗しました。", "", updateResult.Error)
		}
		if updateResult.RowsAffected == 0 {
			logger.Error("Tenant not found during activation", "tenant_id", token.TenantID)
			return model.NewAppError("NOT_FOUND", "アカウントが見つかりません。", "", model.ErrNotFound)
		}

		// 使用済みトークンを削除
		if err := s.tokenRepo.DeleteVerificationToken(ctx, tx, tokenString); err != nil {
			logger.Error("Failed to delete used verification token", "error", err, "token", tokenString)
			// トークン削除エラーは致命的ではないので、処理は続行する
		}

		logger.Info("Account verified successfully", "tenant_id", token.TenantID)
		return nil
	})
}

func (s *authService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	logger := middleware.GetLogger(ctx).With("email", req.Email)

	identity, err := s.identityRepo.FindByProvider(ctx, s.db, model.AuthProviderLocal, req.Email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.NewAppError("AUTHENTICATION_FAILED", "メールアドレスまたはパスワードが正しくありません。", "", model.ErrInvalidInput)
		}
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部エラー", "", err)
	}

	if identity.PasswordHash == nil {
		return nil, model.NewAppError("AUTHENTICATION_FAILED", "メールアドレスまたはパスワードが正しくありません。", "", model.ErrInvalidInput)
	}
	err = bcrypt.CompareHashAndPassword([]byte(*identity.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, model.NewAppError("AUTHENTICATION_FAILED", "メールアドレスまたはパスワードが正しくありません。", "", model.ErrInvalidInput)
	}

	tenant, err := s.tenantRepo.FindByID(ctx, s.db, identity.TenantID)
	if err != nil {
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "ユーザー情報の取得に失敗しました。", "", err)
	}

	if !tenant.IsActive {
		return nil, model.NewAppError("ACCOUNT_NOT_ACTIVE", "アカウントが有効化されていません。登録時に送信されたメールをご確認ください。", "", model.ErrForbidden)
	}

	logger.Info("Login successful", "tenant_id", tenant.TenantID)
	return s.generateAppJWT(ctx, tenant)
}

func (s *authService) HandleGoogleLogin(ctx context.Context, code string) (*model.LoginResponse, error) {
	logger := middleware.GetLogger(ctx)

	googleToken, err := s.googleOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, model.NewAppError("GOOGLE_AUTH_FAILED", "Google認証コードの交換に失敗しました。", "", err)
	}

	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleToken.AccessToken)
	if err != nil {
		return nil, model.NewAppError("GOOGLE_API_FAILED", "Googleからのユーザー情報取得に失敗しました。", "", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, model.NewAppError("GOOGLE_API_FAILED", "ユーザー情報の解析に失敗しました。", "", err)
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &googleUser); err != nil {
		return nil, model.NewAppError("GOOGLE_API_FAILED", "ユーザー情報の解析に失敗しました。", "", err)
	}
	logger = logger.With("google_email", googleUser.Email, "google_id", googleUser.ID)

	identity, err := s.identityRepo.FindByProvider(ctx, s.db, model.AuthProviderGoogle, googleUser.ID)
	if err == nil {
		logger.Info("Google user already exists, logging in")
		tenant, findErr := s.tenantRepo.FindByID(ctx, s.db, identity.TenantID)
		if findErr != nil {
			return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "ユーザー情報の取得に失敗しました。", "", findErr)
		}
		return s.generateAppJWT(ctx, tenant)
	}

	if !errors.Is(err, model.ErrNotFound) {
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "DBエラーが発生しました。", "", err)
	}

	logger.Info("New Google login detected, processing registration or linking")
	var targetTenant *model.Tenant
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existingTenant, findErr := s.tenantRepo.FindByEmail(ctx, tx, googleUser.Email)
		if findErr != nil && !errors.Is(findErr, model.ErrNotFound) {
			return findErr
		}

		if errors.Is(findErr, model.ErrNotFound) {
			logger.Info("No existing email, creating new tenant and identity")
			newTenant := &model.Tenant{
				TenantID: uuid.New(),
				Name:     googleUser.Name,
				Email:    googleUser.Email,
				IsActive: true,
			}
			if createErr := s.tenantRepo.Create(ctx, tx, newTenant); createErr != nil {
				return createErr
			}
			targetTenant = newTenant
		} else {
			logger.Info("Existing email found, linking Google identity to tenant", "tenant_id", existingTenant.TenantID)
			targetTenant = existingTenant
		}

		newIdentity := &model.Identity{
			TenantID:     targetTenant.TenantID,
			AuthProvider: model.AuthProviderGoogle,
			ProviderID:   googleUser.ID,
		}
		if createErr := s.identityRepo.Create(ctx, tx, newIdentity); createErr != nil {
			return createErr
		}
		return nil
	})
	if err != nil {
		return nil, model.NewAppError("DB_OPERATION_FAILED", "ユーザー処理中にエラーが発生しました。", "", err)
	}

	return s.generateAppJWT(ctx, targetTenant)
}

func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	logger := middleware.GetLogger(ctx).With("email", email)

	identity, err := s.identityRepo.FindByProvider(ctx, s.db, model.AuthProviderLocal, email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Warn("Password reset requested for non-existent local identity")
			return nil
		}
		return model.NewAppError("INTERNAL_SERVER_ERROR", "エラーが発生しました。", "", err)
	}

	tokenString, err := s.generateAndSavePasswordResetToken(ctx, s.db, identity.TenantID)
	if err != nil {
		return err
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.App.FrontendURL, tokenString)
	subject := "【Kioku】パスワードの再設定"
	body := fmt.Sprintf("パスワードを再設定するには、以下のリンクをクリックしてください:\n%s\n\nこのリンクの有効期限は1時間です。", resetURL)

	if err := s.mailer.Send(ctx, email, subject, body); err != nil {
		return model.NewAppError("EMAIL_SEND_FAILED", "メールの送信に失敗しました。", "", err)
	}

	logger.Info("Password reset email sent")
	return nil
}

func (s *authService) ResetPassword(ctx context.Context, tokenString, newPassword string) error {
	logger := middleware.GetLogger(ctx)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		token, err := s.tokenRepo.FindPasswordResetToken(ctx, tx, tokenString)
		if err != nil {
			return model.NewAppError("INVALID_TOKEN", "このリンクは無効か、既に使用されています。", "token", model.ErrInvalidInput)
		}
		if time.Now().After(token.ExpiresAt) {
			_ = s.tokenRepo.DeletePasswordResetToken(ctx, tx, tokenString)
			return model.NewAppError("INVALID_TOKEN", "このリンクの有効期限が切れています。", "token", model.ErrInvalidInput)
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの処理中にエラーが発生しました。", "", err)
		}
		hashedPasswordStr := string(hashedPassword)

		result := tx.Model(&model.Identity{}).
			Where("tenant_id = ? AND auth_provider = ?", token.TenantID, model.AuthProviderLocal).
			Update("password_hash", &hashedPasswordStr)
		if result.Error != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの更新に失敗しました。", "", result.Error)
		}
		if result.RowsAffected == 0 {
			return model.NewAppError("NOT_FOUND", "更新対象のローカルアカウントが見つかりません。", "", model.ErrNotFound)
		}

		if err := s.tokenRepo.DeletePasswordResetToken(ctx, tx, tokenString); err != nil {
			logger.Error("Failed to delete used password reset token", "error", err)
		}

		logger.Info("Password reset successfully", "tenant_id", token.TenantID)
		return nil
	})
}

func (s *authService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	tenant, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.NewAppError("TENANT_NOT_FOUND", "テナントが見つかりません。", "", model.ErrNotFound)
		}
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部エラー", "", err)
	}
	return tenant, nil
}

func (s *authService) generateAndSaveVerificationToken(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)
	verificationToken := &model.UserVerificationToken{Token: tokenString, TenantID: tenantID, ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := s.tokenRepo.CreateVerificationToken(ctx, tx, verificationToken); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの保存に失敗しました。", "", err)
	}
	return tokenString, nil
}

func (s *authService) sendVerificationEmail(ctx context.Context, email, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.App.FrontendURL, token)
	subject := "【Kioku】アカウントの有効化をお願いします"
	body := fmt.Sprintf("Kiokuにご登録いただきありがとうございます。\n\n以下のリンクをクリックしてアカウントを有効化してください:\n%s\n\nこのリンクの有効期限は24時間です。", verifyURL)
	return s.mailer.Send(ctx, email, subject, body)
}

func (s *authService) generateAndSavePasswordResetToken(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)
	resetToken := &model.PasswordResetToken{Token: tokenString, TenantID: tenantID, ExpiresAt: time.Now().Add(1 * time.Hour)}
	if err := s.tokenRepo.CreatePasswordResetToken(ctx, tx, resetToken); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの保存に失敗しました。", "", err)
	}
	return tokenString, nil
}

func (s *authService) generateAppJWT(ctx context.Context, tenant *model.Tenant) (*model.LoginResponse, error) {
	claims := &jwt.RegisteredClaims{
		Issuer:    s.cfg.App.Name,
		Subject:   tenant.TenantID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.JWT.AccessTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.cfg.JWT.SecretKey))
	if err != nil {
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}
	return &model.LoginResponse{AccessToken: signedToken}, nil
}
