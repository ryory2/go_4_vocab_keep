// internal/handlers/tenant_handler.go
package handlers

import (
	"log"
	"net/http"

	// プロジェクト名修正
	"go_1_test_repository/internal/service" // プロジェクト名修正
	"go_1_test_repository/internal/webutil" // プロジェクト名修正
)

type TenantHandler struct {
	service service.TenantService
}

func NewTenantHandler(s service.TenantService) *TenantHandler {
	return &TenantHandler{service: s}
}

// CreateTenantRequest はテナント作成リクエストのボディを表すDTO
type CreateTenantRequest struct {
	Name string `json:"name" validate:"required"`
}

// CreateTenant ハンドラ: POST /tenants
func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := webutil.DecodeJSONBody(r, &req); err != nil {
		webutil.RespondWithError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	// 簡単なバリデーション (本来はvalidatorライブラリ推奨)
	if req.Name == "" {
		webutil.RespondWithError(w, http.StatusBadRequest, "Tenant name is required")
		return
	}

	tenant, err := h.service.CreateTenant(r.Context(), req.Name)
	if err != nil {
		statusCode := webutil.MapErrorToStatusCode(err) // エラーに応じたステータスコード取得
		log.Printf("Error creating tenant: %v (status: %d)", err, statusCode)
		webutil.RespondWithError(w, statusCode, err.Error()) // エラーメッセージを返す
		return
	}
	webutil.RespondWithJSON(w, http.StatusCreated, tenant) // 作成されたテナント情報を返す
}
