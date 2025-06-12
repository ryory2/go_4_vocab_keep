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

type ReviewHandler struct {
	service service.ReviewService
	// logger  *slog.Logger // ベースロガーは不要
}

// NewReviewHandler コンストラクタから logger の引数を削除
func NewReviewHandler(s service.ReviewService) *ReviewHandler {
	return &ReviewHandler{
		service: s,
	}
}

func (h *ReviewHandler) GetReviewWords(w http.ResponseWriter, r *http.Request) {
	// リクエストコンテキストからロガーを取得
	logger := middleware.GetLogger(r.Context())

	// 新しい方法でユーザーIDを取得 (tenantIDとして扱う)
	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With(slog.String("tenant_id", userID.String()))

	reviewWords, err := h.service.GetReviewWords(r.Context(), userID)
	if err != nil {
		logger.Error("Error getting review words from service", "error", err)
		webutil.HandleError(w, logger, err)
		return
	}

	if reviewWords == nil {
		reviewWords = []*model.ReviewWordResponse{}
	}
	logger.Info("Review words retrieved successfully", "count", len(reviewWords))
	webutil.RespondWithJSON(w, http.StatusOK, reviewWords, logger)
}

func (h *ReviewHandler) GetReviewSummary(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())

	userID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}
	logger = logger.With("tenant_id", userID.String())

	count, err := h.service.GetReviewWordsCount(r.Context(), userID)
	if err != nil {
		webutil.HandleError(w, logger, err)
		return
	}

	// レスポンスをJSONで返す
	response := map[string]int64{
		"review_count": count,
	}
	webutil.RespondWithJSON(w, http.StatusOK, response, logger)
}

func (h *ReviewHandler) UpsertLearningProgressBasedOnReview(w http.ResponseWriter, r *http.Request) {
	// リクエストコンテキストからロガーを取得
	logger := middleware.GetLogger(r.Context())

	// 新しい方法でユーザーIDを取得 (tenantIDとして扱う)
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

	var req model.SubmitReviewRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", "error", err)
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	// SubmitReviewRequest に validate タグが付いていると仮定
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

	err = h.service.UpsertLearningProgressBasedOnReview(r.Context(), userID, wordID, *req.IsCorrect)
	if err != nil {
		// サービス層から返されたエラーをそのまま処理
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Review result submitted successfully", "is_correct", *req.IsCorrect)
	w.WriteHeader(http.StatusNoContent)
}
