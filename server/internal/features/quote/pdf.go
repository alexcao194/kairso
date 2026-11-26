package quote

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/alexcao194/kairso/pkg/httpx"
)

// SetPDF enables PDF export through the web app's print page.
func (h *Handler) SetPDF(e *docpdf.Exporter) { h.pdf = e }

// @Summary Export a quote as a PDF
// @Description Prints the quote's A4 print page on the server, the same document the browser print page shows.
// @Tags Quotes
// @Security BearerAuth
// @Produce application/pdf
// @Param workspaceId path string true "Workspace ID"
// @Param quoteId path string true "Quote ID"
// @Param template query string false "Layout" Enums(modern, admin)
// @Param sign query string false "Signature block (admin layout): digital stamps or blank for wet ink" Enums(digital, wet)
// @Param place query string false "Place in the date line (admin layout)"
// @Param signer query string false "Approver title (admin layout)"
// @Param merge query string false "1: one signature block for preparer and approver, 0: two (admin layout)" Enums(0, 1)
// @Param fit query string false "0: let a long quote run onto more pages instead of shrinking it onto one (modern layout)" Enums(0, 1)
// @Success 200 {file} binary "Quote (.pdf)"
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 503 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/quotes/{quoteId}/pdf [get]
func (h *Handler) ExportQuotePDF(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.caller(w, r)
	if !ok {
		return
	}
	ws := r.PathValue("workspaceId")
	q, err := h.svc.GetQuote(r.Context(), ws, callerID, r.PathValue("quoteId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	query := docpdf.PrintQuery(r)
	query.Set("ws", ws)
	docpdf.AllowSlowResponse(w)
	data, err := h.pdf.Render(r.Context(), callerID, docpdf.PagePath("/print/quotes/"+url.PathEscape(q.ID), query))
	if err != nil {
		slog.Error("quote pdf export failed", "quote", q.ID, "error", err)
		httpx.Error(w, docpdf.Status(err), "Không thể tạo PDF báo giá: "+err.Error())
		return
	}
	docpdf.Write(w, docexport.SafeFilename(q.Number)+".pdf", data)
}
