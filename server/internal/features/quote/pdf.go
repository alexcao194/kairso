package quote

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/alexcao194/kairso/internal/features/docsign"
	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/pgutil"
)

// SetPDF enables PDF export.
func (h *Handler) SetPDF(e *docpdf.Exporter) { h.pdf = e }

// @Summary Export a quote as a PDF
// @Description A4 PDF drawn on the server in Times New Roman, laid out like the print page.
// @Tags Quotes
// @Security BearerAuth
// @Produce application/pdf
// @Param workspaceId path string true "Workspace ID"
// @Param quoteId path string true "Quote ID"
// @Param template query string false "Layout" Enums(modern, admin)
// @Param sign query string false "Signature boxes (admin layout): stamped when signed in the app, or left blank to sign by hand" Enums(digital, wet)
// @Param place query string false "Place in the date line (admin layout); defaults to the end of the issuer's address"
// @Param signer query string false "Approver title (admin layout), default GIÁM ĐỐC"
// @Param merge query string false "1: one signature block when the director prepares the quote, 0: two (admin layout); default: one when the workspace owner signed" Enums(0, 1)
// @Success 200 {file} binary "Quote (.pdf)"
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 503 {object} httpx.ErrorResponse
// @Router /v1/workspaces/{workspaceId}/quotes/{quoteId}/pdf [get]
func (h *Handler) ExportQuotePDF(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.RequireUser(w, r)
	if !ok {
		return
	}
	ws := r.PathValue("workspaceId")
	q, err := h.svc.GetQuote(r.Context(), ws, callerID, r.PathValue("quoteId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	opts := docpdf.ParseOptions(r)
	var data []byte
	if opts.Admin {
		data, err = h.adminQuotePDF(r.Context(), ws, q, opts)
	} else {
		data, err = h.modernQuotePDF(r.Context(), ws, q)
	}
	if err != nil {
		slog.Error("quote pdf export failed", "quote", q.ID, "error", err)
		httpx.Error(w, docpdf.Status(err), "Không thể tạo PDF báo giá: "+err.Error())
		return
	}
	docpdf.Write(w, docexport.SafeFilename(q.Number)+".pdf", data)
}

type pdfLabels struct {
	docTitle, number, date, validUntil, to, project, module, taxCode, tel, contact string
	no, item, unit, qty, unitPrice, discount, amount, subtotal, discountTotal, vat string
	grand, inWords, notes, terms, bank, preparer, signHint, draft                  string
}

var quotePDFLabels = map[string]pdfLabels{
	"vi": {
		docTitle: "BÁO GIÁ", number: "Số", date: "Ngày", validUntil: "Hiệu lực đến", to: "Kính gửi",
		project: "Dự án", module: "Hạng mục dự án", taxCode: "MST", tel: "ĐT", contact: "Người liên hệ",
		no: "STT", item: "Hạng mục", unit: "ĐVT", qty: "SL", unitPrice: "Đơn giá", discount: "CK", amount: "Thành tiền",
		subtotal: "Cộng tiền hàng", discountTotal: "Chiết khấu", vat: "Thuế VAT", grand: "Tổng cộng", inWords: "Bằng chữ",
		notes: "Ghi chú", terms: "Điều khoản & điều kiện", bank: "Thông tin chuyển khoản", preparer: "Người lập báo giá",
		signHint: "(Ký, ghi rõ họ tên)", draft: "BẢN NHÁP",
	},
	"en": {
		docTitle: "QUOTATION", number: "No.", date: "Date", validUntil: "Valid until", to: "To",
		project: "Project", module: "Scope", taxCode: "Tax ID", tel: "Tel", contact: "Attn.",
		no: "#", item: "Item", unit: "Unit", qty: "Qty", unitPrice: "Unit price", discount: "Disc.", amount: "Amount",
		subtotal: "Subtotal", discountTotal: "Discount", vat: "VAT", grand: "Total", inWords: "In words",
		notes: "Notes", terms: "Terms & conditions", bank: "Payment details", preparer: "Prepared by",
		signHint: "(Signature & full name)", draft: "DRAFT",
	},
}

func quoteLang(q *QuoteResponse) string {
	if q.Language == "en" {
		return "en"
	}
	return "vi"
}

// formatNumber groups thousands like the print page (vi: 1.234,5 · en: 1,234.5).
func formatNumber(v float64, decimals int, fixed bool, lang string) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', decimals, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	if !fixed {
		frac = strings.TrimRight(frac, "0")
	}
	thou, dec := ".", ","
	if lang == "en" {
		thou, dec = ",", "."
	}
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(thou)
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += dec + frac
	}
	if v < 0 {
		out = "-" + out
	}
	return out
}

var currencySymbols = map[string][2]string{ // vi, en
	"VND": {"₫", "₫"}, "USD": {"US$", "$"}, "EUR": {"€", "€"}, "JPY": {"¥", "¥"}, "GBP": {"£", "£"},
}

// formatMoney renders minor units like Intl.NumberFormat with style "currency".
func formatMoney(minor int64, currency string, minorUnits int, lang string) string {
	v := float64(minor) / math.Pow10(minorUnits)
	n := formatNumber(v, minorUnits, true, lang)
	sym, ok := currencySymbols[currency]
	if !ok {
		sym = [2]string{currency, currency + " "}
	}
	if lang == "en" {
		if strings.HasPrefix(n, "-") {
			return "-" + sym[1] + n[1:]
		}
		return sym[1] + n
	}
	return n + " " + sym[0]
}

func pdfPartyLines(p PartyInfo, l pdfLabels) []string {
	var lines []string
	if p.Address != "" {
		lines = append(lines, p.Address)
	}
	if p.TaxCode != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", l.taxCode, p.TaxCode))
	}
	var contact []string
	if p.Phone != "" {
		contact = append(contact, fmt.Sprintf("%s: %s", l.tel, p.Phone))
	}
	if p.Email != "" {
		contact = append(contact, p.Email)
	}
	if len(contact) > 0 {
		lines = append(lines, strings.Join(contact, " · "))
	}
	if p.ContactName != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", l.contact, p.ContactName))
	}
	if p.Website != "" {
		lines = append(lines, p.Website)
	}
	return lines
}

func preparerSig(q *QuoteResponse) *docsign.Signature {
	for i := range q.Signatures {
		if q.Signatures[i].Role == docsign.RolePreparer {
			return &q.Signatures[i]
		}
	}
	return nil
}

func hasLineDiscount(q *QuoteResponse) bool {
	for _, it := range q.Items {
		if it.DiscountPercent > 0 {
			return true
		}
	}
	return false
}

// modernQuotePDF mirrors QuotePrint: logo and issuer, brand title block, customer/project
// strip, item table, totals box, notes and the preparer's signature.
func (h *Handler) modernQuotePDF(ctx context.Context, wsID string, q *QuoteResponse) ([]byte, error) {
	d, ws, err := h.pdf.Begin(ctx, wsID, docpdf.ModernMargins())
	if err != nil {
		return nil, err
	}
	lang := quoteLang(q)
	l := quotePDFLabels[lang]
	money := func(v int64) string { return formatMoney(v, q.Currency, q.MinorUnits, lang) }
	if q.Status == "draft" {
		d.Watermark(l.draft, d.Brand.Tint(0.82))
	}
	d.Start()
	left, width := d.Left(), d.Width()
	px := docpdf.Px

	// Header: logo + issuer (left), title + number/dates (right).
	y0 := d.P.GetY()
	d.Logo(left, y0, 15, ws.HasLogo)
	rightW := 62.0
	issuerX := left + 19
	issuerW := width - 19 - rightW - 4
	issuer := q.Issuer.Name
	if issuer == "" {
		issuer = ws.Name
	}
	d.Font("B", px(14), docpdf.Ink)
	d.P.SetXY(issuerX, y0)
	d.P.MultiCell(issuerW, docpdf.LineH(px(14), 1.25), issuer, "", "L", false)
	d.Font("", px(11), docpdf.Ink2)
	for _, line := range pdfPartyLines(q.Issuer, l) {
		d.P.SetX(issuerX)
		d.P.MultiCell(issuerW, docpdf.LineH(px(11), 1.35), line, "", "L", false)
	}
	leftEnd := d.P.GetY()

	rx := left + width - rightW
	d.Font("B", px(22), d.Brand)
	d.P.SetXY(rx, y0)
	d.P.CellFormat(rightW, docpdf.LineH(px(22), 1.1), l.docTitle, "", 2, "R", false, 0, "")
	d.Gap(1.5)
	meta := []string{fmt.Sprintf("%s: %s", l.number, q.Number), fmt.Sprintf("%s: %s", l.date, docexport.FormatDate(q.IssueDate))}
	if q.ValidUntil != "" {
		meta = append(meta, fmt.Sprintf("%s: %s", l.validUntil, docexport.FormatDate(q.ValidUntil)))
	}
	d.Font("", px(11), docpdf.Ink2)
	for i, m := range meta {
		if i == 0 {
			d.Font("B", px(11), docpdf.Ink)
		} else {
			d.Font("", px(11), docpdf.Ink2)
		}
		d.P.SetX(rx)
		d.P.CellFormat(rightW, docpdf.LineH(px(11), 1.4), m, "", 2, "R", false, 0, "")
	}
	y := max(leftEnd, d.P.GetY(), y0+15) + 3
	d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
	d.P.Rect(left, y, width, 0.5, "F")
	d.P.SetXY(left, y+4)

	// Customer (left) and project/module (right).
	y = d.P.GetY()
	cw := width * 1.4 / 2.4
	label := func(x, w float64, text string) {
		d.Font("B", px(9), d.Brand)
		d.P.SetX(x)
		d.P.CellFormat(w, docpdf.LineH(px(9), 1.4), strings.ToUpper(text), "", 2, "L", false, 0, "")
	}
	d.P.SetY(y)
	label(left, cw, l.to)
	d.Font("B", px(13), docpdf.Ink)
	d.P.SetX(left)
	name := q.Customer.Name
	if name == "" {
		name = "—"
	}
	d.P.MultiCell(cw-6, docpdf.LineH(px(13), 1.3), name, "", "L", false)
	d.Font("", px(11), docpdf.Ink2)
	for _, line := range pdfPartyLines(q.Customer, l) {
		d.P.SetX(left)
		d.P.MultiCell(cw-6, docpdf.LineH(px(11), 1.35), line, "", "L", false)
	}
	custEnd := d.P.GetY()
	px2 := left + cw
	d.P.SetY(y)
	label(px2, width-cw, l.project)
	d.Font("B", px(11.5), docpdf.Ink)
	d.P.SetX(px2)
	d.P.MultiCell(width-cw, docpdf.LineH(px(11.5), 1.35), q.ProjectName, "", "L", false)
	d.Gap(1)
	label(px2, width-cw, l.module)
	d.Font("B", px(11.5), docpdf.Ink)
	d.P.SetX(px2)
	d.P.MultiCell(width-cw, docpdf.LineH(px(11.5), 1.35), q.ModuleName, "", "L", false)
	y = max(custEnd, d.P.GetY()) + 3
	d.HLine(left, left+width, y, 0.25, docpdf.Line)
	d.P.SetXY(left, y+4)

	if q.Title != "" {
		d.Font("B", px(16), docpdf.Ink)
		d.Text(left, width, q.Title, "L", docpdf.LineH(px(16), 1.3))
		d.Gap(3)
	}

	// Items.
	discount := hasLineDiscount(q)
	cols := []docpdf.Col{{Head: l.no, W: 10, Align: "C"}, {Head: l.item}, {Head: l.unit, W: 13}, {Head: l.qty, W: 15, Align: "R"}, {Head: l.unitPrice, W: 30, Align: "R"}}
	if discount {
		cols = append(cols, docpdf.Col{Head: l.discount, W: 13, Align: "R"})
	}
	cols = append(cols, docpdf.Col{Head: l.amount, W: 34, Align: "R"})
	muted := docpdf.Muted
	rows := make([][]docpdf.Cell, 0, len(q.Items))
	for i, it := range q.Items {
		row := []docpdf.Cell{
			{Text: strconv.Itoa(i + 1), Color: &muted},
			{Text: it.Name, Sub: it.Description, Bold: true},
			docpdf.C(it.Unit),
			docpdf.C(formatNumber(it.Quantity, 3, false, lang)),
			docpdf.C(money(it.UnitPrice)),
		}
		if discount {
			pct := ""
			if it.DiscountPercent > 0 {
				pct = formatNumber(it.DiscountPercent, 2, false, lang) + "%"
			}
			row = append(row, docpdf.C(pct))
		}
		row = append(row, docpdf.C(money(it.LineTotal)))
		rows = append(rows, row)
	}
	d.Table(cols, rows, docpdf.ModernTable())

	// Totals box (80 mm, right).
	type total struct {
		label, value string
	}
	totals := []total{{l.subtotal, money(q.Subtotal)}}
	if q.DiscountTotal > 0 {
		lbl := l.discountTotal
		if q.DiscountType == "percent" {
			lbl += fmt.Sprintf(" (%s%%)", formatNumber(q.DiscountPercent, 2, false, lang))
		}
		totals = append(totals, total{lbl, "− " + money(q.DiscountTotal)})
	}
	totals = append(totals, total{fmt.Sprintf("%s (%s%%)", l.vat, formatNumber(q.VatRate, 2, false, lang)), money(q.TaxTotal)})
	rowH := docpdf.LineH(px(12), 1.6)
	d.Ensure(float64(len(totals)+1)*rowH + 12)
	d.Gap(4)
	bx, bw := left+width-80, 80.0
	for _, t := range totals {
		y := d.P.GetY()
		d.Font("", px(12), docpdf.Ink2)
		d.P.SetXY(bx+3, y)
		d.P.CellFormat(bw/2, rowH, t.label, "", 0, "L", false, 0, "")
		d.Font("", px(12), docpdf.Ink)
		d.P.SetXY(bx+bw/2, y)
		d.P.CellFormat(bw/2-3, rowH, t.value, "", 0, "R", false, 0, "")
		d.HLine(bx, bx+bw, y+rowH, 0.2, docpdf.Line)
		d.P.SetXY(left, y+rowH)
	}
	y = d.P.GetY() + 1
	gh := docpdf.LineH(px(14), 1.7)
	d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
	d.P.Rect(bx, y, bw, gh, "F")
	d.Font("B", px(12), docpdf.White)
	d.P.SetXY(bx+3, y)
	d.P.CellFormat(bw/2, gh, l.grand, "", 0, "L", false, 0, "")
	d.Font("B", px(14), docpdf.White)
	d.P.SetXY(bx+bw/2, y)
	d.P.CellFormat(bw/2-3, gh, money(q.GrandTotal), "", 0, "R", false, 0, "")
	d.P.SetXY(left, y+gh+3)

	if q.AmountInWords != "" {
		d.Runs(docpdf.LineH(px(11.5), 1.4), 0,
			docpdf.Run{Text: l.inWords + ": ", Style: "B", Size: px(11.5), Color: docpdf.Ink},
			docpdf.Run{Text: strings.TrimSuffix(q.AmountInWords, ".") + ".", Style: "I", Size: px(11.5), Color: docpdf.Ink2})
	}

	// Notes, terms, bank: a brand bar on the left of each block.
	for _, b := range []struct{ title, body string }{{l.notes, q.Notes}, {l.terms, q.Terms}, {l.bank, q.Issuer.BankInfo}} {
		if strings.TrimSpace(b.body) == "" {
			continue
		}
		d.Font("", px(11.5), docpdf.Ink2)
		lines := d.Wrap(b.body, width-4)
		lh := docpdf.LineH(px(11.5), 1.5)
		d.Ensure(docpdf.LineH(px(9.5), 1.4) + lh*float64(min(len(lines), 3)) + 4)
		d.Gap(4)
		y := d.P.GetY()
		d.Font("B", px(9.5), d.Brand)
		d.P.SetX(left + 4)
		d.P.CellFormat(width-4, docpdf.LineH(px(9.5), 1.4), strings.ToUpper(b.title), "", 2, "L", false, 0, "")
		d.Font("", px(11.5), docpdf.Ink2)
		d.Text(left+4, width-4, b.body, "L", lh)
		d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
		if d.P.GetY() > y {
			d.P.Rect(left, y, 0.6, d.P.GetY()-y, "F")
		}
	}

	slot := docpdf.Slot{Title: l.preparer, Hint: l.signHint}
	if sig := preparerSig(q); sig != nil {
		slot.Name, slot.SignedAt = sig.Name, sig.SignedAt
	}
	d.ModernSignatures([]docpdf.Slot{slot})
	return d.Bytes()
}

// adminQuotePDF mirrors AdminQuotePrint: a Decree 30 document in Vietnamese.
func (h *Handler) adminQuotePDF(ctx context.Context, wsID string, q *QuoteResponse, o docpdf.Options) ([]byte, error) {
	d, ws, err := h.pdf.Begin(ctx, wsID, docpdf.AdminMargins())
	if err != nil {
		return nil, err
	}
	lang := quoteLang(q)
	l := quotePDFLabels["vi"]
	money := func(v int64) string { return formatMoney(v, q.Currency, q.MinorUnits, lang) }
	d.Start()
	left, width := d.Left(), d.Width()
	lh := docpdf.LineH(13, 1.35)

	a := h.adminSetup(ctx, wsID, ws.Name, q, o)
	org := a.org
	d.AdminHeader(org, q.Number, true, a.issued, a.place)

	d.Gap(8)
	d.Centered("BẢNG BÁO GIÁ", "B", 14, docpdf.Black, docpdf.LineH(14, 1.3))
	if q.Title != "" {
		d.Centered("V/v: "+q.Title, "B", 13, docpdf.Black, lh)
	}
	d.Gap(6)

	black := func(text, style string) docpdf.Run {
		return docpdf.Run{Text: text, Style: style, Size: 13, Color: docpdf.Black}
	}
	plain := func(text string) {
		d.Font("", 13, docpdf.Black)
		d.Text(left, width, text, "J", lh)
	}
	customer := q.Customer.Name
	if customer == "" {
		customer = docpdf.Blank
	}
	d.Runs(lh, 0, black("Kính gửi: ", ""), black(customer, "B"))
	if q.Customer.Address != "" {
		plain(q.Customer.Address)
	}
	if line := docexport.Join(" · ", prefixed(l.taxCode, q.Customer.TaxCode), prefixed(l.tel, q.Customer.Phone), q.Customer.Email); line != "" {
		plain(line)
	}
	d.Gap(2)
	d.Runs(lh, 0, black("Đơn vị báo giá: ", ""), black(org, "B"))
	if q.Issuer.Address != "" {
		plain(q.Issuer.Address)
	}
	if line := docexport.Join(" · ", prefixed(l.taxCode, q.Issuer.TaxCode), prefixed(l.tel, q.Issuer.Phone), q.Issuer.Email); line != "" {
		plain(line)
	}
	if line := docexport.Join(" · ", prefixed(l.contact, q.Issuer.ContactName), q.Issuer.Website); line != "" {
		plain(line)
	}
	d.Gap(3)
	d.Runs(lh, 10, black(fmt.Sprintf("%s trân trọng gửi tới Quý khách bảng báo giá cho hạng mục “%s” thuộc dự án “%s” như sau:", org, q.ModuleName, q.ProjectName), ""))
	d.Gap(3)

	discount := hasLineDiscount(q)
	cols := []docpdf.Col{{Head: l.no, W: 12, Align: "C"}, {Head: l.item}, {Head: l.unit, W: 14, Align: "C"}, {Head: l.qty, W: 14, Align: "R"}, {Head: l.unitPrice, W: 28, Align: "R"}}
	if discount {
		cols = append(cols, docpdf.Col{Head: l.discount, W: 12, Align: "R"})
	}
	cols = append(cols, docpdf.Col{Head: l.amount, W: 32, Align: "R"})
	n := len(cols)
	var rows [][]docpdf.Cell
	for i, it := range q.Items {
		row := []docpdf.Cell{docpdf.C(strconv.Itoa(i + 1)), {Text: it.Name, Sub: it.Description}, docpdf.C(it.Unit),
			docpdf.C(formatNumber(it.Quantity, 3, false, lang)), docpdf.C(money(it.UnitPrice))}
		if discount {
			pct := ""
			if it.DiscountPercent > 0 {
				pct = formatNumber(it.DiscountPercent, 2, false, lang) + "%"
			}
			row = append(row, docpdf.C(pct))
		}
		rows = append(rows, append(row, docpdf.C(money(it.LineTotal))))
	}
	totalRow := func(label, value string, bold bool) []docpdf.Cell {
		return []docpdf.Cell{{Text: label, Span: n - 1, Align: "R", Bold: bold}, {Text: value, Bold: bold}}
	}
	rows = append(rows, totalRow(l.subtotal, money(q.Subtotal), false))
	if q.DiscountTotal > 0 {
		lbl := l.discountTotal
		if q.DiscountType == "percent" {
			lbl += fmt.Sprintf(" (%s%%)", formatNumber(q.DiscountPercent, 2, false, lang))
		}
		rows = append(rows, totalRow(lbl, "− "+money(q.DiscountTotal), false))
	}
	rows = append(rows, totalRow(fmt.Sprintf("%s (%s%%)", l.vat, formatNumber(q.VatRate, 2, false, lang)), money(q.TaxTotal), false))
	rows = append(rows, totalRow(strings.ToUpper(l.grand), money(q.GrandTotal), true))
	d.Table(cols, rows, docpdf.AdminTable())

	if q.AmountInWords != "" {
		d.Gap(2)
		d.Runs(lh, 0, black(l.inWords+": ", "B"), black(strings.TrimSuffix(q.AmountInWords, ".")+".", "I"))
	}
	d.Gap(3)
	if q.ValidUntil != "" {
		d.Runs(lh, 10, black(fmt.Sprintf("Báo giá có hiệu lực đến hết ngày %s.", docexport.FormatDate(q.ValidUntil)), ""))
	}
	for _, b := range []struct{ title, body string }{{l.notes, q.Notes}, {l.terms, q.Terms}, {l.bank, q.Issuer.BankInfo}} {
		if strings.TrimSpace(b.body) == "" {
			continue
		}
		d.Ensure(lh * 2)
		d.Runs(lh, 0, black(b.title+":", "B"))
		plain(b.body)
		d.Gap(1)
	}
	d.Runs(lh, 10, black("Rất mong nhận được sự hợp tác của Quý khách. Trân trọng!", ""))

	// The quote is issued under the director's signature and seal; the preparer signs alongside.
	// One block when the director prepared it (default: the workspace owner signed).
	sig, title, single := a.sig, a.title, a.single
	signed := docpdf.Slot{Title: title, Hint: docpdf.HintSeal}
	preparer := docpdf.Slot{Title: "NGƯỜI LẬP BÁO GIÁ", Hint: docpdf.HintSign}
	if sig != nil {
		signed.Name, signed.SignedAt = sig.Name, sig.SignedAt
		preparer.Name, preparer.SignedAt = sig.Name, sig.SignedAt
	}
	if single {
		d.AdminSignatures([]docpdf.Slot{signed}, o.Wet, d.Recipients)
	} else {
		d.AdminSignatures([]docpdf.Slot{preparer, {Title: title, Hint: docpdf.HintSeal, Blank: true}}, o.Wet, nil)
		d.Gap(4)
		d.Ensure(20)
		d.Recipients(left, d.P.GetY(), width/2)
	}
	return d.Bytes()
}

// adminQuote holds what the administrative PDF and Excel decide the same way.
type adminQuote struct {
	org, place string
	issued     time.Time
	sig        *docsign.Signature // the preparer's signature, if signed
	title      string             // approver title
	single     bool               // one signature block (the director prepared the quote)
}

func (h *Handler) adminSetup(ctx context.Context, wsID, wsName string, q *QuoteResponse, o docpdf.Options) adminQuote {
	a := adminQuote{org: q.Issuer.Name, place: o.Place, title: o.Signer, sig: preparerSig(q)}
	if a.org == "" {
		a.org = wsName
	}
	if a.place == "" {
		a.place = docpdf.PlaceFromAddress(q.Issuer.Address)
	}
	var err error
	if a.issued, err = time.Parse("2006-01-02", q.IssueDate); err != nil {
		a.issued = time.Now()
	}
	if a.title == "" {
		a.title = "GIÁM ĐỐC"
	}
	a.single = o.Merge == "1"
	if o.Merge == "" && a.sig != nil {
		a.single = h.isHolder(ctx, wsID, a.sig.UserID)
	}
	return a
}

func (h *Handler) isHolder(ctx context.Context, wsID, userID string) bool {
	wid, err1 := pgutil.ParseUUID(wsID)
	uid, err2 := pgutil.ParseUUID(userID)
	if err1 != nil || err2 != nil {
		return false
	}
	g, err := h.svc.repo.GetCallerAccess(ctx, wid, uid)
	return err == nil && g.IsHolder()
}

func prefixed(label, v string) string {
	if v == "" {
		return ""
	}
	return label + ": " + v
}
