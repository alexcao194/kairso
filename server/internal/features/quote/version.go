package quote

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/jackc/pgx/v5/pgtype"
)

// NewVersion copies a sent/decided quote into the next revision of the same chain
// as an editable draft. Every version keeps the root's number (BG-2026-0001); the version is
// stored separately, never appended to the number. The source keeps its status and
// snapshots untouched; both quotes get a history event pointing at each other.
func (s *Service) NewVersion(ctx context.Context, workspaceID, callerID, quoteID string) (*QuoteResponse, error) {
	ws, caller, err := s.access(ctx, workspaceID, callerID, true)
	if err != nil {
		return nil, err
	}
	id, err := parseID(quoteID, "quoteId")
	if err != nil {
		return nil, err
	}
	src, ok, err := s.repo.GetQuote(ctx, id, ws)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	if src.Status == StatusDraft {
		return nil, invalid("Báo giá nháp vẫn sửa trực tiếp được, không cần tạo phiên bản mới")
	}
	rootID := src.ID
	if src.RootID.Valid {
		rootID = src.RootID
	}
	srcItems, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	st, err := s.settingsOrDefault(ctx, ws)
	if err != nil {
		return nil, err
	}
	today := vietnamToday()
	var validUntil pgtype.Date
	if st.DefaultValidDays > 0 {
		validUntil = pgtype.Date{Time: today.AddDate(0, 0, int(st.DefaultValidDays)), Valid: true}
	}
	items := make([]database.InsertQuoteItemParams, 0, len(srcItems))
	for _, it := range srcItems {
		items = append(items, database.InsertQuoteItemParams{
			Position: it.Position, Name: it.Name, Description: it.Description, Unit: it.Unit,
			QuantityMilli: it.QuantityMilli, UnitPrice: it.UnitPrice, DiscountBp: it.DiscountBp, LineTotal: it.LineTotal,
		})
	}

	var newID pgtype.UUID
	err = s.repo.ExecTx(ctx, func(tx Repository) error {
		versions, err := tx.ListQuoteVersions(ctx, ws, rootID)
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			return ErrNotFound
		}
		next := int32(1)
		for _, v := range versions {
			if v.Version >= next {
				next = v.Version + 1
			}
		}
		number := versions[0].Number // versions[0] is the root (v1)
		newID, err = tx.CreateQuote(ctx, database.CreateQuoteParams{
			WorkspaceID: ws, ProjectID: src.ProjectID, ModuleID: src.ModuleID, CustomerID: src.CustomerID,
			Number: number, Title: src.Title, Currency: src.Currency, Language: src.Language,
			IssueDate: pgtype.Date{Time: today, Valid: true}, ValidUntil: validUntil,
			VatBp: src.VatBp, Terms: src.Terms, CreatedBy: caller,
		})
		if err != nil {
			return err
		}
		if err := tx.SetQuoteVersion(ctx, database.SetQuoteVersionParams{ID: newID, WorkspaceID: ws, RootID: rootID, Version: next}); err != nil {
			return err
		}
		if err := tx.UpdateQuoteDraft(ctx, database.UpdateQuoteDraftParams{
			ID: newID, WorkspaceID: ws, CustomerID: src.CustomerID, Title: src.Title, Currency: src.Currency,
			Language: src.Language, IssueDate: pgtype.Date{Time: today, Valid: true}, ValidUntil: validUntil,
			DiscountType: src.DiscountType, DiscountValue: src.DiscountValue, VatBp: src.VatBp,
			Subtotal: src.Subtotal, DiscountTotal: src.DiscountTotal, TaxTotal: src.TaxTotal, GrandTotal: src.GrandTotal,
			Notes: src.Notes, Terms: src.Terms,
		}); err != nil {
			return err
		}
		if err := tx.ReplaceItems(ctx, newID, items); err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, newID, "created", caller, fmt.Sprintf("version:v%d", src.Version)); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, id, "new_version", caller, fmt.Sprintf("v%d", next))
	})
	if err != nil {
		return nil, err
	}
	return s.loadAndEmit(ctx, "quote.created", workspaceID, callerID, ws, newID)
}

// @Summary Create the next version of a quote
// @Description Copies a sent/decided quote into a new editable draft with the next version number; the quote number stays the same.
// @Tags Quotes
// @Security BearerAuth
// @Produce json
// @Param workspaceId path string true "Workspace ID"
// @Param quoteId path string true "Quote ID"
// @Success 201 {object} QuoteResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/quotes/{quoteId}/new-version [post]
func (h *Handler) NewVersion(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.caller(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.NewVersion(r.Context(), r.PathValue("workspaceId"), callerID, r.PathValue("quoteId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}
