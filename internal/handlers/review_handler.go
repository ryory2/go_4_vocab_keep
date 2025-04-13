// internal/handlers/review_handler.go
package handlers

import (
	"log"
	"net/http"

	"go_1_test_repository/internal/middleware"
	"go_1_test_repository/internal/model"
	"go_1_test_repository/internal/service"
	"go_1_test_repository/internal/webutil"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ReviewHandler struct {
	service service.ReviewService
}

func NewReviewHandler(s service.ReviewService) *ReviewHandler {
	return &ReviewHandler{service: s}
}

func (h *ReviewHandler) GetReviewWords(w http.ResponseWriter, r *http.Request) {
	tenantID, err := middleware.GetTenantIDFromContext(r.Context())
	if err != nil {
		webutil.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	reviewWords, err := h.service.GetReviewWords(r.Context(), tenantID)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err) // 通常は500のはず
		log.Printf("Error getting review words: %v (status: %d)", err, statusCode)
		webutil.RespondWithError(w, statusCode, "Failed to get review words")
		return
	}

	if reviewWords == nil {
		reviewWords = []*model.ReviewWordResponse{}
	}
	webutil.RespondWithJSON(w, http.StatusOK, reviewWords)
}

func (h *ReviewHandler) SubmitReviewResult(w http.ResponseWriter, r *http.Request) {
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

	var req model.SubmitReviewRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	err = h.service.SubmitReviewResult(r.Context(), tenantID, wordID, req.IsCorrect)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		log.Printf("Error submitting review result for word %s: %v (status: %d)", wordIDStr, err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
