package rest

import (
	"net/http"
	"strings"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/jwt"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/alexcao194/kairso/pkg/realtime"
)

type RealtimeHandler struct {
	hub     *realtime.Hub
	jwtSvc  jwt.TokenService
	queries *database.Queries
}

func NewRealtimeHandler(hub *realtime.Hub, jwtSvc jwt.TokenService, queries *database.Queries) *RealtimeHandler {
	return &RealtimeHandler{hub: hub, jwtSvc: jwtSvc, queries: queries}
}

// RegisterRoutes mounts /ws outside the /api/ prefix: browsers' native
// WebSocket API cannot set the Authorization or X-App-Token headers that the
// REST middleware chain requires, so the JWT is instead passed as a query
// parameter and verified here directly.
func (h *RealtimeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws", h.Serve)
}

func (h *RealtimeHandler) Serve(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "token là bắt buộc")
		return
	}
	claims, err := h.jwtSvc.ValidateToken(token)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID, err := pgutil.ParseUUID(claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Người dùng không hợp lệ")
		return
	}
	workspaces, err := h.queries.ListWorkspacesByUser(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Không thể xác định workspace của bạn")
		return
	}
	workspaceIDs := make([]string, len(workspaces))
	for i, ws := range workspaces {
		workspaceIDs[i] = pgutil.UUIDToString(ws.ID)
	}
	// Same reasoning as the token above: the browser's native WebSocket API
	// can't set custom headers on the handshake request, so the client's
	// connection id — used to skip echoing its own mutations back to it —
	// is passed as a query param here too, matching the X-Conn-Id header it
	// sends on ordinary REST calls (see middleware.Authenticate).
	connID := r.URL.Query().Get("connId")

	// On failure, gorilla/websocket has already written the HTTP error
	// response as part of Upgrade(), so there's nothing left to do here.
	_ = h.hub.ServeWS(w, r, workspaceIDs, connID)
}
