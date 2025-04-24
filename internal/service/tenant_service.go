//go:generate mockery --name TenantService --srcpkg go_4_vocab_keep/internal/service --output ./mocks --outpkg mocks --case=underscore
package service

import (
	"context" // errorsを追加
	"errors"
	"log"

	"go_4_vocab_keep/internal/model"      // プロジェクト名修正
	"go_4_vocab_keep/internal/repository" // プロジェクト名修正

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantService interface {
	CreateTenant(ctx context.Context, name string) (*model.Tenant, error)
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) // GetTenantを追加
	DeleteTenant(ctx context.Context, tenantID uuid.UUID) error               // GetTenantを追加
}

type tenantService struct {
	db         *gorm.DB
	tenantRepo repository.TenantRepository
}

func NewTenantService(db *gorm.DB, repo repository.TenantRepository) TenantService {
	return &tenantService{db: db, tenantRepo: repo}
}

func (s *tenantService) CreateTenant(ctx context.Context, name string) (*model.Tenant, error) {
	// --- 入力バリデーション ---
	if name == "" {
		return nil, model.ErrInvalidInput
	}

	// --- トランザクション内で重複チェックと作成を行う ---
	var createdTenant *model.Tenant
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. ★★★ 重複チェック ★★★
		existingTenant, err := s.tenantRepo.FindByName(ctx, tx, name)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			// FindByNameで予期せぬDBエラーが発生した場合
			log.Printf("Error checking tenant name existence in repo: %v", err)
			return model.ErrInternalServer // 内部エラー
		}
		// エラーがなく (err == nil)、かつ existingTenant が見つかった場合 -> 重複
		if err == nil && existingTenant != nil {
			log.Printf("Error Conflict: %v", err)
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
			log.Printf("Error creating tenant in repo: %v", err)
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

	// トランザクションが成功した場合、作成されたテナント情報を返す
	return createdTenant, nil
}

// GetTenant は指定されたIDのテナントを取得します (認証用などに利用)
func (s *tenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*model.Tenant, error) {
	tenant, err := s.tenantRepo.FindByID(ctx, s.db, tenantID)
	if err != nil {
		// model.ErrNotFound や model.ErrInternalServer が返る想定
		return nil, err
	}
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
