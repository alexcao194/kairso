package task

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
)

type forbiddenService struct{ Service }

// Pass the id-based access gate so the test exercises the per-action
// authorization (author-only) that maps ErrForbidden → 403.
func (forbiddenService) AuthorizeTaskAccess(context.Context, string, string) error { return nil }

func (forbiddenService) DeleteComment(context.Context, string, string) error {
	return fmt.Errorf("%w: chi tiết", ErrForbidden)
}

func (forbiddenService) UpdateComment(context.Context, string, string, UpdateTaskCommentRequest) (*TaskCommentResponse, error) {
	return nil, fmt.Errorf("%w: chi tiết", ErrForbidden)
}

func asUser(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey, "u1"))
}

// Lỗi quyền phải luôn ra HTTP 403 dù nội dung thông báo là ngôn ngữ nào: handler
// nhận biết bằng errors.Is(err, ErrForbidden), không so khớp chuỗi.
func TestCommentForbiddenMapsTo403(t *testing.T) {
	h := NewHandler(forbiddenService{})

	w := httptest.NewRecorder()
	h.DeleteComment(w, asUser(httptest.NewRequest(http.MethodDelete, "/x", nil)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("DeleteComment: want 403, got %d (%s)", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.UpdateComment(w, asUser(httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"content":"hi"}`))))
	if w.Code != http.StatusForbidden {
		t.Fatalf("UpdateComment: want 403, got %d (%s)", w.Code, w.Body.String())
	}
}
