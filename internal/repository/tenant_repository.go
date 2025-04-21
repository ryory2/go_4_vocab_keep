//go:generate mockery --name TenantRepository --dir . --output ./mocks --outpkg mocks --structname TenantRepository --filename tenant_repository_mock.go
package repository

import (
	"context"
	"errors"
	"log" // Logを追加

	"go_4_vocab_keep/internal/model" // プロジェクト名修正

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TenantRepository interface {
	Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error
	FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error)
	FindByName(ctx context.Context, db *gorm.DB, name string) (*model.Tenant, error) // テナント名で検索するメソッド
}

type gormTenantRepository struct{}

func NewGormTenantRepository() TenantRepository {
	return &gormTenantRepository{}
}

func (r *gormTenantRepository) Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	// UUIDはService層で設定するか、DBのデフォルト機能に任せる（ここではServiceで設定想定）
	// tenant.TenantID = uuid.New()
	result := db.WithContext(ctx).Create(tenant)
	if result.Error != nil {
		// TODO: より詳細なエラーハンドリング (例: 重複キーエラー)
		log.Printf("Error creating tenant in DB: %v", result.Error)
	}
	return result.Error
}

func (r *gormTenantRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error) {
	var tenant model.Tenant
	// First は自動で DeletedAt IS NULL を考慮する
	result := db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&tenant)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, model.ErrNotFound
		}
		log.Printf("Error finding tenant %s in DB: %v", tenantID, result.Error)
		return nil, model.ErrInternalServer // DBエラーは汎用エラーに
	}
	return &tenant, nil
}

// --- ★★★ FindByName メソッドの実装 ★★★ ---
// FindByName は指定された名前を持つテナントを検索します。
// テナント名が一意制約を持つことを想定し、最初に見つかったレコードを返します。
// 見つからない場合は model.ErrNotFound を返します。
func (r *gormTenantRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*model.Tenant, error) {
	var tenant model.Tenant // 結果を格納するための model.Tenant 型の変数を宣言

	// db.WithContext(ctx): 現在のリクエストのコンテキストをGORMクエリに関連付けます。
	// .Where("name = ?", name): SQLのWHERE句を構築します。
	//   - "name = ?" : カラム名 `name` がプレースホルダ `?` と一致する条件。
	//   - name (第二引数): プレースホルダ `?` にバインドされる実際のテナント名。
	//     GORMが適切にエスケープ処理を行うため、SQLインジェクションを防ぎます。
	// .First(&tenant): WHERE句の条件に一致する最初のレコードを探し、
	//   見つかった場合はそのデータを `tenant` 変数（ポインタを渡す）に格納します。
	//   .First は gorm.Model の DeletedAt (論理削除) も自動で考慮します (deleted_at IS NULL)。
	result := db.WithContext(ctx).Where("name = ?", name).First(&tenant)

	// --- エラーハンドリング ---
	if result.Error != nil {
		// result.Error にエラー情報が含まれているかチェック

		// errors.Is() を使って、発生したエラーが gorm.ErrRecordNotFound かどうかを判定
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// レコードが見つからなかった場合 (これは予期されるエラーの一種)
			// データベース固有のエラーではなく、アプリケーション定義の「見つからない」エラーを返す
			return nil, model.ErrNotFound
		}

		// レコードが見つからない以外のデータベースエラーが発生した場合
		// (例: DB接続断、テーブルが存在しない、権限エラーなど)
		log.Printf("Error finding tenant by name '%s' in DB: %v", name, result.Error)
		// 予期せぬDBエラーとして、アプリケーション定義の内部サーバーエラーを返す
		return nil, model.ErrInternalServer
	}

	// エラーが発生しなかった場合 (result.Error == nil)
	// 見つかったテナントのデータが tenant 変数に格納されている
	// そのテナントへのポインタ (&tenant) と、エラーなし (nil) を返す
	return &tenant, nil
}
