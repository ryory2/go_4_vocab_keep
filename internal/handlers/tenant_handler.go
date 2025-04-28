// Package handlers は、HTTPリクエストを処理するハンドラ関数を定義するパッケージです。
// Webフレームワーク (ここでは chi を想定) から呼び出され、
// リクエストの解析、ビジネスロジック (Service) の呼び出し、レスポンスの生成を行います。
package handlers

import (
	// "log" は、ログメッセージを出力するためのGo標準パッケージです。
	"errors"
	"log"
	"log/slog"

	// "net/http" は、HTTPクライアントとサーバーの実装を含むGo標準パッケージです。
	// Webサーバーのハンドラ関数でリクエスト(r)とレスポンス(w)を扱うために使います。
	"net/http"

	// "go_4_vocab_keep/internal/service" は、ビジネスロジックを担当するサービス層のパッケージです。
	// TenantHandler はこのパッケージの TenantService を利用します。
	"go_4_vocab_keep/internal/service" // プロジェクト名修正
	// "go_4_vocab_keep/internal/webutil" は、Web関連のユーティリティ関数 (JSON処理、レスポンス生成など) を
	// 提供する、このプロジェクト固有のパッケージです。
	"go_4_vocab_keep/internal/webutil" // プロジェクト名修正

	"github.com/go-playground/validator/v10" // validator パッケージをインポート
)

// --- バリデータインスタンスの準備 ---
// validate は validator のインスタンスです。
// 通常、アプリケーション全体で一つだけ生成し、使い回します。
// パッケージレベルの変数として定義するのが一般的です。
var validate *validator.Validate

// パッケージ初期化関数: main より先に一度だけ実行される
func init() {
	validate = validator.New()
	log.Println("Validator initialized.")
	// ここでカスタムバリデーションなどを登録することも可能
	// 例: validate.RegisterValidation("my_custom_tag", myCustomValidationFunc)
}

// --- 構造体定義 ---

/**
 * @struct TenantHandler
 * @brief テナント関連のHTTPリクエストを処理するハンドラです。
 *
 * この構造体は、テナント作成などのAPIエンドポイントに対応するメソッドを持ちます。
 * 実際のビジネスロジックは、保持している `service.TenantService` に委譲します。
 * Java でいう Controller クラスに近い役割を持ちます。
 */

//	type TenantHandler struct { ... }: これはGo言語で「構造体 (struct)」を定義するための構文です。

type TenantHandler struct {
	/**
	 * @field service
	 * @brief テナント関連のビジネスロジックを実行するサービスインスタンスです。
	 *        ハンドラは直接データベースなどを操作せず、このサービスを通じて処理を行います。
	 */
	//
	service service.TenantService
	logger  *slog.Logger
}

// @structは構造体の定義を示す
// @briefは詳細な説明
// @fieldはフィールドの説明を示す
// @paramは関数の引数の説明を示す
// @returnは関数の戻り値の説明を示す
// @exampleは使用例を示す
// @tagはフィールドに付加するタグを示す
// @receiverはメソッドのレシーバを示す
// @functionは関数の定義を示す
// @methodはメソッドの定義を示す
// @exampleは使用例を示す

/**
 * @function NewTenantHandler
 * @brief TenantHandler の新しいインスタンス（ポインタ）を作成します (コンストラクタ関数)。
 *
 * Goでは `NewXxx` という形式の関数で構造体のインスタンスを生成するのが一般的です。
 * 依存する `service.TenantService` を引数として受け取り、
 * それを内部フィールドに保持した `TenantHandler` のポインタ (`*TenantHandler`) を返します。
 * このように外部から依存性を注入する手法を「依存性の注入 (Dependency Injection, DI)」と呼びます。
 *
 * @param s service.TenantService: 依存するテナントサービスの実装。
 * @return *TenantHandler: 新しく作成された TenantHandler のポインタ。
 *
 * @example
 * tenantService := service.NewTenantService(...) // サービス層の準備
 * tenantHandler := handlers.NewTenantHandler(tenantService) // ハンドラの作成
 */
func NewTenantHandler(s service.TenantService, logger *slog.Logger) *TenantHandler {
	// &TenantHandler{service: s} は、TenantHandler構造体の新しいインスタンスを作成し、
	// その service フィールドに引数 s を設定し、
	// そのインスタンスへのメモリアドレス（ポインタ）を返します。
	return &TenantHandler{
		service: s,
		logger:  logger,
	}
}

/**
 * @struct CreateTenantRequest
 * @brief テナント作成API (POST /tenants) へのリクエストボディの構造を表すデータ転送オブジェクト (DTO: Data Transfer Object) です。
 *        JSON形式のリクエストボディをGoの構造体にマッピングするために使用されます。
 */
type CreateTenantRequest struct {
	/**
	 * @field Name
	 * @brief 作成するテナントの名前。
	 * @tag json:"name" - JSONキーのマッピング。
	 * @tag validate:"required,min=1,max=100" - バリデーションルール。
	 *        - required: 必須項目。
	 *        - min=1: 最低1文字。
	 *        - max=100: 最大100文字。
	 *        (ルールは必要に応じて調整してください)
	 */
	Name string `json:"name" validate:"required,min=1,max=100"` // バリデーションタグを修正・追加
}

// --- メソッド定義 ---

/**
 * @method CreateTenant
 * @brief HTTP POSTリクエストを受け取り、新しいテナントを作成します。 (ハンドラ関数)
 *        対応するエンドポイントは通常 `POST /api/v1/tenants` のようなパスになります。
 *
 * このメソッドは `TenantHandler` 型に紐付けられています (レシーバ `h *TenantHandler` があるため)。
 * Goではこのように構造体にメソッドを定義します。
 *
 * @receiver h *TenantHandler: このメソッドを呼び出す TenantHandler インスタンスへのポインタ。
 *                              メソッド内で `h.service` のようにフィールドにアクセスできます。
 * @param w http.ResponseWriter: HTTPレスポンスを書き込むためのインターフェース。
 *                               ステータスコードの設定やレスポンスボディの書き込みに使います。
 * @param r *http.Request:       受信したHTTPリクエストの情報を持つ構造体へのポインタ。
 *                               リクエストヘッダー、ボディ、URLパラメータなどを取得できます。
 *
 * @example (Webフレームワーク chi でのルーティング設定例)
 * r.Post("/tenants", tenantHandler.CreateTenant)
 */
func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	logger := h.logger
	// --- リクエストボディのデコード ---
	// `var req CreateTenantRequest` で、リクエストボディのデータを格納するための
	// CreateTenantRequest型の変数 `req` を宣言します。
	var req CreateTenantRequest

	// `webutil.DecodeJSONBody(r, &req)` を呼び出します。
	// - `r`: HTTPリクエスト情報 (ここからリクエストボディを読み取る)
	// - `&req`: デコードしたJSONデータを格納する変数 `req` のメモリアドレス (ポインタ)
	// この関数は、リクエストボディを読み取り、JSONとして解釈し、
	// その結果を `req` 変数に書き込みます。
	// Goでは、関数が処理の成否を示すためにエラー (`error` 型) を返すのが一般的です。
	// `if err := ...; err != nil` は、関数呼び出しとそのエラーチェックを簡潔に書くためのGoの慣用句です。
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		// デコードに失敗した場合 (例: JSON形式が不正、データ型が違うなど)。

		// `webutil.RespondWithError` を呼び出して、クライアントにエラーレスポンスを返します。
		// - `w`: レスポンスを書き込む対象。
		// - `http.StatusBadRequest`: HTTPステータスコード 400 (Bad Request) を示す定数。
		// - `"Invalid request body: " + err.Error()`: エラーメッセージ。`err.Error()` でエラーの詳細を取得。
		logger.Warn("Failed to decode request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		// `return` でこの関数の処理を中断します。これ以降の処理は実行されません。
		return
	}

	// --- バリデーション (go-playground/validator を使用) ---
	// validate.Struct(req) を呼び出して、req インスタンスのタグに基づきバリデーションを実行
	err := validate.Struct(req)
	if err != nil {
		// バリデーションエラーが発生した場合
		var validationErrors validator.ValidationErrors
		// エラーがバリデーションエラーの型かを確認 (errors.As を使うのが推奨)
		if errors.As(err, &validationErrors) {
			logger.Warn(
				"Validation failed for CreateTenant request",
				slog.Any("validation_errors", validationErrors.Error()),
				slog.Any("request_body", req),
			)
			errorMsg := "Validation failed: " + validationErrors.Error()
			webutil.RespondWithError(w, http.StatusBadRequest, errorMsg)
		} else {
			logger.Error(
				"Unexpected error during validation",
				slog.Any("error", err),
				slog.Any("request_body", req),
			)
			webutil.RespondWithError(w, http.StatusInternalServerError, "Error validating request") // 500 Internal Server Error
		}
		return // 処理を中断
	}
	// --- バリデーションここまで ---

	// バリデーションが成功した場合のみ、以下の処理に進む

	// (削除) 以前の手動バリデーションは不要になる
	// if req.Name == "" { ... }

	// --- ビジネスロジックの呼び出し (Service層) ---
	// `h.service.CreateTenant` メソッドを呼び出して、実際にテナントを作成する処理を依頼します。
	// - `r.Context()`: リクエストに関連付けられたコンテキスト(Context)を取得します。
	//   コンテキストは、リクエストのタイムアウトやキャンセル、リクエスト固有の値 (認証情報など) を
	//   伝搬させるために使われます。サービス層やリポジトリ層まで引き回されることが多いです。
	// - `req.Name`: リクエストから取得したテナント名。
	// このメソッドは、作成されたテナント情報 (`*model.Tenant` 型などを想定) と、
	// 処理中に発生したエラー (`error` 型) を返します。
	tenant, err := h.service.CreateTenant(r.Context(), req.Name)
	// ここでも `err != nil` でエラーが発生したかどうかをチェックします。
	if err != nil {
		// --- エラーレスポンスの生成 ---
		// `webutil.MapErrorToStatusCode(err)` を呼び出して、発生したエラーの種類に応じて
		// 適切なHTTPステータスコード (例: 既に存在するなら 409 Conflict) を決定します。
		statusCode := webutil.MapErrorToStatusCode(err) // エラーに応じたステータスコード取得

		logger.Error(
			"Failed to create tenant in service",
			slog.Any("error", err), // エラーオブジェクト全体 [8, 17]
			slog.Int("status_code", statusCode),
			slog.String("requested_name", req.Name), // どのリクエストで失敗したかのコンテキスト [6, 14]
		)

		// `webutil.RespondWithError` で、決定したステータスコードとエラーメッセージをクライアントに返します。
		webutil.RespondWithError(w, statusCode, err.Error()) // エラーメッセージを返す
		// 処理を中断します。
		return
	}

	// --- 正常レスポンスの生成 ---
	// エラーが発生しなかった場合 (テナント作成成功)。
	// `webutil.RespondWithJSON` を呼び出して、クライアントに成功レスポンスを返します。
	// - `w`: レスポンスを書き込む対象。
	// - `http.StatusCreated`: HTTPステータスコード 201 (Created) を示す定数。リソース作成成功時に使われます。
	// - `tenant`: サービス層から返された、作成されたテナントの情報。
	// この関数は、`tenant` オブジェクトをJSON形式に変換し、適切なヘッダーと共にレスポンスボディとして書き込みます。
	webutil.RespondWithJSON(w, http.StatusCreated, tenant) // 作成されたテナント情報を返す
} // CreateTenant メソッドの終わり
