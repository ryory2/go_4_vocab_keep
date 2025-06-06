//go:generate mockery --name TenantService --srcpkg go_4_vocab_keep/internal/service --output ./mocks --outpkg mocks --case=underscore
package service

import (
	"context" // errorsを追加
	"errors"
	"log/slog"

	"go_4_vocab_keep/internal/model"      // プロジェクト名修正
	"go_4_vocab_keep/internal/repository" // プロジェクト名修正

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantService interface {
	CreateTenant(ctx context.Context, name string) (*model.Tenant, error)
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error)
	DeleteTenant(ctx context.Context, tenantID uuid.UUID) error
}

type tenantService struct {
	db         *gorm.DB
	tenantRepo repository.TenantRepository
	logger     *slog.Logger
}

func NewTenantService(db *gorm.DB, repo repository.TenantRepository, logger *slog.Logger) TenantService {
	return &tenantService{
		db:         db,
		tenantRepo: repo,
		logger:     logger,
	}
}

func (s *tenantService) CreateTenant(ctx context.Context, name string) (*model.Tenant, error) {
	// --- 入力バリデーション ---
	if name == "" {
		return nil, model.ErrInvalidInput
	}
	s.logger.DebugContext(ctx, "Attempting to create tenant", slog.String("name", name))
	// --- トランザクション内で重複チェックと作成を行う ---
	var createdTenant *model.Tenant
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. ★★★ 重複チェック ★★★
		existingTenant, err := s.tenantRepo.FindByName(ctx, tx, name)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			// FindByNameで予期せぬDBエラーが発生した場合
			s.logger.ErrorContext(ctx, "Error checking tenant name existence in repo", slog.Any("error", err), slog.String("name", name))
			return model.ErrInternalServer // 内部エラー
		}
		// エラーがなく (err == nil)、かつ existingTenant が見つかった場合 -> 重複
		if err == nil && existingTenant != nil {
			s.logger.WarnContext(ctx, "Tenant name already exists", slog.String("name", name), slog.String("existing_tenant_id", existingTenant.TenantID.String()))
			return model.ErrConflict // 重複エラーを返す
		}
		// ここまで到達した場合、err は model.ErrNotFound または nil (かつ existingTenant == nil) なので重複なし

		// 2. --- テナントの作成 ---
		tenant := &model.Tenant{
			TenantID: uuid.New(), // Service層でUUIDを生成
			Name:     name,
			// gorm.Model の CreatedAt, UpdatedAt は GORM が自動で設定
		}
		if err := s.tenantRepo.Create(ctx, tx, tenant); err != nil {
			// Create でエラーが発生した場合 (DB制約違反の可能性もあるが特定は難しい)
			s.logger.ErrorContext(ctx, "Error creating tenant in repo", slog.Any("error", err), slog.String("name", name))
			// TODO: GORMエラーから重複キーエラーを特定できれば ErrConflict を返す
			// if isDuplicateKeyError(err) { return model.ErrConflict }
			return model.ErrInternalServer // 汎用エラーを返す
		}

		// 作成成功したテナント情報を保持しておく
		createdTenant = tenant

		return nil // トランザクションをコミット
	})

	// --- トランザクション結果の処理 ---
	if err != nil {
		// トランザクション内で return したエラーがそのまま返ってくる
		// (model.ErrConflict や model.ErrInternalServer)
		return nil, err
	}
	s.logger.InfoContext(ctx, "Successfully created tenant", slog.String("tenant_id", createdTenant.TenantID.String()), slog.String("name", createdTenant.Name))

	// トランザクションが成功した場合、作成されたテナント情報を返す
	return createdTenant, nil
}

// GetTenant は指定されたIDのテナントを取得します (認証用などに利用)
func (s *tenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	operation := "GetTenant" // ログ用の操作名
	s.logger.Debug("テナント取得",
		slog.String("operation", operation),
		slog.String("tenant_id", tenantID.String()),
	)
	// 	s.logger.DebugContext(ctx, "Getting tenant", slog.String("tenant_id", tenantID.String()))
	tenant, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			s.logger.WarnContext(ctx, "Tenant not found", slog.String("tenant_id", tenantID.String()))
		} else {
			s.logger.ErrorContext(ctx, "Error finding tenant by ID", slog.Any("error", err), slog.String("tenant_id", tenantID.String()))
		}
		return nil, err
	}
	s.logger.DebugContext(ctx, "Tenant found", slog.String("tenant_id", tenantID.String()), slog.String("name", tenant.Name))
	return tenant, nil
}

// GetTenant は指定されたIDのテナントを取得します (認証用などに利用)
func (s *tenantService) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
	// --- 入力バリデーション ---
	if tenantID == uuid.Nil {
		return model.ErrInvalidInput
	}
	// _は戻り値の一部を受け取る必要がない場合に用いる
	// 通常、宣言したものは利用しないとエラーになるが、この場合は問題ない
	_, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		return err
	}
	err = s.tenantRepo.Delete(ctx, s.db, tenantID)
	if err != nil {
		return err
	}
	return nil
}
