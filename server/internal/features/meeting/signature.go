package meeting

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/docsign"
	"github.com/alexcao194/kairso/internal/features/workspace"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/pgutil"
)

func (s *service) attachSignatures(ctx context.Context, m *MeetingMinutesResponse) error {
	id, err := pgutil.ParseUUID(m.ID)
	if err != nil {
		return err
	}
	m.Signatures, err = s.repo.Signatures().List(ctx, docsign.TypeMeeting, id)
	return err
}

// canSign: the preparer is whoever took the minutes (the creator or a listed recorder);
// the controller is the workspace holder.
func canSign(m *MeetingMinutesResponse, callerID, wsRole, role string) bool {
	switch role {
	case docsign.RolePreparer:
		if m.CreatedBy.ID == callerID {
			return true
		}
		for _, r := range m.Recorders {
			if r.Kind == ParticipantUser && r.UserID == callerID {
				return true
			}
		}
		return false
	case docsign.RoleController:
		return wsRole == string(workspace.WorkspaceRoleHolder)
	}
	return false
}

func (s *service) Sign(ctx context.Context, id, callerID, role string) (*MeetingMinutesResponse, error) {
	if !docsign.ValidRole(role) {
		return nil, invalid("vai trò ký không hợp lệ")
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	wsRole, err := s.callerRole(ctx, m.WorkspaceID, callerID)
	if err != nil {
		return nil, err
	}
	if !canSign(m, callerID, wsRole, role) {
		if role == docsign.RoleController {
			return nil, fmt.Errorf("%w: chỉ chủ workspace mới ký ở vai trò người kiểm soát", ErrForbidden)
		}
		return nil, fmt.Errorf("%w: chỉ người ghi chép mới ký ở vai trò người lập", ErrForbidden)
	}
	mid, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return nil, ErrForbidden
	}
	if err := s.repo.Signatures().Sign(ctx, docsign.TypeMeeting, mid, role, uid); err != nil {
		return nil, err
	}
	return s.afterSign(ctx, m, callerID)
}

func (s *service) Unsign(ctx context.Context, id, callerID, role string) (*MeetingMinutesResponse, error) {
	if !docsign.ValidRole(role) {
		return nil, invalid("vai trò ký không hợp lệ")
	}
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	wsRole, err := s.callerRole(ctx, m.WorkspaceID, callerID)
	if err != nil {
		return nil, err
	}
	mid, err := pgutil.ParseUUID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	sig, ok, err := s.repo.Signatures().Find(ctx, docsign.TypeMeeting, mid, role)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, docsign.ErrNoSignature
	}
	if sig.UserID != callerID && wsRole != string(workspace.WorkspaceRoleHolder) {
		return nil, fmt.Errorf("%w: chỉ người đã ký hoặc chủ workspace mới gỡ được chữ ký", ErrForbidden)
	}
	if err := s.repo.Signatures().Unsign(ctx, docsign.TypeMeeting, mid, role); err != nil {
		return nil, err
	}
	return s.afterSign(ctx, m, callerID)
}

func (s *service) afterSign(ctx context.Context, m *MeetingMinutesResponse, callerID string) (*MeetingMinutesResponse, error) {
	if err := s.attachSignatures(ctx, m); err != nil {
		return nil, err
	}
	s.broadcast(ctx, m.WorkspaceID, "meeting.updated", m.ID, callerID, m)
	return m, nil
}

// @Summary Sign meeting minutes
// @Description Stamps the caller's name and the current time. role=preparer: the note-taker (creator or a listed recorder); role=controller: the workspace holder.
// @Tags Meetings
// @Security BearerAuth
// @Produce json
// @Param meetingId path string true "Meeting minutes ID"
// @Param role path string true "Signature role" Enums(preparer, controller)
// @Success 200 {object} MeetingMinutesResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /v1/meetings/{meetingId}/signatures/{role} [post]
func (h *Handler) SignMeeting(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	resp, err := h.svc.Sign(r.Context(), r.PathValue("meetingId"), userID, r.PathValue("role"))
	if err != nil {
		writeServiceError(w, err, "Không thể ký biên bản họp")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Remove a meeting signature
// @Description Only the signer or the workspace holder can remove it.
// @Tags Meetings
// @Security BearerAuth
// @Produce json
// @Param meetingId path string true "Meeting minutes ID"
// @Param role path string true "Signature role" Enums(preparer, controller)
// @Success 200 {object} MeetingMinutesResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/meetings/{meetingId}/signatures/{role} [delete]
func (h *Handler) UnsignMeeting(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	resp, err := h.svc.Unsign(r.Context(), r.PathValue("meetingId"), userID, r.PathValue("role"))
	if err != nil {
		writeServiceError(w, err, "Không thể gỡ chữ ký biên bản họp")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
