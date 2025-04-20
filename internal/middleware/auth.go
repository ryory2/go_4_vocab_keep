// internal/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"go_4_vocab_keep/internal/model" // プロジェクト名修正
	// プロジェクト名修正
	"go_4_vocab_keep/internal/service" // serviceを追加 (Authenticatorから使う場合)
	"go_4_vocab_keep/internal/webutil" // プロジェクト名修正

	"github.com/google/uuid"
	// gormは直接使わない (service経由にする)
)

// TenantAuthenticator はテナント認証を行うためのインターフェース
type TenantAuthenticator interface {
	Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// serviceTenantAuthenticator は TenantService を使って認証を行う実装
type serviceTenantAuthenticator struct {
	tenantService service.TenantService
}

// NewServiceTenantAuthenticator は新しいオーセンティケータを作成します
func NewServiceTenantAuthenticator(ts service.TenantService) TenantAuthenticator {
	return &serviceTenantAuthenticator{tenantService: ts}
}

// Authenticate はテナントの存在と有効性を確認します
func (a *serviceTenantAuthenticator) Authenticate(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	_, err := a.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil // 見つからない場合は認証失敗 (エラーではない)
		}
		// その他のエラー (DBエラーなど)
		log.Printf("Error during tenant authentication check for %s: %v", tenantID, err)
		return false, err
	}
	// エラーなく取得できれば有効なテナント
	return true, nil
}

// TenantAuthMiddleware はテナント認証を行うミドルウェアを作成します
func TenantAuthMiddleware(auth TenantAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantIDStr := r.Header.Get("X-Tenant-ID")
			if tenantIDStr == "" {
				log.Println("Authentication failed: X-Tenant-ID header missing")
				webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: Missing X-Tenant-ID header")
				return
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				log.Printf("Authentication failed: Invalid X-Tenant-ID format: %s", tenantIDStr)
				webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: Invalid X-Tenant-ID format")
				return
			}

			// --- テナントの存在と有効性をチェック ---
			isValid, err := auth.Authenticate(r.Context(), tenantID)
			if err != nil {
				// 認証処理中のエラー (DBエラーなど)
				webutil.RespondWithError(w, http.StatusInternalServerError, "Authentication check failed")
				return
			}
			if !isValid {
				log.Printf("Forbidden: Tenant ID %s not found or inactive", tenantIDStr)
				webutil.RespondWithError(w, http.StatusForbidden, "Forbidden: Invalid or inactive tenant")
				return
			}
			// --- チェック完了 ---

			// コンテキストにテナントIDをセット
			ctx := context.WithValue(r.Context(), model.TenantIDKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantIDFromContext は、渡されたコンテキスト(ctx)からテナントIDを取得する関数です。
// 成功した場合はテナントID(uuid.UUID)とnilを、
// 失敗した場合は空のUUID(uuid.Nil)とエラー情報を返します。

// --- 関数の定義 ---
// 書式: func 関数名(引数名1 引数型1, 引数名2 引数型2, ...) (戻り値型1, 戻り値型2, ...) { ... }
// ここでは、GetTenantIDFromContext という名前の関数を定義しています。
// - 引数: `ctx` という名前で `context.Context` 型の値を1つ受け取ります。
// - 戻り値: `uuid.UUID` 型の値と `error` 型の値を合計2つ返します。
func GetTenantIDFromContext(ctx context.Context) (uuid.UUID, error) {

	// --- 変数宣言とメソッド呼び出し、型アサーション ---
	// ctx.Valueメソッドを使って、コンテキストからキー(model.TenantIDKey)に対応する値を取り出します。
	// さらに、.(uuid.UUID) という型アサーションを使って、取り出した値が本当にuuid.UUID型かを確認します。
	//
	// 型アサーションの結果:
	// - 成功: valueに実際の値が、okにtrueが入ります。
	// - 失敗 (キーが存在しない、または型が違う): valueにはuuid.UUID型のゼロ値(uuid.Nil相当)が、okにfalseが入ります。

	// 書式（ショート変数宣言）: 変数名1, 変数名2 := 式
	//   `:=` を使うと、変数の型宣言を省略しつつ、新しい変数を宣言して右辺の結果を代入できます。
	// 書式（メソッド呼び出し）: 変数名.メソッド名(引数)
	//   `ctx` という変数が持つ `Value` というメソッド（機能）を呼び出しています。引数は `model.TenantIDKey` です。
	// 書式（型アサーション、カンマokイディオム）: interface型の値.(期待する型)
	//   `ctx.Value(...)` の結果（interface{}型）を `uuid.UUID` 型に変換しようと試みます。
	//   この書式では、変換の成否が `ok` (bool型) に、変換後の値（またはゼロ値）が `value` に入ります。
	value, ok := ctx.Value(model.TenantIDKey).(uuid.UUID)

	// --- 条件分岐 (if文) ---
	// okがfalseかどうかをチェックします。
	// falseの場合は、コンテキストに期待するテナントIDが存在しなかったか、
	// または、存在したものの期待するuuid.UUID型ではなかったことを意味します。

	// 書式: if 条件式 { ... }
	//   `!` は否定演算子で、`ok` が `false` の場合に条件式 `!ok` は `true` となります。
	//   条件式が `true` の場合、`{}` ブロック内の処理が実行されます。
	if !ok {
		// テナントIDが見つからなかった旨をログに出力します。
		// これは主に開発者がデバッグ時に問題を発見しやすくするためのものです。

		// --- パッケージ関数の呼び出し ---
		// 書式: パッケージ名.関数名(引数)
		//   `log` パッケージに含まれる `Println` という関数を呼び出しています。
		//   `Println` は引数で受け取った文字列などを標準ログに出力し、最後に改行します。
		log.Println("Error: Tenant ID not found in context")

		// --- 関数の終了と値の返却 (return文) ---
		// この関数を呼び出した箇所に、エラーが発生したことを伝えます。
		// ここでは、システム内部の予期せぬエラーというよりは、
		// 「必要な情報（テナントID）がコンテキストに設定されていなかった」
		// （例えば、認証が未完了、リクエストに必要な情報が含まれていないなど）
		// という状況を示すために、事前に定義された model.ErrTenantNotFound エラーを返します。
		// また、テナントIDとしては無効な値を示す uuid.Nil を返します。

		// 書式: return 値1, 値2, ...
		//   関数の実行をここで終了し、指定された値を呼び出し元に返します。
		//   この関数は `(uuid.UUID, error)` という2つの戻り値を返すように定義されているため、
		//   `uuid.Nil` (uuid.UUID型) と `model.ErrTenantNotFound` (error型) の2つの値を返しています。
		//   - `uuid.Nil`: `uuid` パッケージで定義されている、すべてゼロの特別なUUID値。
		//   - `model.ErrTenantNotFound`: `model` パッケージで定義されているエラー値（変数）。
		return uuid.Nil, model.ErrTenantNotFound
	} // if文の終わり

	// --- 関数の終了と値の返却 (return文) ---
	// ここに到達した場合、okはtrueであり、valueには有効なテナントID(uuid.UUID型)が格納されています。
	// 取得したテナントID(value)と、エラーが発生しなかったことを示すnilを返します。

	// 再び return 文です。今回は正常終了のパターンです。
	// 取得したテナントIDである `value` (uuid.UUID型) と、
	// Go言語で「エラーがない」ことを示す慣用的な値 `nil` (error型として扱われる) を返します。
	return value, nil
} // 関数 GetTenantIDFromContext の終わり

/*
補足: (コメントは変更なし)
- `context.Context`: リクエスト固有のデータ（ユーザーID、テナントID、トレース情報など）や、
  キャンセル信号、タイムアウトなどを伝搬させるためのGoの標準的な仕組みです。
- `model.TenantIDKey`: コンテキストにテナントIDを格納する際に使われる「キー」です。
  通常は衝突を避けるためにstring型や専用の型で定義されます (例: `type tenantIDKey string; const TenantIDKey tenantIDKey = "tenantID"`)。
  ここでは `model` パッケージで定義されていると仮定しています。
- `uuid.UUID`:  universally unique identifierの略で、世界中でほぼ一意になるように生成されるIDです。
  テナントIDのような、重複してはいけない識別子によく利用されます。
- `model.ErrTenantNotFound`: テナントが見つからなかったことを示すために、あらかじめ定義されたエラー変数です。
  このように特定のエラー状況を示す変数を用意しておくと、呼び出し側でエラーの種類に応じた処理を書きやすくなります。
- `log.Println`: 標準のログ出力関数です。通常、サーバーアプリケーションなどではより高機能なロギングライブラリが使われることも多いです。
*/
