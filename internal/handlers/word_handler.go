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
	// logger *slog.Logger // ベースロガーは不要になる
}

// NewWordHandler は WordHandler の新しいインスタンスを生成します
// コンストラクタから logger の引数を削除
func NewWordHandler(s service.WordService) *WordHandler {
	return &WordHandler{
		service: s,
	}
}

// PostWord は新しい単語リソースを作成するためのハンドラ
func (h *WordHandler) PostWord(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	var req model.PostWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed", "errors", validationErrors.Error())
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			logger.Error("Unexpected error during validation", "error", err)
			webutil.HandleError(w, logger, err)
		}
		return
	}

	word, err := h.service.PostWord(r.Context(), userID, &req)
	if err != nil {
		logger.Error("Error posting word in service", "error", err, "request", req)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word posted successfully", "word_id", word.WordID.String())
	webutil.RespondWithJSON(w, http.StatusCreated, word, logger)
}

// GetWords は単語リソースの一覧を取得するためのハンドラ
func (h *WordHandler) GetWords(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.Debug("GetWords called")

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	words, err := h.service.GetWords(r.Context(), userID)
	if err != nil {
		logger.Error("Error listing words in service", "error", err)
		webutil.HandleError(w, logger, err)
		return
	}

	if words == nil {
		words = []*model.Word{}
	}
	logger.Info("Words listed successfully", "count", len(words))
	webutil.RespondWithJSON(w, http.StatusOK, words, logger)
}

// GetWord は特定の単語リソースを取得するためのハンドラ
func (h *WordHandler) GetWord(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format", "word_id_str", wordIDStr, "error", err)
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	word, err := h.service.GetWord(r.Context(), userID, wordID)
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word retrieved successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// PutWord は特定の単語リソースを完全に置き換えるためのハンドラ
func (h *WordHandler) PutWord(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format", "word_id_str", wordIDStr, "error", err)
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	var req model.PutWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed for PutWord", "errors", validationErrors.Error())
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			logger.Error("Unexpected error during validation for PutWord", "error", err)
			webutil.HandleError(w, logger, err)
		}
		return
	}

	word, err := h.service.PutWord(r.Context(), userID, wordID, &req)
	if err != nil {
		logger.Error("Error putting word in service", "error", err, "request", req)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word put successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// PatchWord は特定の単語リソースの一部を更新するためのハンドラ
func (h *WordHandler) PatchWord(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format", "word_id_str", wordIDStr, "error", err)
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	var req model.PatchWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	if err := webutil.Validator.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			logger.Warn("Validation failed for PatchWord", "errors", validationErrors.Error())
			appErr := webutil.NewValidationErrorResponse(validationErrors)
			webutil.HandleError(w, logger, appErr)
		} else {
			logger.Error("Unexpected error during validation for PatchWord", "error", err)
			webutil.HandleError(w, logger, err)
		}
		return
	}

	word, err := h.service.PatchWord(r.Context(), userID, wordID, &req)
	if err != nil {
		logger.Error("Error patching word in service", "error", err, "request", req)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word patched successfully")
	webutil.RespondWithJSON(w, http.StatusOK, word, logger)
}

// DeleteWord は特定の単語リソースを削除するためのハンドラ
func (h *WordHandler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format", "word_id_str", wordIDStr, "error", err)
		appErr := model.NewAppError("INVALID_URL_PARAM", "word_idの形式が正しくありません。", "word_id", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	err = h.service.DeleteWord(r.Context(), userID, wordID)
	if err != nil {
		logger.Error("Error deleting word in service", "error", err)
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Word deleted successfully (or was already deleted)")
	w.WriteHeader(http.StatusNoContent)
}
