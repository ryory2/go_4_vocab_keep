// internal/handlers/word_handler.go
package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"go_4_vocab_keep/internal/middleware"
	"go_4_vocab_keep/internal/model"
	"go_4_vocab_keep/internal/service"
	"go_4_vocab_keep/internal/webutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type WordHandler struct {
	service service.WordService
	logger  *slog.Logger
}

func NewWordHandler(s service.WordService, logger *slog.Logger) *WordHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WordHandler{
		service: s,
		logger:  logger,
	}
}

// PostWord は新しい単語リソースを作成するためのハンドラ
func (h *WordHandler) PostWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PostWord"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	var req model.PostWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	// --- ★ここを validator を使ったバリデーションに置き換え ---
	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		// エラーがバリデーションエラーか判定
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed", slog.Any("errors", validationErrors.Error()), slog.Any("request", req))

			// 最初のエラーを代表としてクライアントに返す
			firstErr := validationErrors[0]
			// 日本語メッセージに翻訳
			translatedMsg := firstErr.Translate(webutil.Trans)

			// 詳細なエラー情報を AppError として生成
			appErr := model.NewAppError(
				"VALIDATION_ERROR",
				translatedMsg,
				firstErr.Field(), // エラーが発生したフィールド (jsonタグ名)
				model.ErrInvalidInput,
			)
			webutil.HandleError(w, logger, appErr)
		} else {
			// バリデーションライブラリ自体のエラーなど、予期せぬエラー
			logger.Error("Unexpected error during validation", slog.Any("error", err))
			webutil.HandleError(w, logger, err)
		}
		return
	}

	word, err := h.service.PostWord(r.Context(), tenantID, &req)
	if err != nil {
		logger.Error("Error posting word in service", slog.Any("error", err), slog.Any("request", req))
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word posted successfully", slog.String("word_id", word.WordID.String()))
	webutil.RespondWithJSON(w, http.StatusCreated, word, logger)
}

// GetWords は単語リソースの一覧を取得するためのハンドラ
func (h *WordHandler) GetWords(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "GetWords"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	words, err := h.service.GetWords(r.Context(), tenantID)
	if err != nil {
		logger.Error("Error listing words in service", slog.Any("error", err))
		webutil.HandleError(w, logger, err)
		return
	}

	if words == nil {
		words = []*model.Word{}
	}
	logger.Info("Words listed successfully", slog.Int("count", len(words)))
	webutil.RespondWithJSON(w, http.StatusOK, words, logger)
}

// GetWord は特定の単語リソースを取得するためのハンドラ
func (h *WordHandler) GetWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "GetWord"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	word, err := h.service.GetWord(r.Context(), tenantID, wordID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Info("Word not found in service", slog.Any("error", err))
		} else {
			logger.Error("Error getting word from service", slog.Any("error", err))
		}
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word retrieved successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// PutWord は特定の単語リソースを完全に置き換えるためのハンドラ
func (h *WordHandler) PutWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PutWord"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for PutWord", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for PutWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	var req model.PutWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode PutWord request body", slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	word, err := h.service.PutWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		logger.Error("Error putting word in service", slog.Any("error", err), slog.Any("request", req))
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word put successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// PatchWord は特定の単語リソースの一部を更新するためのハンドラ
func (h *WordHandler) PatchWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "PatchWord"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for PatchWord", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for PatchWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	var req model.PatchWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode PatchWord request body", slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if req.Term == nil && req.Definition == nil {
		logger.Warn("PatchWord called with no fields provided for update", slog.Any("request", req))
		appErr := model.NewAppError("VALIDATION_ERROR", "更新するフィールドが指定されていません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	word, err := h.service.PatchWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		logger.Error("Error patching word in service", slog.Any("error", err), slog.Any("request", req))
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word patched successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// DeleteWord は特定の単語リソースを削除するためのハンドラ
func (h *WordHandler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "DeleteWord"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for DeleteWord", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for DeleteWord", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	err = h.service.DeleteWord(r.Context(), tenantID, wordID)
	if err != nil {
		logger.Error("Error deleting word in service", slog.Any("error", err))
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word deleted successfully (or was already deleted)")
	w.WriteHeader(http.StatusNoContent)
}
