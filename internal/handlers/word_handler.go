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

func (h *WordHandler) UpdateWord(w http.ResponseWriter, r *http.Request) {
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

	var req model.UpdateWordRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if req.Term == nil && req.Definition == nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "No fields provided for update")
		return
	}
	// 入力値自体のバリデーション (空文字列を許可しないなど) もここで行う

	word, err := h.service.UpdateWord(r.Context(), tenantID, wordID, &req)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err)
		log.Printf("Error updating word %s: %v (status: %d)", wordIDStr, err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error())
		return
	}

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
