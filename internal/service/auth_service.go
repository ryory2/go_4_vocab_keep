package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go_4_vocab_keep/internal/config"
	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
}

// authService 構造体に依存関係を追加
type authService struct {
	db         *gorm.DB
	tenantRepo repository.TenantRepository
	tokenRepo  repository.TokenRepository
	mailer     Mailer
	cfg        *config.Config
}

// NewAuthService は AuthService の新しいインスタンスを生成します
func NewAuthService(db *gorm.DB, tenantRepo repository.TenantRepository, tokenRepo repository.TokenRepository, mailer Mailer, cfg *config.Config) AuthService {
	return &authService{
		db:         db,
		tenantRepo: tenantRepo,
		tokenRepo:  tokenRepo,
		mailer:     mailer,
		cfg:        cfg,
	}
}

// RegisterTenant は新しいユーザーを登録し、有効化メールを送信します
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

		// Nameでの重複チェック
		_, err = s.tenantRepo.FindByName(ctx, tx, req.Name)
		if err == nil {
			logger.Warn("Tenant name already exists", "name", req.Name)
			return model.NewAppError("DUPLICATE_NAME", "そのユーザ名は既に使用されています。", "name", model.ErrConflict)
		}
		if !errors.Is(err, model.ErrNotFound) {
			logger.Error("Failed to check name existence", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部でエラーが発生しました。", "", err)
		}

		// パスワードのハッシュ化
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Error("Failed to hash password", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの処理中にエラーが発生しました。", "", err)
		}

		// 新しいTenantモデルを作成
		tenant := &model.Tenant{
			TenantID:     uuid.New(),
			Name:         req.Name,
			Email:        req.Email,
			PasswordHash: string(hashedPassword),
			IsActive:     false,
		}

		// ユーザーをDBに保存
		if err := s.tenantRepo.Create(ctx, tx, tenant); err != nil {
			// Create内で重複エラーが検知された場合 (レースコンディション対策)
			if errors.Is(err, model.ErrConflict) {
				logger.Warn("Conflict during tenant creation (race condition)", "error", err)
				return model.NewAppError("DUPLICATE_ENTRY", "指定された名前またはEmailは既に使用されています。", "name,email", model.ErrConflict)
			}
			logger.Error("Failed to create tenant in DB", "error", err)
			return model.NewAppError("INTERNAL_SERVER_ERROR", "ユーザーの作成に失敗しました。", "", err)
		}
		newTenant = tenant

		// --- メール認証トークン生成・メール送信処理 ---
		tokenString, err := s.generateAndSaveVerificationToken(ctx, tx, newTenant.TenantID)
		if err != nil {
			// generateAndSaveVerificationToken内でログは出力済み
			return err
		}

		if err := s.sendVerificationEmail(ctx, newTenant.Email, tokenString); err != nil {
			// sendVerificationEmail内でログは出力済み
			return model.NewAppError("EMAIL_SEND_FAILED", "確認メールの送信に失敗しました。時間をおいて再度お試しください。", "", err)
		}

		return nil // トランザクション成功
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

// Login はユーザーを認証し、JWTを返します
func (s *authService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	logger := middleware.GetLogger(ctx).With("email", req.Email)

	tenant, err := s.tenantRepo.FindByEmail(ctx, s.db, req.Email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Warn("Login failed: user not found")
			return nil, model.NewAppError("AUTHENTICATION_FAILED", "メールアドレスまたはパスワードが正しくありません。", "", model.ErrInvalidInput)
		}
		logger.Error("Login failed: db error on FindByEmail", "error", err)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部エラー", "", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(tenant.PasswordHash), []byte(req.Password))
	if err != nil {
		logger.Warn("Login failed: password mismatch", "tenant_id", tenant.TenantID)
		return nil, model.NewAppError("AUTHENTICATION_FAILED", "メールアドレスまたはパスワードが正しくありません。", "", model.ErrInvalidInput)
	}

	if !tenant.IsActive {
		logger.Warn("Login failed: account not active", "tenant_id", tenant.TenantID)
		return nil, model.NewAppError("ACCOUNT_NOT_ACTIVE", "アカウントが有効化されていません。登録時に送信されたメールをご確認ください。", "", model.ErrForbidden)
	}

	claims := &jwt.RegisteredClaims{
		Issuer:    s.cfg.App.Name,
		Subject:   tenant.TenantID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.JWT.AccessTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.cfg.JWT.SecretKey))
	if err != nil {
		logger.Error("Failed to sign JWT", "error", err, "tenant_id", tenant.TenantID)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}

	logger.Info("Login successful", "tenant_id", tenant.TenantID)
	return &model.LoginResponse{AccessToken: signedToken}, nil
}

// GetTenant は指定されたIDのテナントを取得します
func (s *authService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	logger := middleware.GetLogger(ctx)
	logger.Debug("Getting tenant by ID", "tenant_id", tenantID.String())
	tenant, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Warn("Tenant not found", "tenant_id", tenantID.String())
			return nil, model.NewAppError("TENANT_NOT_FOUND", "テナントが見つかりません。", "", model.ErrNotFound)
		}
		logger.Error("Error finding tenant by ID", "error", err)
		return nil, model.NewAppError("INTERNAL_SERVER_ERROR", "サーバー内部エラー", "", err)
	}
	logger.Debug("Tenant found", "tenant_id", tenantID.String())
	return tenant, nil
}

// --- ヘルパー関数 ---

func (s *authService) generateAndSaveVerificationToken(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID) (string, error) {
	logger := middleware.GetLogger(ctx)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		logger.Error("Failed to generate random bytes for token", "error", err)
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)

	verificationToken := &model.UserVerificationToken{
		Token:     tokenString,
		TenantID:  tenantID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := s.tokenRepo.CreateVerificationToken(ctx, tx, verificationToken); err != nil {
		// CreateVerificationToken内でログは出力済みなので、ここではエラーをラップして返すだけ
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの保存に失敗しました。", "", err)
	}
	return tokenString, nil
}

func (s *authService) sendVerificationEmail(ctx context.Context, email, token string) error {
	logger := middleware.GetLogger(ctx)
	// フロントエンドのURLを設定ファイルから取得
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.cfg.App.FrontendURL, token)
	subject := "【Kioku】アカウントの有効化をお願いします"
	body := fmt.Sprintf("Kiokuにご登録いただきありがとうございます。\n\n以下のリンクをクリックしてアカウントを有効化してください:\n%s\n\nこのリンクの有効期限は24時間です。", verifyURL)

	logger.Info("Sending verification email", "to", email) // 送信前にログ
	return s.mailer.Send(ctx, email, subject, body)
}

func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	logger := middleware.GetLogger(ctx).With("email", email)

	tenant, err := s.tenantRepo.FindByEmail(ctx, s.db, email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Warn("Password reset requested for non-existent email")
			// ユーザーが存在しない場合でも、それを悟られないように成功として扱う
			return nil
		}
		return model.NewAppError("INTERNAL_SERVER_ERROR", "エラーが発生しました。", "", err)
	}

	// トークン生成
	tokenString, err := s.generateAndSavePasswordResetToken(ctx, s.db, tenant.TenantID)
	if err != nil {
		return err // 内部でAppErrorにラップ済み
	}

	// メール送信
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.App.FrontendURL, tokenString)
	subject := "【Kioku】パスワードの再設定"
	body := fmt.Sprintf("パスワードを再設定するには、以下のリンクをクリックしてください:\n%s\n\nこのリンクの有効期限は1時間です。", resetURL)

	if err := s.mailer.Send(ctx, tenant.Email, subject, body); err != nil {
		return model.NewAppError("EMAIL_SEND_FAILED", "メールの送信に失敗しました。", "", err)
	}

	logger.Info("Password reset email sent")
	return nil
}

// --- ResetPassword メソッドを新規実装 ---
func (s *authService) ResetPassword(ctx context.Context, tokenString, newPassword string) error {
	logger := middleware.GetLogger(ctx)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. トークンを検証
		token, err := s.tokenRepo.FindPasswordResetToken(ctx, tx, tokenString)
		if err != nil {
			return model.NewAppError("INVALID_TOKEN", "このリンクは無効か、既に使用されています。", "token", model.ErrInvalidInput)
		}
		if time.Now().After(token.ExpiresAt) {
			_ = s.tokenRepo.DeletePasswordResetToken(ctx, tx, tokenString)
			return model.NewAppError("INVALID_TOKEN", "このリンクの有効期限が切れています。", "token", model.ErrInvalidInput)
		}

		// 2. 新しいパスワードをハッシュ化
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの処理中にエラーが発生しました。", "", err)
		}

		// 3. パスワードを更新
		result := tx.Model(&model.Tenant{}).Where("tenant_id = ?", token.TenantID).Update("password_hash", string(hashedPassword))
		if result.Error != nil || result.RowsAffected == 0 {
			return model.NewAppError("INTERNAL_SERVER_ERROR", "パスワードの更新に失敗しました。", "", result.Error)
		}

		// 4. 使用済みトークンを削除
		if err := s.tokenRepo.DeletePasswordResetToken(ctx, tx, tokenString); err != nil {
			logger.Error("Failed to delete used password reset token", "error", err)
		}

		logger.Info("Password reset successfully", "tenant_id", token.TenantID)
		return nil
	})
}

// --- パスワードリセット用のヘルパー関数 ---
func (s *authService) generateAndSavePasswordResetToken(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの生成に失敗しました。", "", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)
	resetToken := &model.PasswordResetToken{
		Token:     tokenString,
		TenantID:  tenantID,
		ExpiresAt: time.Now().Add(1 * time.Hour), // 有効期限は1時間
	}
	if err := s.tokenRepo.CreatePasswordResetToken(ctx, tx, resetToken); err != nil {
		return "", model.NewAppError("INTERNAL_SERVER_ERROR", "トークンの保存に失敗しました。", "", err)
	}
	return tokenString, nil
}
