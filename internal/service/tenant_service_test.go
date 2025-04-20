// internal/service/tenant_service_test.go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go_4_vocab_keep/internal/model" // プロジェクトのパスに合わせて修正
	// インターフェース
	"go_4_vocab_keep/internal/repository/mocks" // 作成したモック

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite" // テスト用にインメモリDBを使う例（ただしDB操作はモックする）
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- テストヘルパー: テスト用のDBインスタンスを準備 ---
// このテストではDB操作自体はモックしますが、サービスが *gorm.DB を必要とするため準備します。
// トランザクションのテストを厳密に行う場合は sqlmock が必要です。
func setupTestDB() *gorm.DB {
	// インメモリSQLiteを使う（ファイルIOなし）
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // テスト中はログを抑制
	})
	if err != nil {
		panic("failed to connect database for testing: " + err.Error())
	}
	return db
}

// --- テスト本体 ---
func Test_tenantService_CreateTenant(t *testing.T) {
	// --- テスト用の準備 ---
	ctx := context.Background()
	db := setupTestDB()                           // テスト用DB準備 (トランザクション用)
	mockTenantRepo := new(mocks.TenantRepository) // モックリポジトリのインスタンス作成

	// テスト対象のサービスインスタンス作成 (モックを注入)
	tenantService := NewTenantService(db, mockTenantRepo)

	// --- テストケースの定義 ---
	testTenantName := "Test Tenant"

	// テストケースを構造体で管理すると見やすい
	tests := []struct {
		name         string // テストケース名
		inputName    string // CreateTenant に渡すテナント名
		setupMock    func() // 各テスト前のモック設定
		wantErr      error  // 期待されるエラー (nilならエラーなし)
		wantTenantID bool   // テナントIDが生成されることを期待するか
	}{
		{
			name:      "正常系: 新規テナント作成成功",
			inputName: testTenantName,
			setupMock: func() {
				// 1. FindByName (重複チェック) が呼ばれるはず
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					Return(nil, model.ErrNotFound). // 見つからない(ErrNotFound)ことを期待
					Once()                          // 1回だけ呼ばれる

				// 2. Create が呼ばれるはず
				// 引数の tenant (*model.Tenant) は具体的な値がわからないため、mock.AnythingOfType を使う
				mockTenantRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Tenant")).
					// 引数で渡されるテナントにIDがセットされているかなどを assert でチェックすることも可能
					// .Run(func(args mock.Arguments) {
					// 	arg := args.Get(2).(*model.Tenant) // 3番目の引数 (tenant) を取得
					// 	assert.NotEqual(t, uuid.Nil, arg.TenantID)
					// 	assert.Equal(t, testTenantName, arg.Name)
					// }).
					Return(nil). // Create は成功する (nil エラーを返す)
					Once()
			},
			wantErr:      nil,  // エラーは発生しない
			wantTenantID: true, // テナントIDは生成される
		},
		{
			name:      "異常系: テナント名が重複",
			inputName: testTenantName,
			setupMock: func() {
				// 1. FindByName (重複チェック) が呼ばれる
				existingTenant := &model.Tenant{TenantID: uuid.New(), Name: testTenantName, CreatedAt: time.Now()}
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					Return(existingTenant, nil). // 既存のテナントが見つかる
					Once()
				// 2. Create は呼ばれないはずなので、On() で設定しない
			},
			wantErr:      model.ErrConflict, // 重複エラーを期待
			wantTenantID: false,             // テナントは作成されない
		},
		{
			name:      "異常系: 重複チェック中にDBエラー",
			inputName: testTenantName,
			setupMock: func() {
				// 1. FindByName (重複チェック) が呼ばれ、予期せぬエラーが発生
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					Return(nil, errors.New("unexpected DB error")). // 何らかのDBエラー
					Once()
				// 2. Create は呼ばれない
			},
			wantErr:      model.ErrInternalServer, // 内部サーバーエラーを期待
			wantTenantID: false,
		},
		{
			name:      "異常系: テナント作成中にDBエラー",
			inputName: testTenantName,
			setupMock: func() {
				// 1. FindByName は成功 (見つからない)
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					Return(nil, model.ErrNotFound).
					Once()
				// 2. Create が呼ばれるが、エラーが発生
				mockTenantRepo.On("Create", ctx, mock.AnythingOfType("*gorm.DB"), mock.AnythingOfType("*model.Tenant")).
					Return(errors.New("failed to insert tenant")). // DBエラー
					Once()
			},
			wantErr:      model.ErrInternalServer, // 内部サーバーエラーを期待
			wantTenantID: false,
		},
		// 他に必要なテストケース（例: 入力名が空の場合など、サービス層でバリデーションするなら追加）
	}

	// --- テストケースの実行 ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// --- 各テスト前のモック設定 ---
			// 古い設定をリセット（必要に応じて）
			mockTenantRepo.Mock = mock.Mock{} // または testify の require を使う
			// 現在のテストケース用のモックを設定
			tt.setupMock()

			// --- テスト対象メソッドの実行 ---
			createdTenant, err := tenantService.CreateTenant(ctx, tt.inputName)

			// --- アサーション (結果の検証) ---
			if tt.wantErr != nil {
				// エラーが期待される場合
				require.Error(t, err)              // エラーが発生したことを確認
				assert.ErrorIs(t, err, tt.wantErr) // 期待した種類のエラーか確認
				assert.Nil(t, createdTenant)       // テナントは作成されないはず
			} else {
				// エラーが期待されない場合
				require.NoError(t, err)                           // エラーが発生しないことを確認
				require.NotNil(t, createdTenant)                  // テナントが作成されたことを確認
				assert.Equal(t, tt.inputName, createdTenant.Name) // 名前が正しいか
				if tt.wantTenantID {
					assert.NotEqual(t, uuid.Nil, createdTenant.TenantID) // IDが生成されているか
				}
			}

			// --- モックの検証 ---
			// 設定したモックメソッドが期待通りに呼び出されたか検証
			mockTenantRepo.AssertExpectations(t)
		})
	}
}
