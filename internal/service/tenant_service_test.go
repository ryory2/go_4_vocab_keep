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

// --- ここから GetTenant のテストコード ---
func Test_tenantService_GetTenant(t *testing.T) {
	// --- テスト用の準備 ---
	ctx := context.Background()
	db := setupTestDB() // GetTenant は db を直接使うが、操作はモック経由
	mockTenantRepo := new(mocks.TenantRepository)
	tenantService := NewTenantService(db, mockTenantRepo)

	testTenantID := uuid.New()       // テスト用の固定ID
	expectedTenant := &model.Tenant{ // 正常系で返されることを期待するテナント
		TenantID:  testTenantID,
		Name:      "Test Tenant Get",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		// gorm.Model のフィールドは GORM が自動で埋めるか、DBから読み込む
	}

	// --- テストケースの定義 ---
	tests := []struct {
		name       string
		inputID    uuid.UUID
		setupMock  func(m *mocks.TenantRepository) // モック設定関数
		wantErr    error
		wantTenant *model.Tenant // 期待されるテナント (エラー時はnil)
	}{
		{
			name:    "正常系: テナント取得成功",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				// FindByID が ctx, db, testTenantID で呼ばれたら、
				// expectedTenant と nil エラーを返すように設定
				m.On("FindByID", ctx, db, testTenantID).
					Return(expectedTenant, nil).Once()
			},
			wantErr:    nil,            // エラーは期待しない
			wantTenant: expectedTenant, // 期待するテナント
		},
		{
			name:    "異常系: テナントが見つからない",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				// FindByID が ctx, db, testTenantID で呼ばれたら、
				// nil テナントと model.ErrNotFound を返すように設定
				m.On("FindByID", ctx, db, testTenantID).
					Return(nil, model.ErrNotFound).Once()
			},
			wantErr:    model.ErrNotFound, // 見つからないエラーを期待
			wantTenant: nil,               // テナントは返されない
		},
		{
			name:    "異常系: リポジトリで予期せぬエラーが発生",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				m.On("FindByID", ctx, db, testTenantID).
					Return(nil, model.ErrInternalServer).Once()
			},
			// GetTenant はリポジトリのエラーをそのまま返す実装なので、
			// モックで設定したエラーがそのまま返ることを期待
			wantErr:    model.ErrInternalServer,
			wantTenant: nil,
		},
		{
			name:    "異常系: 不正なID (uuid.Nil)",
			inputID: uuid.Nil, // ゼロ値のUUID
			setupMock: func(m *mocks.TenantRepository) {
				// 不正なIDで FindByID が呼ばれた場合、
				// 見つからないエラー (ErrNotFound) が返ると想定
				m.On("FindByID", ctx, db, uuid.Nil).
					Return(nil, model.ErrNotFound).Once()
			},
			wantErr:    model.ErrNotFound, // 見つからないエラーを期待
			wantTenant: nil,
		},
	}

	// --- テストケースの実行 ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// --- 各テスト前のモックリセットと再設定 ---
			mockTenantRepo.Mock = mock.Mock{} // モックの状態をリセット
			if tt.setupMock != nil {
				tt.setupMock(mockTenantRepo) // 現在のテストケース用のモックを設定
			}

			// --- テスト対象メソッドの実行 ---
			tenant, err := tenantService.GetTenant(ctx, tt.inputID)

			// --- アサーション (結果の検証) ---
			if tt.wantErr != nil {
				// エラーが期待される場合
				require.Error(t, err, "Expected an error but got nil")
				// エラーの種類または内容を比較
				// サービスがエラーをラップしていない場合、errors.Is が有効
				assert.ErrorIs(t, err, tt.wantErr, "Expected error '%v', but got '%v'", tt.wantErr, err)
				// assert.EqualError(t, err, tt.wantErr.Error()) // エラーメッセージ文字列で比較する場合
				assert.Nil(t, tenant, "Tenant should be nil on error")
			} else {
				// エラーが期待されない場合
				require.NoError(t, err, "Did not expect an error but got: %v", err)
				require.NotNil(t, tenant, "Expected a tenant but got nil")
				// 取得したテナントが期待通りか比較
				assert.Equal(t, tt.wantTenant, tenant, "Returned tenant does not match expected tenant")
			}

			// --- モックの検証 ---
			// 設定したモックメソッドが期待通りに呼び出されたか検証
			mockTenantRepo.AssertExpectations(t)
		})
	}
}

// --- ここから DeleteTenant のテストコード ---
func Test_tenantService_DeleteTenant(t *testing.T) {
	// --- テスト用の準備 ---
	ctx := context.Background()
	db := setupTestDB() // GetTenant は db を直接使うが、操作はモック経由
	mockTenantRepo := new(mocks.TenantRepository)
	tenantService := NewTenantService(db, mockTenantRepo)

	testTenantID := uuid.New() // テスト用の固定ID

	dummyTenant := &model.Tenant{ // FindByIDが返すダミーのテナント情報
		TenantID:  testTenantID,
		Name:      "Tenant to Delete",
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}

	// --- テストケースの定義 ---
	tests := []struct {
		name      string
		inputID   uuid.UUID
		setupMock func(m *mocks.TenantRepository) // モック設定関数
		wantErr   error
	}{
		{
			name:    "正常系: テナント削除成功",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				// FindByID が ctx, db, testTenantID で呼ばれたら、
				// expectedTenant と nil エラーを返すように設定
				m.On("FindByID", ctx, db, testTenantID).
					Return(dummyTenant, nil).Once()
				m.On("Delete", ctx, db, testTenantID).
					Return(nil).Once()
			},
			wantErr: nil, // エラーは期待しない
		},
		{
			name:    "異常系: テナントIDが空",
			inputID: uuid.Nil, // 空のUUIDを入力
			setupMock: func(m *mocks.TenantRepository) {
			},
			wantErr: model.ErrInvalidInput, // 入力エラーを期待
		},
		{
			name:    "異常系: テナントが見つからない",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				// FindByID が ctx, db, testTenantID で呼ばれたら、
				// nil テナントと model.ErrNotFound を返すように設定
				m.On("FindByID", ctx, db, testTenantID).
					Return(nil, model.ErrNotFound).Once()
			},
			wantErr: model.ErrNotFound, // 見つからないエラーを期待
		},
		{
			name:    "異常系: リポジトリで予期せぬエラーが発生",
			inputID: testTenantID,
			setupMock: func(m *mocks.TenantRepository) {
				// FindByID が呼ばれたら、nil と simulatedError を返す
				m.On("FindByID", ctx, db, testTenantID).
					Return(nil, model.ErrInternalServer).Once()
			},
			// GetTenant はリポジトリのエラーをそのまま返す実装なので、
			// モックで設定したエラーがそのまま返ることを期待
			wantErr: model.ErrInternalServer,
		},
	}

	// --- テストケースの実行 ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// --- 各テスト前のモックリセットと再設定 ---
			mockTenantRepo.Mock = mock.Mock{} // モックの状態をリセット
			if tt.setupMock != nil {
				tt.setupMock(mockTenantRepo) // 現在のテストケース用のモックを設定
			}

			// --- テスト対象メソッドの実行 ---
			err := tenantService.DeleteTenant(ctx, tt.inputID)

			// 基本的な書式
			// ほとんどの require および assert の関数は、以下のような引数の構成になっています。
			// パッケージ名.関数名(t *testing.T, 引数1, [引数2, ...], [msgAndArgs ...interface{}])
			// 　パッケージ名: require または assert のどちらかです。
			// 　　require：必須。ない場合に、エラーとしてテストを中断する。
			// 　　assert：任意。それがなくとも、エラーがあってもテストを続行する。
			// 　関数名: 実行したいチェック内容を表す名前です（例: Error, NoError, Equal, Nil）。
			// 　　Error: 「（引数で渡されたものが）エラーであること」を確認する。
			// 　　NoError: 「（引数で渡されたものが）エラーでないこと（＝nilであること）」を確認する。
			// 　　Equal: 「（引数で渡された2つのものが）等しい（Equal）こと」を確認する。
			// 　　Nil: 「（引数で渡されたものが）nilであること」を確認する。
			// 　　NotNil: 「（引数で渡されたものが）nilでないこと」を確認する。
			// 　　True: 「（引数で渡されたものが）真（True）であること」を確認する。
			// 　　False: 「（引数で渡されたものが）偽（False）であること」を確認する。
			// 　　Len: 「（引数で渡されたコレクションの）長さ（Length）が、期待する値と等しいこと」を確認する。
			// 　t *testing.T: これはGoの標準の testing パッケージから渡されるテストの状態を管理するオブジェクトです。テスト関数には必ず引数として渡ってくるので、それをそのまま渡します。テストフレームワークに結果（成功/失敗）を報告するために必要です。常に最初の引数になります。
			// 　引数1, [引数2, ...]: チェックする対象の値や、期待する値などが入ります。関数によって必要な引数の数や種類が変わります。
			// 　[msgAndArgs ...interface{}]: これはオプション（省略可能）の引数です。テストが失敗した場合に表示されるカスタムメッセージを指定できます。
			// 　　最初の要素がメッセージ文字列（書式指定子 %v, %s, %d などを含めることができる）で、その後に書式指定子に対応する値を順番に指定します。
			// 　　fmt.Printf と同じような感覚で使えます。
			// 　　指定しない場合は、ライブラリがデフォルトのエラーメッセージを表示します。

			// --- アサーション (結果の検証) ---
			// require: チェックして、もし間違っていたら即座にテストを中断します。「これは絶対に満たされていないとおかしい！」という強いチェックに使います。
			// assert: チェックして、もし間違っていてもテストは中断せず、最後まで続行します。ただし、間違っていたことは記録されます。複数の項目をチェックしたい場合に便利です。
			if tt.wantErr != nil {
				// require.Error(t, err, msgAndArgs...)
				// 書式: require.Error(t *testing.T, err error, msgAndArgs ...interface{})
				// 意味: err が nil でないこと（エラーが必須であること）を要求。エラーがない場合、第三引数を出力する。
				// 引数:
				// 　t: テストオブジェクト。
				// 　err: チェック対象のエラー変数。
				// 　msgAndArgs: 失敗時（err が nil だった場合）のカスタムメッセージ（省略可能）。
				// 例: require.Error(t, err, "関数Xはエラーを返すべきなのにnilが返りました")
				require.Error(t, err, "Expected an error but got nil")
				// サービスがエラーをラップしていない場合、errors.Is が有効
				// assert.ErrorIs(t, err, tt.wantErr, msgAndArgs...)
				// 書式: assert.ErrorIs(t *testing.T, actual error, target error, msgAndArgs ...interface{})
				// 意味: 実際に関するから返却されたエラー、想定されるエラーとを比較し、想定と異なる場合はエラーメッセージを出力
				// 引数:
				// 　t: テストオブジェクト。
				// 　actual: 実際に発生したエラー。
				// 　target: 期待されるエラーの種類（エラー変数）。
				// 　msgAndArgs: 失敗時（エラーの種類が違う場合）のカスタムメッセージ（省略可能）。
				// 例: assert.ErrorIs(t, err, sql.ErrNoRows, "取得エラーが発生するはずが、違う種類のエラー(%v)でした", err)
				assert.ErrorIs(t, err, tt.wantErr, "Expected error '%v', but got '%v'", tt.wantErr, err)
				// assert.EqualError(t, err, tt.wantErr.Error()) // エラーメッセージ文字列で比較する場合
				// assert.Nil(t, tenant, msgAndArgs...)
				// 書式: assert.Nil(t *testing.T, object interface{}, msgAndArgs ...interface{})
				// 意味: object が nil であること（ポインタ、インターフェース、マップ、スライス、チャネルなどが nil であること）を表明します。
				// 引数:
				// 　t: テストオブジェクト。
				// 　object: nil かどうかチェックしたい変数。
				// 　msgAndArgs: 失敗時（object が nil でなかった場合）のカスタムメッセージ（省略可能）。
				// 例: assert.Nil(t, result, "エラー発生時は結果がnilであるべきです")
				// assert.Nil(t, tenant, "Tenant should be nil on error")
				// tenantがnilであればOK。nilでなければエラーメッセージを出力
			} else {
				// エラーが期待されない場合
				require.NoError(t, err, "Did not expect an error but got: %v", err)
				// エラーが無いことを必須とする。エラーがあれば、テストがおかしいとして中断する。
				// require.NotNil(t, tenant, "Expected a tenant but got nil")
				//　tenantがnilデないことを確認。
				// 取得したテナントが期待通りか比較
				// assert.Equal(t, tt.wantTenant, tenant, "Returned tenant does not match expected tenant")
			}

			// --- モックの検証 ---
			// 設定したモックメソッドが期待通りに呼び出されたか検証
			mockTenantRepo.AssertExpectations(t)
		})
	}
}
