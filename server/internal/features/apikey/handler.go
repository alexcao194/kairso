package apikey

import (
	"errors"
	"net/http"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/pkg/httpx"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes adds key management. These need a signed-in session: an API key cannot
// list, create or revoke keys.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMid *middleware.AuthMiddleware) {
	mux.HandleFunc("GET /v1/users/me/api-keys", authMid.Handle(middleware.RequireSession(h.List)))
	mux.HandleFunc("POST /v1/users/me/api-keys", authMid.Handle(middleware.RequireSession(h.Create)))
	mux.HandleFunc("DELETE /v1/users/me/api-keys/{keyId}", authMid.Handle(middleware.RequireSession(h.Revoke)))
}

// @Summary List my API keys
// @Description Personal API keys of the caller (without their secrets)
// @Tags API keys
// @Produce json
// @Security BearerAuth
// @Success 200 {array} APIKeyResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Router /v1/users/me/api-keys [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	keys, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Không thể tải danh sách API key")
		return
	}
	httpx.JSON(w, http.StatusOK, keys)
}

// @Summary Create an API key
// @Description Create a personal API key. The plain key is returned only in this response.
// @Tags API keys
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateAPIKeyRequest true "Key name and lifetime"
// @Success 201 {object} CreatedAPIKeyResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Router /v1/users/me/api-keys [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	var req CreateAPIKeyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Dữ liệu không hợp lệ")
		return
	}
	resp, err := h.svc.Create(r.Context(), userID, req)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// @Summary Revoke an API key
// @Tags API keys
// @Security BearerAuth
// @Param keyId path string true "API key ID"
// @Success 204
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/users/me/api-keys/{keyId} [delete]
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	if err := h.svc.Revoke(r.Context(), userID, r.PathValue("keyId")); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "Không thể thu hồi API key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
