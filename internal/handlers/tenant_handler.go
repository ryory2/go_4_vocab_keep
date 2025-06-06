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
	"go_4_vocab_keep/internal/model"
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
 */
func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "CreateTenant")) // ログにコンテキスト追加

	// --- リクエストボディのデコード ---
	var req CreateTenantRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", slog.String("error", err.Error()))
		// AppErrorを生成して一元的なエラーハンドラに渡す
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	// --- バリデーション ---
	if err := validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			// バリデーションエラーの場合、webutilのヘルパーでAppErrorを生成
			logger.Warn("Validation failed for request", slog.Any("errors", validationErrors.Error()), slog.Any("request", req))
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			// バリデーションライブラリ自体に予期せぬエラーが発生した場合
			logger.Error("Unexpected error during validation", slog.Any("error", err), slog.Any("request", req))
			webutil.HandleError(w, logger, err) // 予期せぬエラーとして処理
		}
		return
	}

	// --- ビジネスロジックの呼び出し (Service層) ---
	tenant, err := h.service.CreateTenant(r.Context(), req.Name)
	if err != nil {
		// サービス層で発生したエラーをログに記録し、一元的なハンドラに渡す
		logger.Error(
			"Failed to create tenant in service",
			slog.Any("error", err),
			slog.String("requested_name", req.Name),
		)
		webutil.HandleError(w, logger, err)
		return
	}

	// --- 正常レスポンスの生成 ---
	logger.Info("Tenant created successfully", slog.String("tenant_id", tenant.TenantID.String()))
	webutil.RespondWithJSON(w, http.StatusCreated, tenant, logger)
}
