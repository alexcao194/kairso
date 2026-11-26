package quote

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docsign"
)

type excelLabels struct {
	title, number, date, validUntil, about, customer, issuer, taxCode, tel, contact                    string
	no, item, desc, unit, qty, unitPrice, discount, amount, subtotal, discountTotal, vat, grand, words string
	notes, terms, bank, preparer                                                                       string
}

var quoteExcelLabels = map[string]excelLabels{
	"vi": {
		title: "BẢNG BÁO GIÁ", number: "Số", date: "Ngày", validUntil: "Hiệu lực đến", about: "V/v",
		customer: "Kính gửi", issuer: "Đơn vị báo giá", taxCode: "MST", tel: "ĐT", contact: "Người liên hệ",
		no: "STT", item: "Hạng mục", desc: "Mô tả", unit: "ĐVT", qty: "SL", unitPrice: "Đơn giá", discount: "CK (%)",
		amount: "Thành tiền", subtotal: "Cộng tiền hàng", discountTotal: "Chiết khấu", vat: "Thuế VAT", grand: "TỔNG CỘNG",
		words: "Bằng chữ", notes: "Ghi chú", terms: "Điều khoản & điều kiện", bank: "Thông tin chuyển khoản", preparer: "Người lập",
	},
	"en": {
		title: "QUOTATION", number: "No.", date: "Date", validUntil: "Valid until", about: "Re",
		customer: "To", issuer: "Quoted by", taxCode: "Tax code", tel: "Tel", contact: "Contact",
		no: "No.", item: "Item", desc: "Description", unit: "Unit", qty: "Qty", unitPrice: "Unit price", discount: "Disc. (%)",
		amount: "Amount", subtotal: "Subtotal", discountTotal: "Discount", vat: "VAT", grand: "GRAND TOTAL",
		words: "In words", notes: "Notes", terms: "Terms & conditions", bank: "Bank details", preparer: "Prepared by",
	},
}

func fmtQuoteDate(d string) string {
	if t, err := time.Parse("2006-01-02", d); err == nil {
		return t.Format("02/01/2006")
	}
	return d
}

func pct(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func partyLines(b *docexport.Book, p PartyInfo, l excelLabels) {
	if p.Address != "" {
		b.Line(p.Address)
	}
	var parts []string
	if p.TaxCode != "" {
		parts = append(parts, fmt.Sprintf("%s: %s", l.taxCode, p.TaxCode))
	}
	if p.Phone != "" {
		parts = append(parts, fmt.Sprintf("%s: %s", l.tel, p.Phone))
	}
	if p.Email != "" {
		parts = append(parts, p.Email)
	}
	if len(parts) > 0 {
		b.Line(strings.Join(parts, " · "))
	}
	var more []string
	if p.ContactName != "" {
		more = append(more, fmt.Sprintf("%s: %s", l.contact, p.ContactName))
	}
	if p.Website != "" {
		more = append(more, p.Website)
	}
	if len(more) > 0 {
		b.Line(strings.Join(more, " · "))
	}
}

// ExportQuoteExcel renders a quote as an .xlsx sheet in the quote's language.
func (s *Service) ExportQuoteExcel(ctx context.Context, workspaceID, callerID, quoteID string) (*ExportFile, error) {
	q, err := s.GetQuote(ctx, workspaceID, callerID, quoteID)
	if err != nil {
		return nil, err
	}
	return quoteWorkbook(q)
}

func quoteWorkbook(q *QuoteResponse) (*ExportFile, error) {
	l, ok := quoteExcelLabels[q.Language]
	if !ok {
		l = quoteExcelLabels["vi"]
	}
	hasDiscount := false
	for _, it := range q.Items {
		if it.DiscountPercent > 0 {
			hasDiscount = true
		}
	}
	// STT | Item | Description | Unit | Qty | Unit price | [Discount] | Amount
	widths := []float64{6, 30, 34, 9, 9, 16}
	if hasDiscount {
		widths = append(widths, 9)
	}
	widths = append(widths, 18)
	b, err := docexport.New(q.Number, widths, q.MinorUnits)
	if err != nil {
		return nil, err
	}
	money := func(v int64) docexport.Money { return docexport.MinorToMajor(v, q.MinorUnits) }

	b.Bold(q.Issuer.Name)
	partyLines(b, q.Issuer, l)
	b.Blank()
	b.Title(l.title)
	sub := fmt.Sprintf("%s: %s · %s: %s", l.number, q.Number, l.date, fmtQuoteDate(q.IssueDate))
	if q.ValidUntil != "" {
		sub += fmt.Sprintf(" · %s: %s", l.validUntil, fmtQuoteDate(q.ValidUntil))
	}
	b.Subtitle(sub)
	if q.Title != "" {
		b.Subtitle(fmt.Sprintf("%s: %s", l.about, q.Title))
	}
	b.Blank()
	b.Bold(fmt.Sprintf("%s: %s", l.customer, q.Customer.Name))
	partyLines(b, q.Customer, l)
	b.Blank()

	head := []string{l.no, l.item, l.desc, l.unit, l.qty, fmt.Sprintf("%s (%s)", l.unitPrice, q.Currency)}
	if hasDiscount {
		head = append(head, l.discount)
	}
	head = append(head, fmt.Sprintf("%s (%s)", l.amount, q.Currency))
	b.Header(head...)
	for i, it := range q.Items {
		row := []any{i + 1, it.Name, it.Description, docexport.Center(it.Unit), docexport.Qty(it.Quantity), money(it.UnitPrice)}
		if hasDiscount {
			var d any
			if it.DiscountPercent > 0 {
				d = docexport.Qty(it.DiscountPercent)
			}
			row = append(row, d)
		}
		row = append(row, money(it.LineTotal))
		b.Row(row...)
	}
	b.Total(l.subtotal, money(q.Subtotal), false)
	if q.DiscountTotal > 0 {
		label := l.discountTotal
		if q.DiscountType == "percent" {
			label += fmt.Sprintf(" (%s%%)", pct(q.DiscountPercent))
		}
		b.Total(label, -money(q.DiscountTotal), false)
	}
	b.Total(fmt.Sprintf("%s (%s%%)", l.vat, pct(q.VatRate)), money(q.TaxTotal), false)
	b.Total(l.grand, money(q.GrandTotal), true)
	if q.AmountInWords != "" {
		b.Italic(fmt.Sprintf("%s: %s", l.words, q.AmountInWords))
	}

	for _, sec := range []struct{ title, body string }{{l.notes, q.Notes}, {l.terms, q.Terms}, {l.bank, q.Issuer.BankInfo}} {
		if strings.TrimSpace(sec.body) == "" {
			continue
		}
		b.Blank()
		b.Bold(sec.title)
		b.Line(sec.body)
	}
	for _, sig := range q.Signatures {
		if sig.Role == docsign.RolePreparer {
			b.Blank()
			b.Line(fmt.Sprintf("%s: %s (%s)", l.preparer, sig.Name, sig.SignedAt.Format("02/01/2006")))
		}
	}

	data, err := b.Bytes()
	if err != nil {
		return nil, err
	}
	return &ExportFile{Filename: docexport.SafeFilename(q.Number) + ".xlsx", Data: data}, nil
}

func writeFile(w http.ResponseWriter, file *ExportFile) {
	w.Header().Set("Content-Type", docexport.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
