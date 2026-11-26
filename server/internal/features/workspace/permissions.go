package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/internal/features/access"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/pgutil"
)

// RolePermissionsResponse is the workspace's permission matrix and what the caller may do.
type RolePermissionsResponse struct {
	// Permissions lists every permission key in display order.
	Permissions []string `json:"permissions"`
	// Roles maps "admin" and "member" to their permissions. The holder always has all of them.
	Roles map[string][]string `json:"roles"`
	// Defaults are the permissions each role has when the workspace never changed them.
	Defaults map[string][]string `json:"defaults"`
	// Role is the caller's role; Mine the caller's permissions.
	Role string   `json:"role"`
	Mine []string `json:"mine"`
}

// UpdateRolePermissionsRequest sets the permissions of "admin" and/or "member"; a role left out
// keeps its current permissions.
type UpdateRolePermissionsRequest struct {
	Roles map[string][]string `json:"roles"`
}

func toStrings(ps []access.Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}

func matrixResponse(m access.Matrix, g access.Grant) *RolePermissionsResponse {
	resp := &RolePermissionsResponse{
		Permissions: toStrings(access.All),
		Roles:       map[string][]string{},
		Defaults:    map[string][]string{},
		Role:        g.Role,
		Mine:        toStrings(g.Permissions()),
	}
	for role, perms := range m.Resolved() {
		resp.Roles[role] = toStrings(perms)
		resp.Defaults[role] = toStrings(access.Defaults(role))
	}
	return resp
}

// GetRolePermissions returns the matrix to any member.
func (s *service) GetRolePermissions(ctx context.Context, workspaceID, callerID string) (*RolePermissionsResponse, error) {
	g, err := s.repo.GetCallerAccess(ctx, workspaceID, callerID)
	if err != nil {
		return nil, err
	}
	if !g.IsMember() {
		return nil, ErrForbidden
	}
	raw, err := s.repo.GetRolePermissions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return matrixResponse(access.ParseMatrix(raw), g), nil
}

// UpdateRolePermissions lets the holder change what admins and members may do.
func (s *service) UpdateRolePermissions(ctx context.Context, workspaceID, callerID string, req UpdateRolePermissionsRequest) (*RolePermissionsResponse, error) {
	g, err := s.repo.GetCallerAccess(ctx, workspaceID, callerID)
	if err != nil {
		return nil, err
	}
	if !g.IsHolder() {
		return nil, ErrForbidden
	}
	next := access.Matrix{}
	for role, perms := range req.Roles {
		for _, p := range perms {
			next[role] = append(next[role], access.Permission(p))
		}
		if next[role] == nil {
			next[role] = []access.Permission{}
		}
	}
	if err := access.Validate(next); err != nil {
		return nil, ValidationError(err.Error())
	}
	raw, err := s.repo.GetRolePermissions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	merged := access.ParseMatrix(raw)
	for role, perms := range next {
		merged[role] = perms
	}
	merged = access.Normalize(merged)
	body, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpdateRolePermissions(ctx, workspaceID, body)
	if err != nil {
		return nil, err
	}
	matrix := access.ParseMatrix(saved)
	s.broadcast(ctx, workspaceID, "workspace.permissions_updated", workspaceID, callerID, matrix.Resolved())
	return matrixResponse(matrix, g), nil
}

// ValidationError is a bad request from the client.
type ValidationError string

func (e ValidationError) Error() string { return string(e) }

func (r *repository) GetRolePermissions(ctx context.Context, workspaceID string) ([]byte, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	return r.queries.GetWorkspaceRolePermissions(ctx, wid)
}

func (r *repository) UpdateRolePermissions(ctx context.Context, workspaceID string, matrix []byte) ([]byte, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	return r.queries.UpdateWorkspaceRolePermissions(ctx, database.UpdateWorkspaceRolePermissionsParams{ID: wid, RolePermissions: matrix})
}

// guard runs next only when the caller passes check in the {workspaceId} of the path.
func (h *Handler) guard(check func(access.Grant) bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID, ok := middleware.GetUserIDFromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
			return
		}
		g, err := h.svc.CallerAccess(r.Context(), r.PathValue("workspaceId"), callerID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "Workspace không hợp lệ")
			return
		}
		if !g.IsMember() {
			httpx.Error(w, http.StatusNotFound, "Không tìm thấy workspace")
			return
		}
		if !check(g) {
			httpx.Error(w, http.StatusForbidden, ErrForbidden.Error())
			return
		}
		next(w, r)
	}
}

func (h *Handler) memberOnly(next http.HandlerFunc) http.HandlerFunc {
	return h.guard(func(access.Grant) bool { return true }, next)
}

func (h *Handler) holderOnly(next http.HandlerFunc) http.HandlerFunc {
	return h.guard(access.Grant.IsHolder, next)
}

func (h *Handler) permitted(p access.Permission, next http.HandlerFunc) http.HandlerFunc {
	return h.guard(func(g access.Grant) bool { return g.Can(p) }, next)
}

// CallerAccess returns what callerID may do in the workspace.
func (s *service) CallerAccess(ctx context.Context, workspaceID, callerID string) (access.Grant, error) {
	return s.repo.GetCallerAccess(ctx, workspaceID, callerID)
}

// @Summary Get the workspace's role permissions
// @Description The permission matrix for admins and members (the holder has every permission) and the caller's own permissions.
// @Tags Workspaces
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Success 200 {object} RolePermissionsResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/permissions [get]
func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.GetUserIDFromContext(r.Context())
	resp, err := h.svc.GetRolePermissions(r.Context(), r.PathValue("workspaceId"), callerID)
	if err != nil {
		writePermErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Update the workspace's role permissions
// @Description Holder only. Sets which permissions admins and/or members have; a role left out keeps its permissions.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Param request body UpdateRolePermissionsRequest true "Permissions per role"
// @Success 200 {object} RolePermissionsResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/permissions [put]
func (h *Handler) UpdateRolePermissions(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.GetUserIDFromContext(r.Context())
	var req UpdateRolePermissionsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Nội dung yêu cầu không hợp lệ: "+err.Error())
		return
	}
	resp, err := h.svc.UpdateRolePermissions(r.Context(), r.PathValue("workspaceId"), callerID, req)
	if err != nil {
		writePermErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func writePermErr(w http.ResponseWriter, err error) {
	var ve ValidationError
	switch {
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, err.Error())
	case errors.As(err, &ve):
		httpx.Error(w, http.StatusBadRequest, ve.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "Lỗi máy chủ: "+err.Error())
	}
}
