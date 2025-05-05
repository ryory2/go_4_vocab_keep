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

// PostWord は新しい単語リソースを作成するためのハンドラ
func (h *WordHandler) PostWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PostWord")) // ハンドラ名をログコンテキストに追加

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		// slog で警告ログ (認証失敗)
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	// テナントIDを以降のログに追加
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	var req model.PostWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		// slog で警告ログ (クライアントリクエストエラー)
		logger.Warn("Failed to decode PostWord request body", slog.String("error", err.Error()))
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

	// サービス層のメソッド呼び出しも変更 (PostWord -> PostWord を想定)
	// ※もしサービス層が PostWord のままなら、ここは h.service.PostWord(...) にする
	word, err := h.service.PostWord(r.Context(), tenantID, &req)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		logger.Error("Error posting word in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
			slog.Any("request", req),
		)
		webutil.RespondWithError(w, statusCode, "Failed to create word") // サービス層が返したエラーメッセージを返すように修正
		return
	}

	logger.Info("Word posted successfully", slog.String("word_id", word.WordID.String())) // ログメッセージ変更
	webutil.RespondWithJSON(w, http.StatusCreated, word)
}

// GetWords は単語リソースの一覧を取得するためのハンドラ
func (h *WordHandler) GetWords(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "GetWords"))
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	words, err := h.service.GetWords(r.Context(), tenantID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		logger.Error("Error listing words in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
		)
		webutil.RespondWithError(w, statusCode, "Failed to list words")
		return
	}

	if words == nil {
		words = []*model.Word{}
	}
	logger.Info("Words listed successfully", slog.Int("count", len(words)))
	webutil.RespondWithJSON(w, http.StatusOK, words)
}

// GetWord は特定の単語リソースを取得するためのハンドラ
func (h *WordHandler) GetWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "GetWord"))
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
		logger.Warn("Invalid word ID format in URL", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	word, err := h.service.GetWord(r.Context(), tenantID, wordID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		if errors.Is(err, model.ErrNotFound) {
			logger.Info("Word not found in service", slog.Int("status_code", statusCode))
		} else {
			logger.Error("Error getting word from service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
			)
		}
		webutil.RespondWithError(w, statusCode, "Failed to get word")
		return
	}

	logger.Info("Word retrieved successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word)
}

// PutWord は特定の単語リソースを完全に置き換える (全部更新) ためのハンドラ
// URLパスパラメータから更新対象の単語IDを取得し、
// リクエストボディから更新後の内容すべての項目を受け取り、単語データを更
func (h *WordHandler) PutWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PutWord"))

	// --- 1. 認証情報の取得 ---
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for PutWord", slog.String("error", err.Error())) // ★ログメッセージ更新
		webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	// --- 2. 更新対象の単語IDの取得 ---
	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for PutWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error())) // ★ログメッセージ更新
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	// --- 3. リクエストボディの解析とバリデーション ---
	var req model.PutWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode PutWord request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// TODO:リクエストパラメータすべて必須のため、バリデーションライブラリ等でチェック

	// --- 4. サービス層の呼び出し（実際の更新処理） ---
	word, err := h.service.PutWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		// --- 5. エラーハンドリング (サービス層でエラーが発生した場合) ---
		statusCode := webutil.MapErrorToStatusCode(err)
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) {
			logger.Info("PutWord service returned expected error",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req),
			)
		} else {
			logger.Error("Error putting word in service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req),
			)
		}
		webutil.RespondWithError(w, statusCode, "Failed to update word")
		return
	}

	// --- 6. 成功レスポンスの送信 ---
	logger.Info("Word put successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word)
}

// PatchWord は特定の単語リソースの一部を更新 (部分更新) するためのハンドラ
// URLパスパラメータから更新対象の単語IDを取得し、
// リクエストボディから更新したいフィールドとその値を受け取り、
// サービス層を通じて単語データの一部を更新します。
func (h *WordHandler) PatchWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PatchWord")) // ハンドラ名をログコンテキストに追加

	// --- 1. 認証情報の取得 ---
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for PatchWord", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	// --- 2. 更新対象の単語IDの取得 ---
	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for PatchWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	// --- 3. リクエストボディの解析とバリデーション ---
	var req model.PatchWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode PatchWord request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// PATCHの場合、最低1つのフィールドが必要か等のバリデーション
	if req.Term == nil && req.Definition == nil {
		logger.Warn("PatchWord called with no fields provided for update", slog.Any("request", req))
		webutil.RespondWithError(w, http.StatusBadRequest, "No fields provided for update")
		return
	}

	// --- 4. サービス層の呼び出し（実際の更新処理） ---
	// ※サービス層のメソッド名は PatchWord を想定
	word, err := h.service.PatchWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		// --- 5. エラーハンドリング (サービス層でエラーが発生した場合) ---
		statusCode := webutil.MapErrorToStatusCode(err)
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) {
			logger.Info("PatchWord service returned expected error",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req),
			)
		} else {
			logger.Error("Error patching word in service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Any("request", req),
			)
		}
		webutil.RespondWithError(w, statusCode, "Failed to patch word") // メッセージ変更
		return
	}

	// --- 6. 成功レスポンスの送信 ---
	logger.Info("Word patched successfully")        // ログメッセージ変更
	webutil.RespondWithJSON(w, http.StatusOK, word) // 更新後のリソースを返す場合
}

// --- PatchWord ハンドラここまで ---

// DeleteWord は特定の単語リソースを削除するためのハンドラ (変更なし)
func (h *WordHandler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "DeleteWord"))
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
		statusCode := webutil.MapErrorToStatusCode(err)
		logger.Error("Error deleting word in service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
		)
		webutil.RespondWithError(w, statusCode, "Failed to delete word")
		return
	}

	logger.Info("Word deleted successfully (or was already deleted)")
	w.WriteHeader(http.StatusNoContent)
}
