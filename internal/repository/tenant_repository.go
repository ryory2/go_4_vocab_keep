//go:generate mockery --name TenantRepository --srcpkg go_4_vocab_keep/internal/repository --output ../repository/mocks --outpkg mocks --case=underscore
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
	Delete(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) error               // テナント名で検索するメソッド
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

// Delete は指定されたIDのテナントを論理削除します。
// gorm.Model を埋め込んでいるため、GORM は deleted_at カラムを更新します。
// 削除対象が見つからない場合、GORMのDeleteはエラーを返さないことが多いですが、
// RowsAffected で確認することも可能です。ここではシンプルに結果を返します。
func (r *gormTenantRepository) Delete(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) error {
	// 削除対象のモデルを指定して Delete を呼び出す
	// GORMは指定されたモデルのテーブル名と主キーを使って WHERE 句を組み立てる
	// この場合、 WHERE tenant_id = ? で検索し、見つかったレコードの deleted_at を更新する
	// --- GORMによるレコード削除処理 ---

	// db: GORMのデータベース接続オブジェクト (*gorm.DB)
	// .WithContext(ctx): GORMの操作にGoのコンテキスト(context.Context)を関連付けます。
	//                   これにより、タイムアウトやキャンセル信号をDB操作に伝搬できます。
	//                   リクエスト処理などでは、そのリクエストのコンテキストを渡すのが一般的です。
	// .Delete(引数1, 引数2): 指定された条件に一致するレコードをデータベースから削除するためのメソッドです。
	//
	//   引数1: &model.Tenant{}
	//          - 削除操作の対象となる「テーブル」をGORMに教えるための情報です。
	//          - GORMは `model.Tenant` 構造体の定義を見て、どのテーブル（通常は複数形で `tenants`）を
	//            操作すればよいか判断します。
	//          - `&model.Tenant{}` のように空の構造体のポインタを渡すのが一般的です。
	//            この構造体自体にデータが入っている必要はありません。型情報が重要です。
	//          - ★重要★: `model.Tenant` が `gorm.Model` を埋め込んでいる場合、
	//            この `Delete` 操作はデフォルトで「論理削除」になります。
	//            つまり、実際のレコードは削除されず、`deleted_at` カラムに現在時刻が設定されます。
	//            物理的に削除したい場合は、`.Unscoped().Delete(...)` を使います。
	//
	//   引数2: tenantID (uuid.UUID型)
	//          - 削除するレコードを特定するための「条件」です。
	//          - GORMは、引数1で指定されたモデル (`model.Tenant`) の「主キー」フィールド
	//            (通常は `ID` や、 `gorm:"primaryKey"` タグが付いたフィールド。
	//            この場合は `TenantID` が主キーと仮定）に対して、
	//            この `tenantID` の値が一致するレコードを削除対象とします。
	//          - SQLで言うと `WHERE tenant_id = [tenantIDの値]` のような条件が生成されます。
	//          - 複数の値を渡したり、構造体を渡して複数条件を指定することも可能です。
	//
	// 戻り値: result (*gorm.DB 型)
	//         - `Delete` メソッドを実行した結果を格納するGORMのオブジェクトです。
	//         - この `result` オブジェクトを通じて、エラー情報や影響を受けた行数などを確認できます。
	result := db.WithContext(ctx).Delete(&model.Tenant{}, tenantID) // tenantID を条件として渡す

	// エラーチェック
	if result.Error != nil {
		// 削除中に予期せぬDBエラーが発生した場合
		log.Printf("Error deleting tenant %s in DB: %v", tenantID, result.Error)
		return model.ErrInternalServer // 汎用エラーを返す
	}

	// オプション: 実際に削除された（影響を受けた）行数を確認する場合
	// if result.RowsAffected == 0 {
	// 	// 削除対象のレコードが見つからなかった場合
	// 	// ここで ErrNotFound を返すかどうかは要件による
	// 	// (冪等性を考慮すると、見つからなくてもエラーにしない場合もある)
	// 	// return model.ErrNotFound
	// }

	// エラーが発生しなければ nil を返す
	return nil
}
