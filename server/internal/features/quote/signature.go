package quote

import (
	"context"
	"github.com/alexcao194/kairso/internal/features/access"
	"net/http"

	"github.com/alexcao194/kairso/internal/features/docsign"
	"github.com/alexcao194/kairso/pkg/httpx"
)

// A quote has a single stamp: the person who prepared it.
func quoteRole(role string) error {
	if role != docsign.RolePreparer {
		return invalid("Báo giá chỉ có chữ ký của người lập")
	}
	return nil
}

// SignQuote stamps the quote with the caller's name and today's date as its preparer.
func (s *Service) SignQuote(ctx context.Context, workspaceID, callerID, quoteID, role string) (*QuoteResponse, error) {
	ws, caller, err := s.access(ctx, workspaceID, callerID, true)
	if err != nil {
		return nil, err
	}
	if err := quoteRole(role); err != nil {
		return nil, err
	}
	id, err := parseID(quoteID, "quoteId")
	if err != nil {
		return nil, err
	}
	if _, ok, err := s.repo.GetQuote(ctx, id, ws); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	if err := s.repo.Signatures().Sign(ctx, docsign.TypeQuote, id, role, caller); err != nil {
		return nil, err
	}
	resp, err := s.load(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "quote.updated", workspaceID, callerID, quoteID, resp)
	return resp, nil
}

// UnsignQuote removes the stamp. Only the signer or a workspace manager may do so.
func (s *Service) UnsignQuote(ctx context.Context, workspaceID, callerID, quoteID, role string) (*QuoteResponse, error) {
	ws, caller, err := s.access(ctx, workspaceID, callerID, true)
	if err != nil {
		return nil, err
	}
	if err := quoteRole(role); err != nil {
		return nil, err
	}
	id, err := parseID(quoteID, "quoteId")
	if err != nil {
		return nil, err
	}
	if _, ok, err := s.repo.GetQuote(ctx, id, ws); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotFound
	}
	sig, ok, err := s.repo.Signatures().Find(ctx, docsign.TypeQuote, id, role)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, docsign.ErrNoSignature
	}
	g, err := s.repo.GetCallerAccess(ctx, ws, caller)
	if err != nil {
		return nil, err
	}
	if sig.UserID != callerID && !g.Can(access.QuotesManage) {
		return nil, ErrForbidden
	}
	if err := s.repo.Signatures().Unsign(ctx, docsign.TypeQuote, id, role); err != nil {
		return nil, err
	}
	resp, err := s.load(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	s.emit(ctx, "quote.updated", workspaceID, callerID, quoteID, resp)
	return resp, nil
}

// @Summary Sign a quote as its preparer
// @Description Stamps the caller's name and the current time. Managers only.
// @Tags Quotes
// @Security BearerAuth
// @Produce json
// @Param workspaceId path string true "Workspace ID"
// @Param quoteId path string true "Quote ID"
// @Param role path string true "Signature role" Enums(preparer)
// @Success 200 {object} QuoteResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/quotes/{quoteId}/signatures/{role} [post]
func (h *Handler) SignQuote(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.caller(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.SignQuote(r.Context(), r.PathValue("workspaceId"), callerID, r.PathValue("quoteId"), r.PathValue("role"))
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// @Summary Remove a quote signature
// @Description Only the signer or a workspace manager can remove it.
// @Tags Quotes
// @Security BearerAuth
// @Produce json
// @Param workspaceId path string true "Workspace ID"
// @Param quoteId path string true "Quote ID"
// @Param role path string true "Signature role" Enums(preparer)
// @Success 200 {object} QuoteResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/quotes/{quoteId}/signatures/{role} [delete]
func (h *Handler) UnsignQuote(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.caller(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.UnsignQuote(r.Context(), r.PathValue("workspaceId"), callerID, r.PathValue("quoteId"), r.PathValue("role"))
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
