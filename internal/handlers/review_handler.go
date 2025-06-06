// internal/handlers/review_handler.go
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

// ReviewHandler 構造体に logger フィールドを追加
type ReviewHandler struct {
	service service.ReviewService
	logger  *slog.Logger // slog.Logger フィールドを追加
}

// NewReviewHandler コンストラクタで logger を受け取るように変更
func NewReviewHandler(s service.ReviewService, logger *slog.Logger) *ReviewHandler { // logger を引数に追加
	// ロガーが nil の場合にデフォルトロガーを使用
	if logger == nil {
		logger = slog.Default()
	}
	return &ReviewHandler{
		service: s,
		logger:  logger, // logger を設定
	}
}

func (h *ReviewHandler) GetReviewWords(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "GetReviewWords"))

	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt", slog.String("error", err.Error()))
		appErr := model.NewAppError("UNAUTHORIZED", "認証情報が見つかりません。", "", model.ErrForbidden)
		webutil.HandleError(w, logger, appErr)
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	reviewWords, err := h.service.GetReviewWords(r.Context(), tenantID)
	if err != nil {
		logger.Error("Error getting review words from service", slog.Any("error", err))
		webutil.HandleError(w, logger, err)
		return
	}

	if reviewWords == nil {
		reviewWords = []*model.ReviewWordResponse{}
	}
	logger.Info("Review words retrieved successfully", slog.Int("count", len(reviewWords)))
	webutil.RespondWithJSON(w, http.StatusOK, reviewWords, logger)
}

func (h *ReviewHandler) UpsertLearningProgressBasedOnReview(w http.ResponseWriter, r *http.Request) {
	logger := h.logger.With(slog.String("handler", "UpsertLearningProgressBasedOnReview"))

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

	var req model.SubmitReviewRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode request body", slog.String("error", err.Error()))
		appErr := model.NewAppError("INVALID_REQUEST_BODY", "リクエストボディの形式が正しくありません。", "", model.ErrInvalidInput)
		webutil.HandleError(w, logger, appErr)
		return
	}

	err = h.service.UpsertLearningProgressBasedOnReview(r.Context(), tenantID, wordID, req.IsCorrect)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			logger.Info("UpsertLearningProgressBasedOnReview service returned not found")
		} else {
			logger.Error("Error submitting review result in service")
		}
		webutil.HandleError(w, logger, err)
		return
	}

	logger.Info("Review result submitted successfully", slog.Bool("is_correct_submitted", req.IsCorrect))
	w.WriteHeader(http.StatusNoContent)
}
