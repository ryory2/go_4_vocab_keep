// internal/service/tenant_service_test.go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	// プロジェクトのパスに合わせて修正
	// インターフェース
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/repository/mocks" // 作成したモック

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite" // テスト用にインメモリDBを使う例（ただしDB操作はモックする）
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- テストヘルパー関数 ---
// setupTestDB は、テストで使用するための（この場合インメモリSQLiteの）
// GORMデータベース接続インスタンス (*gorm.DB) を作成して返す関数です。
// 実際のテストではDB操作はモックされますが、
// テスト対象のサービスが *gorm.DB 型の引数を必要とするため、形だけ用意します。
func setupTestDB() *gorm.DB {
	// gorm.Open で SQLite データベース（メモリ上）への接続を試みます。
	// logger.Silent で GORM のログ出力を抑制しています。
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	// もし接続に失敗したら、panic を起こしてテストを異常終了させます。
	// (テストの前提条件が満たせないため)
	if err != nil {
		panic("failed to connect database for testing: " + err.Error())
	}
	// 成功したら、DB接続インスタンスを返します。
	return db
}

// // --- テスト本体: テナント名が空の場合のみをテスト ---
// // --- テスト関数: Test_tenantService_CreateTenant_EmptyName ---
// // Goのテスト関数は "Test" で始まる名前を持ち、引数に *testing.T を取ります。
// // この関数は、tenantService の CreateTenant メソッドに空の名前("")を渡した場合の
// // 挙動をテストします。
// func Test_tenantService_CreateTenant_EmptyName(t *testing.T) {
// 	// t はテストの状態を管理し、ログ出力や失敗判定、ヘルパー関数の呼び出しなどに使います。

// 	// --- テスト用の準備 (Arrange) ---

// 	// context.Background() は、基本的な空のコンテキストを作成します。
// 	// 非同期処理のキャンセルや値の受け渡しに使われますが、このテストではプレースホルダ的な役割です。
// 	ctx := context.Background()

// 	// setupTestDB() を呼び出して、テスト用のDB接続インスタンスを取得します。
// 	// 実際にはDBにアクセスしませんが、NewTenantService に渡すために必要です。
// 	db := setupTestDB()

// 	// new(mocks.TenantRepository) は、mockery が生成したモック構造体
// 	// (TenantRepositoryインターフェースの偽物) の新しいインスタンスを作成します。
// 	// これにより、本物のリポジトリ(DBアクセス)を使わずにテストできます。
// 	mockTenantRepo := new(mocks.TenantRepository)

// 	// NewTenantService() は、テスト対象のサービスインスタンスを生成する関数です。
// 	// ここで、本物のDB接続(形だけ)と「偽物のリポジトリ(mockTenantRepo)」を
// 	// サービスに「注入(Dependency Injection)」しています。
// 	// これにより、テスト対象のサービスは、テスト中に我々が制御できる偽物のリポジトリと連携します。
// 	tenantService := NewTenantService(db, mockTenantRepo)

// 	// --- テスト実行 (Act & Assert) ---

// 	// t.Run() は、テスト関数内で「サブテスト」を定義・実行します。
// 	// テストをグループ化し、個別に成功/失敗を報告するのに便利です。
// 	// 第一引数はサブテストの名前、第二引数はサブテストを実行する関数です。
// 	t.Run("異常系: テナント名が空文字列", func(t *testing.T) {
// 		// t *testing.T はサブテスト用のテスト管理オブジェクトです。

// 		// --- モックの設定 (Arrange for subtest) ---
// 		// このテストケースでは、CreateTenant("") を呼び出すと、
// 		// サービス内の最初の if name == "" チェックで弾かれ、
// 		// リポジトリメソッド (Create) は呼び出されないはずです。
// 		// そのため、mockTenantRepo.On(...) のような事前設定は行いません。

// 		// --- テスト対象メソッドの実行 (Act) ---
// 		// 実際にテストしたい tenantService の CreateTenant メソッドを呼び出します。
// 		// 引数として、準備したコンテキスト ctx と、テストしたい値である空文字列 "" を渡します。
// 		// 戻り値として、作成されたテナント情報(のポインタ)とエラーを受け取ります。
// 		createdTenant, err := tenantService.CreateTenant(ctx, "")

// 		// --- アサーション (結果の検証 - Assert) ---
// 		// ここから、メソッド呼び出しの結果が期待通りだったかを確認します。

// 		// require.Error(t, err) は、変数 err が nil でないこと(エラーが発生したこと)を
// 		// 「必須条件」として検証します。もし err が nil なら、テストはこの時点で
// 		// 即座に失敗としてマークされ、以降のアサーションは実行されません。
// 		require.Error(t, err)

// 		// assert.ErrorIs(t, err, model.ErrInvalidInput) は、発生したエラー err が、
// 		// 期待されるエラーの種類 model.ErrInvalidInput と「同じ種類のエラー」であるかを検証します。
// 		// (errors.Is を使った比較)
// 		// require と違い、assert は失敗してもテストは中断せず、後続のアサーションも実行されます。
// 		assert.ErrorIs(t, err, model.ErrInvalidInput)

// 		// assert.Nil(t, createdTenant) は、変数 createdTenant が nil であること
// 		// (テナントが作成されなかったこと) を検証します。
// 		assert.Nil(t, createdTenant)

// 		// --- モックの検証 (Assert - モック固有) ---
// 		// モックオブジェクトが期待通りに使われたか（または使われなかったか）を確認します。

// 		// mockTenantRepo.AssertNotCalled(t, "Create", ...) は、
// 		// テスト実行中に mockTenantRepo の "Create" メソッドが
// 		// 一度も呼び出されなかったことを検証します。
// 		// mock.Anything は、引数の型が合っていればどんな値でもマッチすることを示します。
// 		// (今回は呼ばれないはずなので、引数の中身はあまり重要ではありません)
// 		mockTenantRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)

// 	}) // t.Run の終わり
// } // テスト関数の終わり

// --- テスト本体 ---
func Test_tenantService_CreateTenant(t *testing.T) {
	// --- テスト用の準備 ---
	// context.Background(): Go言語の標準ライブラリ context パッケージが提供する関数です。
	// これは、最も基本的で空の Context オブジェクトを生成します。
	// Context とは: リクエストのタイムアウト、キャンセル信号、リクエスト固有の値（ユーザーID、トレースIDなど）を関数呼び出し間で伝搬させるための仕組みです。
	// サーバーアプリケーションなどでは、リクエストごとに固有の Context が生成され、処理中に引き回されます。
	// なぜ空のcontextを使うのか：コンテキストの主な役割はタイムアウト・リクエスト情報だが、ユニットテストでは利用しないため
	ctx := context.Background()
	// テストで使うための *gorm.DB（GORMのデータベース接続オブジェクト）のインスタンスを準備して返す
	// gorm.Open を使ってインメモリのSQLiteデータベースに接続し、その接続オブジェクトを返し
	db := setupTestDB() // テスト用DB準備 (トランザクション用)
	// ユニットテストでは、実際のデータベース操作を行わずにサービスのロジックだけを検証したいので、このモックオブジェクト (mockTenantRepo)を利用
	// ★★★このインスタンスを利用して、どのような値が帰ってきてほしいかを設定する
	// mockTenantRepo には、このテストケース用の「期待されるメソッド呼び出しと、その際の戻り値」が setupMock 関数によって設定されています。具体的には:
	// 期待1: FindByName が ctx, *gorm.DB, "Test Tenant" という引数で 1回 呼ばれたら、nil テナントと model.ErrNotFound エラーを返す。
	// 期待2: Create が ctx, *gorm.DB, *model.Tenant という引数で 1回 呼ばれたら、nil エラーを返す。
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
			name: "異常系: テナント名が空",
			// 以下が実行される。引数ctxはからのコンテキスト、tt.inputNameはinputName。
			// createdTenant, err := tenantService.CreateTenant(ctx, tt.inputName)
			inputName: "", // 空文字を入力
			// tenantService.CreateTenant実行時に利用されるモックを設定
			setupMock: func() {
				// このケースではリポジトリメソッドは呼ばれないはずなので、
				// 何も設定しないか、意図的に呼ばれないことを AssertNotCalled で確認する。
				// mockTenantRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
			},
			wantErr:      model.ErrInvalidInput, // 入力エラーを期待
			wantTenantID: false,                 // テナントは作成されない
		},
		// { ... } は、テストケース一つ分を表す「構造体リテラル」です。
		// この中に、テストに必要な情報（名前、入力、期待結果など）をまとめて記述します。
		{
			// name: このテストケースが何を確認するものかを示す、人間が読むための名前です。
			name: "異常系: テナント名が重複",

			// inputName: テスト対象の CreateTenant 関数に渡す「テナント名」の入力値です。
			// testTenantName という変数には、あらかじめ "Existing Tenant" のような
			// 「既に存在するはず」という想定の名前が入っています。
			inputName: testTenantName, // 既存のテナント名と同じ名前を使う

			// setupMock: このテストケースを実行する「前」に行う「事前準備」を定義する関数です。
			// ここでは主に「モックオブジェクト」の振る舞いを設定します。
			// モックオブジェクトとは、テスト対象が依存している部品（ここではリポジトリ）の「偽物」で、
			// テスト中に都合の良い動きをさせることができます。
			setupMock: func() { // func() { ... } は、事前準備の処理内容をまとめた無名関数です。

				// --- モックの設定1: FindByName メソッドが呼ばれたときの動き ---
				// テスト対象の CreateTenant メソッドは、まず同じ名前のテナントがないか
				// リポジトリの FindByName メソッドを呼び出してチェックするはずです。
				// ここでは、その FindByName が呼ばれたときに「どうなってほしいか」を設定します。

				// 1-1. 既存のテナント情報をダミーで作成します。
				//     FindByName が「見つけた」という状況を作るためです。
				//     &model.Tenant{...} は、Tenant構造体の新しいインスタンス（へのポインタ）を作っています。
				//     IDや日時はテストに影響しないので、ダミーの値でOKです。
				existingTenant := &model.Tenant{
					TenantID:  uuid.New(),                 // 新しいユニークIDを生成（ダミー）
					Name:      testTenantName,             // 重複しているはずの名前
					CreatedAt: time.Now().Add(-time.Hour), // 適当な過去の時間
					UpdatedAt: time.Now().Add(-time.Hour), // 適当な過去の時間
				}

				// 1-2. mockTenantRepo (リポジトリの偽物) に対して、メソッド呼び出しの期待値を設定します。
				//     .On("メソッド名", 引数1, 引数2, ...) : 「このメソッドが、これらの引数で呼ばれたら」という設定を開始します。
				//       - "FindByName": 期待するメソッド名。
				//       - ctx: 期待する第一引数 (Context)。テストで使っている ctx と同じもの。
				//       - mock.AnythingOfType("*gorm.DB"): 期待する第二引数 (DB接続)。
				//         テスト実行時に具体的なDB接続インスタンスは確定できないので、
				//         「*gorm.DB 型の値なら何でもOK」という意味の特殊な指定を使います。
				//       - testTenantName: 期待する第三引数 (検索する名前)。テストで使う inputName と同じもの。
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					// .Return(戻り値1, 戻り値2, ...) : 上記の条件でメソッドが呼ばれたときに「返す値」を指定します。
					//   FindByName は (*model.Tenant, error) を返すので、2つの値を指定します。
					//   - existingTenant: 最初に準備した「見つかった」テナント情報を返します。
					//   - nil: エラーは発生しなかったことにします (見つかったのでErrNotFoundではない)。
					Return(existingTenant, nil). // ★★★ 既存テナントが見つかり、エラーは無し、という状況をシミュレート ★★★
					// .Once(): この FindByName の呼び出しは、このテストケース中に「1回だけ」
					//          行われるはずだ、という期待を指定します。
					//          もし2回呼ばれたり、1回も呼ばれなかったりしたら、テストの最後でエラーになります。
					Once()

				// --- モックの設定2: Create メソッドが呼ばれないことの期待 ---
				// テナント名が重複している場合、CreateTenant メソッドは
				// 新しいテナントを作成するリポジトリの Create メソッドを呼び出すべきではありません。
				// そのため、ここでは Create メソッドに対する .On(...) の設定は「行いません」。
				// もしテスト実行中に意図せず Create が呼ばれてしまった場合、
				// モックは「そんな呼び出しは聞いてないよ！」とパニックを起こすか、
				// テスト最後の AssertExpectations で「設定してないメソッドが呼ばれた」
				// または「設定したメソッドが全部呼ばれていない」としてエラーになります。
				// (コメントアウトされている AssertNotCalled を使って明示的に検証も可能です)

			}, // setupMock 関数の終わり

			// wantErr: このテストケースを実行した結果、CreateTenant 関数から
			//          返ってくることが「期待されるエラー」の種類を指定します。
			// model.ErrConflict は、おそらく "internal/model" パッケージで定義されている
			// 「重複エラー」を示すための変数（または定数）です。
			wantErr: model.ErrConflict, // ★★★ 期待するエラーは「重複エラー」 ★★★

			// wantTenantID: このテストケースでは、新しいテナントは作成されないはずなので、
			//               生成されたテナントIDがあるかどうかをチェックする必要はありません (false)。
			//               (正常系のテストでは true にして、IDが生成されたかチェックします)
			wantTenantID: false, // テナントは作成されない

		}, // テストケース構造体の終わり
		{
			name:      "正常系: 新規テナント作成成功",
			inputName: testTenantName,
			setupMock: func() {
				// 1. FindByName (重複チェック) が呼ばれるはず
				mockTenantRepo.On("FindByName", ctx, mock.AnythingOfType("*gorm.DB"), testTenantName).
					Return(nil, model.ErrNotFound). // 見つからない(ErrNotFound)ことを期待
					Once()                          // 1回だけ呼ばれる

				// 2. Create が呼ばれるはず
				// 期待値は（*model.Tenant, error）→（適当なmodel、nil）となるはず。
				// mockTenantRepo (リポジトリの偽物) に対して、メソッド呼び出しで期待したい値を設定
				//     .On("メソッド名", 引数1, 引数2, ...) : 「このメソッドが、これらの引数で呼ばれたら」という過程を設定
				//       - "Create": 期待するメソッド名。
				//       - ctx: 期待する第一引数 (Context)。テストで使っている ctx と同じもの。
				//       - mock.AnythingOfType("*gorm.DB"): 期待する第二引数 (DB接続)。
				//         テスト実行時に具体的なDB接続インスタンスは確定できないので、
				//         「*gorm.DB 型の値なら何でもOK」という意味の特殊な指定を使います。
				//       - testTenantName: 期待する第三引数 (検索する名前)。テストで使う inputName と同じもの。
				// 　　→この場合、Createメソッドが呼ばれる。
				// 　　→Createメソッドの引数は、「(ctx context.Context, db *gorm.DB, tenant *model.Tenant)」のため、3つの引数を容易
				// 　　→
				// 引数の tenant (*model.Tenant) は具体的な値がわからないため、mock.AnythingOfType を使う
				// mock.AnythingOfType("*model.Tenant")
				// この式自体が何か具体的な値を返すわけではなく、「この位置に来る引数は、
				// 指定された型（この場合は *model.Tenant）であれば、具体的な値が何であってもマッチするとみなす」というルールを設定
				// mock.On() メソッドは、このような特殊なマッチャーを受け取ることができるように設計されている。
				//
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
		// // --- ↓↓↓ テナント名が空の場合のテストケースを追加 ↓↓↓ ---
		{
			name: "異常系: テナント名が空文字列",
			// 以下が実行される。引数ctxはからのコンテキスト、tt.inputNameはinputName。
			// createdTenant, err := tenantService.CreateTenant(ctx, tt.inputName)
			inputName: "", // repositoryのcreate関数の引数。空文字列を入力とする
			setupMock: func() {
				// サービス層のバリデーションで弾かれるはずなので、
				// リポジトリのメソッド (Create) は呼び出されないことを期待。
				// そのため、ここでは On() による設定は不要。
			},
			wantErr: model.ErrInvalidInput, // サービス層で定義された入力エラーを期待
			// テストコードが tenantService.CreateTenant(ctx, "") を呼び出します。引数 name には "" が渡されます。
			// CreateTenant 関数内の if name == "" の条件が評価されます。name は "" なので、条件は true になります。
			// if 文の中の return nil, model.ErrInvalidInput が実行されます。
			// CreateTenant 関数はここで処理を終了し、nil と model.ErrInvalidInput を返します。
			// 重要な点: if 文で早期に return したため、その後のトランザクション処理や s.tenantRepo.Create(...) などのリポジトリメソッドは一切呼び出されません。
		},
		{
			name: "異常系: 重複チェック中にDBエラー",
			// 以下が実行される。引数ctxはからのコンテキスト、tt.inputNameはinputName。
			// createdTenant, err := tenantService.CreateTenant(ctx, tt.inputName)
			inputName: testTenantName,
			// tenantService.CreateTenant実行時に利用されるモックを設定
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
	}

	// --- テストケースの実行 ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// --- 各テスト前のモック設定 ---
			// 古い設定をリセット（必要に応じて）
			mockTenantRepo.Mock = mock.Mock{} // または testify の require を使う
			// 現在のテストケース用のモックを設定
			if tt.setupMock != nil {
				tt.setupMock() // モックを設定 (設定がないケースもある)
			}

			// --- テスト対象メソッドの実行 ---
			createdTenant, err := tenantService.CreateTenant(ctx, tt.inputName)

			// --- アサーション (結果の検証) ---
			if tt.wantErr != nil {
				// エラーが期待される場合
				require.Error(t, err) // エラーが発生したことを確認
				// エラーの種類を比較 (errors.Is がより堅牢)
				if !errors.Is(err, tt.wantErr) {
					// assert.ErrorIs だと失敗時にメッセージが見やすい
					assert.ErrorIs(t, err, tt.wantErr, "Expected error type %T, but got %T", tt.wantErr, err)
				}
				assert.Nil(t, createdTenant) // テナントは作成されないはず
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
