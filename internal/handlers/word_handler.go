// internal/handlers/word_handler.go
package handlers

import (
	"log"
	"net/http"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/service"
	"go_4_vocab_keep/internal/webutil"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WordHandler struct {
	service service.WordService
}

func NewWordHandler(s service.WordService) *WordHandler {
	return &WordHandler{service: s}
}

func (h *WordHandler) CreateWord(w http.ResponseWriter, r *http.Request) {
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req model.CreateWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	// 簡単なバリデーション
	if req.Term == "" || req.Definition == "" {
		webutil.RespondWithError(w, http.StatusBadRequest, "Term and definition are required")
		return
	}

	word, err := h.service.CreateWord(r.Context(), tenantID, &req)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		log.Printf("Error creating word: %v (status: %d)", err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error())
		return
	}

	webutil.RespondWithJSON(w, http.StatusCreated, word)
}

func (h *WordHandler) ListWords(w http.ResponseWriter, r *http.Request) {
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	words, err := h.service.ListWords(r.Context(), tenantID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err) // 通常は500のはず
		log.Printf("Error listing words: %v (status: %d)", err, statusCode)
		webutil.RespondWithError(w, statusCode, "Failed to list words") // エラーメッセージを汎用化
		return
	}

	if words == nil {
		words = []*model.Word{}
	}
	webutil.RespondWithJSON(w, http.StatusOK, words)
}

func (h *WordHandler) GetWord(w http.ResponseWriter, r *http.Request) {
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}

	word, err := h.service.GetWord(r.Context(), tenantID, wordID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		log.Printf("Error getting word %s: %v (status: %d)", wordIDStr, err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error())
		return
	}

	webutil.RespondWithJSON(w, http.StatusOK, word)
}

// UpdateWord は特定の単語情報を更新するためのHTTPリクエストハンドラ関数です。
// URLパスパラメータから更新対象の単語IDを取得し、
// リクエストボディから更新内容（単語の綴りや定義）を受け取り、
// サービス層を通じて単語データを更新します。
//
// 引数:
//
//	w http.ResponseWriter: クライアントへのレスポンスを書き込むためのインターフェース。
//	                       これを使ってHTTPステータスコードやレスポンスボディを設定します。
//	r *http.Request:       クライアントからのHTTPリクエスト情報（メソッド、URL、ヘッダー、ボディなど）を持つ構造体へのポインタ。
//
// 戻り値:
//
//	なし (この関数は直接値を返さず、w を通じてレスポンスを送信します)
func (h *WordHandler) UpdateWord(w http.ResponseWriter, r *http.Request) {
	// --- 1. 認証情報の取得 ---
	// リクエストのコンテキストからテナントID（どのユーザー/グループのリクエストか）を取得します。
	// middleware.GetTenantIDFromContext は、ミドルウェアで事前に設定されたテナントIDをコンテキストから取り出す関数です。
	// 戻り値:
	//   tenantID (uuid.UUID): 取得したテナントID
	//   err (error):          取得中にエラーが発生した場合にエラー情報、成功時は nil
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		// テナントIDが取得できない場合（例: 認証されていない）、401 Unauthorized エラーを返す。
		// webutil.RespondWithError は、指定されたステータスコードとエラーメッセージでJSON形式のエラーレスポンスを作成し、w に書き込みます。
		webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return // エラーが発生したので処理を中断
	}

	// --- 2. 更新対象の単語IDの取得 ---
	// URLパスパラメータから "word_id" の値を取得します。
	// 例: /words/123e4567-e89b-12d3-a456-426614174000 の場合、"123e..." の部分を取得。
	// chi.URLParam は chi ルーターの機能で、URLパスから指定した名前のパラメータを文字列として取得します。
	// 戻り値:
	//   wordIDStr (string): URLから取得した word_id の文字列
	wordIDStr := chi.URLParam(r, "word_id")
	// 取得した文字列のIDを UUID 型に変換（パース）します。
	// uuid.Parse は、文字列が正しいUUID形式か検証し、UUID型に変換します。
	// 戻り値:
	//   wordID (uuid.UUID): パースされたUUID
	//   err (error):       文字列が有効なUUID形式でない場合にエラー情報、成功時は nil
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		// IDの形式が無効な場合、400 Bad Request エラーを返す。
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return // エラーが発生したので処理を中断
	}

	// --- 3. リクエストボディの解析とバリデーション ---
	// リクエストボディ（クライアントから送られてきた更新内容）をJSON形式としてデコードし、
	// model.UpdateWordRequest 構造体にマッピングします。
	var req model.UpdateWordRequest // 更新リクエストの内容を格納するための構造体変数
	// webutil.DecodeJSONBody は、リクエスト(r)のボディを読み取り、JSONとして解釈し、指定された構造体ポインタ(&req)に値を格納するヘルパー関数です。
	// 戻り値:
	//   err (error): デコード中にエラー（形式が違う、読み取れない等）が発生した場合にエラー情報、成功時は nil
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		// JSONのデコードに失敗した場合、400 Bad Request エラーを返す。
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return // エラーが発生したので処理を中断
	}

	// 更新内容の基本的なバリデーション（検証）
	// 更新するフィールド (Term: 単語の綴り, Definition: 定義) がどちらも指定されていない場合はエラーとする。
	// if req.Term == nil && req.Definition == nil {
	// 	// 更新する内容がない場合、400 Bad Request エラーを返す。
	// 	webutil.RespondWithError(w, http.StatusBadRequest, "No fields provided for update")
	// 	return // エラーが発生したので処理を中断
	// }
	// TODO: ここでさらに詳細な入力値バリデーションを行うことが推奨されます。
	// 例: req.Term が空文字列でないか、文字数制限は守られているかなど。

	// --- 4. サービス層の呼び出し（実際の更新処理） ---
	// 準備が整ったので、サービス層の UpdateWord メソッドを呼び出して単語の更新を依頼します。
	// h.service.UpdateWord は、ビジネスロジック（データベース操作など）を担当するメソッドです。
	// 引数:
	//   r.Context(): リクエストのコンテキスト（タイムアウトやキャンセル情報を伝搬させるため）
	//   tenantID:    どのテナントのデータか識別するため
	//   wordID:      どの単語を更新するか識別するため
	//   &req:        どのように更新するか（更新内容）
	// 戻り値:
	//   word (*model.Word): 更新後の単語データへのポインタ
	//   err (error):        更新処理中にエラーが発生した場合にエラー情報、成功時は nil
	word, err := h.service.UpdateWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		// --- 5. エラーハンドリング (サービス層でエラーが発生した場合) ---
		// サービス層での処理中に何らかのエラーが発生した場合の処理。
		// webutil.MapErrorToStatusCode は、発生したエラーの種類に応じて適切なHTTPステータスコードを決定するヘルパー関数です。
		// (例: データが見つからない場合は 404 Not Found, データベースエラーなら 500 Internal Server Error など)
		statusCode := webutil.MapErrorToStatusCode(err)
		// エラー内容をサーバーのログに出力します。エラー追跡に役立ちます。
		// log.Printf は、フォーマット指定でログメッセージを出力する標準ライブラリの関数です。
		log.Printf("Error updating word %s for tenant %s: %v (status: %d)", wordIDStr, tenantID, err, statusCode)
		// クライアントにエラーレスポンスを返します。エラーメッセージにはサービス層から返されたエラー内容を含めます。
		webutil.RespondWithError(w, statusCode, "Failed to update word: "+err.Error())
		return // エラーが発生したので処理を中断
	}

	// --- 6. 成功レスポンスの送信 ---
	// 更新処理が成功した場合。
	// webutil.RespondWithJSON は、指定されたステータスコード(200 OK)とデータをJSON形式でレスポンスとして作成し、w に書き込みます。
	// ここでは、更新後の単語データ (word) をクライアントに返します。
	webutil.RespondWithJSON(w, http.StatusOK, word)
}

func (h *WordHandler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}

	err = h.service.DeleteWord(r.Context(), tenantID, wordID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		log.Printf("Error deleting word %s: %v (status: %d)", wordIDStr, err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
