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
	logger := h.logger
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for GetReviewWords", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	reviewWords, err := h.service.GetReviewWords(r.Context(), tenantID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err) // 通常は500のはず
		// slog でエラーログ (サービス層エラー)
		logger.Error("Error getting review words from service",
			slog.Any("error", err),
			slog.Int("status_code", statusCode),
		)
		webutil.RespondWithError(w, statusCode, "Failed to get review words") // 汎用メッセージ
		return
	}

	if reviewWords == nil {
		reviewWords = []*model.ReviewWordResponse{}
	}
	// slog で成功ログ (任意)
	logger.Info("Review words retrieved successfully", slog.Int("count", len(reviewWords)))
	webutil.RespondWithJSON(w, http.StatusOK, reviewWords)
}

func (h *ReviewHandler) UpsertLearningProgressBasedOnReview(w http.ResponseWriter, r *http.Request) {
	logger := h.logger
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		logger.Warn("Unauthorized access attempt for UpsertLearningProgressBasedOnReview", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	logger = logger.With(slog.String("tenant_id", tenantID.String()))

	wordIDStr := chi.URLParam(r, "word_id")
	wordID, err := uuid.Parse(wordIDStr)
	if err != nil {
		logger.Warn("Invalid word ID format in URL for UpsertLearningProgressBasedOnReview", slog.String("word_id_str", wordIDStr), slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid word ID format")
		return
	}
	logger = logger.With(slog.String("word_id", wordID.String()))

	var req model.SubmitReviewRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		logger.Warn("Failed to decode UpsertLearningProgressBasedOnReview request body", slog.String("error", err.Error()))
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	// is_correct フィールドの存在チェックは、サービス層またはここでさらに厳密に行うことも可能

	err = h.service.UpsertLearningProgressBasedOnReview(r.Context(), tenantID, wordID, req.IsCorrect)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		// slog でエラーまたは情報ログ (エラー種別による)
		// ★ 修正点: logAttrs スライスを使わず、直接 slog.Attr を渡す
		if errors.Is(err, model.ErrNotFound) {
			logger.Info("UpsertLearningProgressBasedOnReview service returned not found",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Bool("is_correct_submitted", req.IsCorrect),
			)
		} else {
			logger.Error("Error submitting review result in service",
				slog.Any("error", err),
				slog.Int("status_code", statusCode),
				slog.Bool("is_correct_submitted", req.IsCorrect),
			)
		}
		// クライアントにはサービス層から返されたエラーメッセージを使うか、汎用的なメッセージにするか
		webutil.RespondWithError(w, statusCode, "Failed to submit review result") // 汎用メッセージ例
		return
	}

	// slog で成功ログ
	logger.Info("Review result submitted successfully", slog.Bool("is_correct_submitted", req.IsCorrect))
	w.WriteHeader(http.StatusNoContent) // 204 No Content
}
