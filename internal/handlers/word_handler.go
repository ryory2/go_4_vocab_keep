// internal/handlers/word_handler.go
package handlers

import (
	"errors"   // errors パッケージをインポート
	"log/slog" // slog パッケージをインポート
	"net/http"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/service"
	"go_4_vocab_keep/internal/webutil"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// WordHandler 構造体に logger フィールドを追加
type WordHandler struct {
	service service.WordService
	logger  *slog.Logger // slog.Logger フィールドを追加
}

// NewWordHandler コンストラクタで logger を受け取るように変更
func NewWordHandler(s service.WordService, logger *slog.Logger) *WordHandler { // logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &WordHandler{
		service: s,
		logger:  logger, // logger を設定
	}
}

func (h *WordHandler) CreateWord(w http.ResponseWriter, r *http.Request) {
	// リクエスト固有の情報をロガーに追加することも検討 (例: リクエストID)
	logger := h.logger

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		// slog で警告ログ (認証失敗)
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	// テナントIDを以降のログに追加
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	var req model.CreateWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		// slog で警告ログ (クライアントリクエストエラー)
		logger.Warn("Failed to decode CreateWord request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	// 簡単なバリデーション
	if req.Term == "" || req.Definition == "" {
		// slog で警告ログ (クライアント入力エラー)
		logger.Warn("Validation failed: Term and definition are required", slog.Any("request", req))
		webutil.RespondWithError(w, http.StatusBadRequest, "Term and definition are required")
		return
	}

	word, err := h.service.CreateWord(r.Context(), tenantID, &req)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		// slog でエラーログ (サービス層エラー)
		logger.Error("Error creating word in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
			slog.Any("request", req), // 注意: リクエスト内容に機密情報がないか確認
		)
		// クライアントにはサービス層が返したエラーメッセージをそのまま返すか、
		// または汎用的なメッセージにするかを検討
		webutil.RespondWithError(w, statusCode, "Failed to create word") // 汎用メッセージ例
		return
	}

	// slog で成功ログ
	logger.Info("Word created successfully", slog.String("word_id", word.WordID.String()))
	webutil.RespondWithJSON(w, http.StatusCreated, word)
}

func (h *WordHandler) ListWords(w http.ResponseWriter, r *http.Request) {
	logger := h.logger
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	words, err := h.service.ListWords(r.Context(), tenantID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err) // 通常は500のはず
		// slog でエラーログ (サービス層エラー)
		logger.Error("Error listing words in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
		)
		webutil.RespondWithError(w, statusCode, "Failed to list words") // 汎用メッセージ
		return
	}

	if words == nil {
		words = []*model.Word{} // 空のスライスを返す
	}
	// slog で成功ログ (任意)
	logger.Info("Words listed successfully", slog.Int("count", len(words)))
	webutil.RespondWithJSON(w, http.StatusOK, words)
}

func (h *WordHandler) GetWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		// slog で警告ログ (クライアントリクエストエラー)
		logger.Warn("Invalid word ID format in URL", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	word, err := h.service.GetWord(r.Context(), tenantID, wordID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		// slog でエラーまたは情報ログ (サービス層の結果による)
		if errors.Is(err, model.ErrNotFound) {
			logger.Info("Word not found in service", slog.Int("status_code", statusCode))
		} else {
			logger.Error("Error getting word from service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
			)
		}
		webutil.RespondWithError(w, statusCode, "Failed to get word") // 汎用メッセージ
		return
	}

	// slog で成功ログ (任意)
	logger.Info("Word retrieved successfully")
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
	logger := h.logger // ハンドラ固有のロガーを取得

	// --- 1. 認証情報の取得 ---
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for UpdateWord", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String())) // テナントIDをログコンテキストに追加

	// --- 2. 更新対象の単語IDの取得 ---
	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for UpdateWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String())) // 単語IDをログコンテキストに追加

	// --- 3. リクエストボディの解析とバリデーション ---
	var req model.UpdateWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode UpdateWord request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// 更新内容の基本的なバリデーション（サービス層でも行うが、ハンドラ層でも早期リターン）
	// if req.Term == nil && req.Definition == nil {
	//     logger.Warn("UpdateWord called with no fields provided for update", slog.Any("request", req))
	// 	   webutil.RespondWithError(w, http.StatusBadRequest, "No fields provided for update")
	// 	   return
	// }
	// TODO: 詳細なバリデーション

	// --- 4. サービス層の呼び出し（実際の更新処理） ---
	word, err := h.service.UpdateWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		// --- 5. エラーハンドリング (サービス層でエラーが発生した場合) ---
		statusCode := webutil.MapErrorToStatusCode(err)
		// slog でエラーまたは情報ログ (エラー種別による)
		// ★ 修正点: logAttrs スライスを使わず、直接 slog.Attr を渡す
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) {
			logger.Info("UpdateWord service returned expected error",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req), // 注意: 機密情報がないか確認
			)
		} else {
			logger.Error("Error updating word in service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req), // 注意: 機密情報がないか確認
			)
		}
		webutil.RespondWithError(w, statusCode, "Failed to update word") // 汎用メッセージ
		return
	}

	// --- 6. 成功レスポンスの送信 ---
	logger.Info("Word updated successfully") // 更新後の単語IDは word.WordID で取得可能
	webutil.RespondWithJSON(w, http.StatusOK, word)
}

func (h *WordHandler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for DeleteWord", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for DeleteWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	err = h.service.DeleteWord(r.Context(), tenantID, wordID)
	if err != nil {
		// DeleteWord サービスは ErrNotFound の場合 nil を返す想定 (冪等性のため)
		// それ以外のエラーの場合
		statusCode := webutil.MapErrorToStatusCode(err) // 通常は 500
		logger.Error("Error deleting word in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
		)
		webutil.RespondWithError(w, statusCode, "Failed to delete word") // 汎用メッセージ
		return
	}

	// slog で成功ログ
	logger.Info("Word deleted successfully (or was already deleted)")
	w.WriteHeader(http.StatusNoContent) // 204 No Content
}
