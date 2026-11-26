package meeting

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/alexcao194/kairso/pkg/httpx"
)

// SetPDF enables PDF export through the web app's print page.
func (h *Handler) SetPDF(e *docpdf.Exporter) { h.pdf = e }

// @Summary Export meeting minutes as a PDF
// @Description Prints the minutes' A4 print page on the server, the same document the browser print page shows.
// @Tags Meetings
// @Security BearerAuth
// @Produce application/pdf
// @Param meetingId path string true "Meeting minutes ID"
// @Param template query string false "Layout" Enums(modern, admin)
// @Param sign query string false "Signature block (admin layout): digital stamps or blank for wet ink" Enums(digital, wet)
// @Param place query string false "Place in the date line (admin layout)"
// @Success 200 {file} binary "Minutes (.pdf)"
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 503 {object} httpx.ErrorResponse
// @Router /v1/meetings/{meetingId}/pdf [get]
func (h *Handler) ExportMeetingPDF(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	m, err := h.svc.Get(r.Context(), r.PathValue("meetingId"), userID)
	if err != nil {
		writeServiceError(w, err, "Không thể xuất biên bản họp")
		return
	}
	query := docpdf.PrintQuery(r)
	query.Set("ws", m.WorkspaceID)
	docpdf.AllowSlowResponse(w)
	data, err := h.pdf.Render(r.Context(), userID, docpdf.PagePath("/print/meetings/"+url.PathEscape(m.ID), query))
	if err != nil {
		slog.Error("meeting pdf export failed", "meeting", m.ID, "error", err)
		httpx.Error(w, docpdf.Status(err), "Không thể tạo PDF biên bản họp: "+err.Error())
		return
	}
	name := "Bien-ban-hop-" + strings.ReplaceAll(fmtMeetingDate(m.MeetingDate), "/", "-")
	docpdf.Write(w, docexport.SafeFilename(name)+".pdf", data)
}
