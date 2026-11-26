package oauth

import (
	"encoding/json"
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

// RegisterPublicRoutes adds the endpoints MCP clients call directly (no app signature):
// dynamic client registration and the token endpoint.
func (h *Handler) RegisterPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /oauth/register", h.register)
	mux.HandleFunc("POST /oauth/token", h.token)
}

// RegisterRoutes adds the consent endpoints the web app calls for a signed-in user.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMid *middleware.AuthMiddleware) {
	mux.HandleFunc("GET /v1/oauth/clients/{clientId}", authMid.Handle(middleware.RequireSession(h.GetClient)))
	mux.HandleFunc("POST /v1/oauth/authorize", authMid.Handle(middleware.RequireSession(h.Authorize)))
}

func writeOAuth(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeOAuth(w, http.StatusBadRequest, oauthErr(http.StatusBadRequest, "invalid_client_metadata", "body must be JSON"))
		return
	}
	resp, e := h.svc.Register(r.Context(), req)
	if e != nil {
		writeOAuth(w, e.status, e)
		return
	}
	writeOAuth(w, http.StatusCreated, resp)
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		writeOAuth(w, http.StatusBadRequest, oauthErr(http.StatusBadRequest, "invalid_request", "body must be form encoded"))
		return
	}
	resp, e := h.svc.Token(r.Context(), r.PostForm)
	if e != nil {
		writeOAuth(w, e.status, e)
		return
	}
	writeOAuth(w, http.StatusOK, resp)
}

// @Summary Describe an app asking for access
// @Description For the OAuth consent screen: the client's name, checked against the redirect URI
// @Tags OAuth
// @Produce json
// @Security BearerAuth
// @Param clientId path string true "OAuth client ID"
// @Param redirectUri query string true "Redirect URI from the authorization request"
// @Success 200 {object} ClientResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/oauth/clients/{clientId} [get]
func (h *Handler) GetClient(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetClient(r.Context(), r.PathValue("clientId"), r.URL.Query().Get("redirectUri"))
	if err != nil {
		writeConsentError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Approve an OAuth authorization request
// @Description Issues an authorization code for the signed-in user and returns the client's redirect URL
// @Tags OAuth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body AuthorizeRequest true "Authorization request"
// @Success 200 {object} AuthorizeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Router /v1/oauth/authorize [post]
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	var req AuthorizeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Dữ liệu không hợp lệ")
		return
	}
	resp, err := h.svc.Authorize(r.Context(), userID, req)
	if err != nil {
		writeConsentError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func writeConsentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrClientNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrBadRedirect), errors.Is(err, ErrPKCE):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "Không thể xử lý yêu cầu cấp quyền")
	}
}
