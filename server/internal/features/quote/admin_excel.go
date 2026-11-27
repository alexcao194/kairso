package quote

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docpdf"
)

// moneyNumFmt is an Excel number format that shows amounts like formatMoney (symbol and
// decimals), keeping the cells numeric. Excel applies the reader's own digit grouping.
func moneyNumFmt(currency string, minorUnits int, lang string) string {
	num := "#,##0"
	if minorUnits > 0 {
		num += "." + strings.Repeat("0", minorUnits)
	}
	sym, ok := currencySymbols[currency]
	if !ok {
		sym = [2]string{currency, currency + " "}
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, "") + `"` }
	pos := num + q(" "+sym[0])
	if lang == "en" {
		pos = q(sym[1]) + num
	}
	return pos + ";" + q("− ") + pos
}

// adminQuoteExcel mirrors adminQuotePDF (and AdminQuotePrint) cell by cell: A4 with the
// Decree 30 margins, Times New Roman, the same blocks at the same positions, so the workbook
// prints like the PDF while amounts stay numbers.
func (h *Handler) adminQuoteExcel(ctx context.Context, wsID string, q *QuoteResponse, o docpdf.Options) (*ExportFile, error) {
	ws := h.pdf.Look(ctx, wsID)
	lang := quoteLang(q)
	l := quotePDFLabels["vi"]
	moneyFmt := moneyNumFmt(q.Currency, q.MinorUnits, lang)
	money := func(v int64) float64 { return float64(v) / math.Pow10(q.MinorUnits) }
	brand := fmt.Sprintf("%02X%02X%02X", ws.Brand.R, ws.Brand.G, ws.Brand.B)

	a := h.adminSetup(ctx, wsID, ws.Name, q, o)
	org, place, issued, sig, title, single := a.org, a.place, a.issued, a.sig, a.title, a.single

	m := docpdf.AdminMargins()
	const W = 210.0 - 30 - 15 // text width
	var measure docexport.Measure
	if mm := h.pdf.Measurer(); mm != nil {
		measure = mm
	}

	// Horizontal layout (mm). Table columns as on the print page.
	discount := hasLineDiscount(q)
	widths := []float64{10, 0, 14, 14, 28}
	if discount {
		widths = append(widths, 12)
	}
	widths = append(widths, 32)
	fixed := 0.0
	for _, w := range widths {
		fixed += w
	}
	widths[1] = W - fixed
	colX := make([]float64, len(widths)+1)
	for i, w := range widths {
		colX[i+1] = colX[i] + w
	}
	lw := W * 0.42 // header: issuing body | national motto
	rw := W - lw
	orgRule := [2]float64{lw / 3, 2 * lw / 3}
	mottoRule := [2]float64{lw + rw*0.225, lw + rw*0.775}
	const slotW, stampH = 68.0, 30.0
	type slot struct {
		docpdf.Slot
		x, sw float64
		stamp bool
	}
	signed := docpdf.Slot{Title: title, Hint: docpdf.HintSeal}
	preparer := docpdf.Slot{Title: "NGƯỜI LẬP BÁO GIÁ", Hint: docpdf.HintSign}
	if sig != nil {
		signed.Name, signed.SignedAt = sig.Name, sig.SignedAt
		preparer.Name, preparer.SignedAt = sig.Name, sig.SignedAt
	}
	var slots []slot
	if single {
		slots = []slot{{Slot: signed, x: W - slotW}}
	} else {
		slots = []slot{{Slot: preparer, x: 0}, {Slot: docpdf.Slot{Title: title, Hint: docpdf.HintSeal, Blank: true}, x: W - slotW}}
	}

	edges := append([]float64{lw, orgRule[0], orgRule[1], mottoRule[0], mottoRule[1], W / 2}, colX...)
	// The stamp is as wide as the signer's name, like on the print page.
	probe, err := docexport.NewSheet("x", W, nil, measure, docexport.PageMargins{})
	if err != nil {
		return nil, err
	}
	for i := range slots {
		s := &slots[i]
		edges = append(edges, s.x, s.x+slotW)
		if !o.Wet && !s.Blank && !s.SignedAt.IsZero() {
			nw := probe.TextWidth(strings.ToUpper(s.Name), "B", 10)
			dw := probe.TextWidth(s.SignedAt.Format("02/01/2006"), "", 8)
			s.sw = math.Min(math.Max(math.Max(nw, dw)+12, 30), slotW-4)
			s.stamp = true
			cx := s.x + slotW/2
			edges = append(edges, cx-s.sw/2, cx+s.sw/2)
		}
	}

	sh, err := docexport.NewSheet(q.Number, W, edges, measure, docexport.PageMargins{Top: m.Top, Right: m.Right, Bottom: m.Bottom, Left: m.Left})
	if err != nil {
		return nil, err
	}
	lineH := func(pt float64) float64 { return pt * 1.32 }
	height := func(text, style string, pt, w float64) float64 {
		return float64(sh.Lines(text, style, pt, w)) * lineH(pt)
	}
	indent := ""
	if spw := sh.TextWidth(" ", "", 13); spw > 0 {
		indent = strings.Repeat(" ", int(math.Round(10/spw)))
	}
	// para writes a full-width paragraph of runs (13 pt), first line indented 1 cm when asked.
	para := func(runs []docexport.Run, firstIndent bool, h string) {
		if firstIndent && len(runs) > 0 {
			runs = append([]docexport.Run{{Text: indent}}, runs...)
		}
		var all strings.Builder
		style := ""
		for _, r := range runs {
			all.WriteString(r.Text)
			if r.Bold {
				style = "B"
			}
		}
		r := sh.Row(height(all.String(), style, 13, W))
		sh.Rich(r, r, 0, W, runs, docexport.Style{Size: 13, H: h})
	}
	text := func(s string) { para([]docexport.Run{{Text: s}}, false, "left") }

	// Header.
	orgUp := strings.ToUpper(org)
	orgLines := sh.Lines(orgUp, "B", 12, lw)
	r1 := sh.Row(lineH(12))
	r2 := sh.Row(lineH(13))
	bold12 := docexport.Style{Size: 12, Bold: true, H: "center", V: "top"}
	if orgLines > 1 {
		sh.Put(r1, r2, 0, lw, orgUp, bold12)
		sh.Grow(r2, float64(orgLines)*lineH(12)-lineH(12))
	} else {
		sh.Put(r1, r1, 0, lw, orgUp, bold12)
	}
	sh.Put(r1, r1, lw, W, docpdf.Nation, bold12)
	sh.Put(r2, r2, lw, W, docpdf.Motto, docexport.Style{Size: 13, Bold: true, H: "center", V: "top"})
	r3 := sh.Row(docexport.PtMM(2.5)) // thin row carrying the header rules
	sh.Put(r3, r3, mottoRule[0], mottoRule[1], nil, docexport.Style{Rule: "T"})
	if orgLines > 1 {
		sh.Put(r3, r3, orgRule[0], orgRule[1], nil, docexport.Style{Rule: "T"})
	} else {
		sh.Put(r2, r2, orgRule[0], orgRule[1], nil, docexport.Style{Rule: "T", V: "top"})
	}
	number := q.Number
	if number == "" {
		number = "……/……"
	}
	r4 := sh.Row(lineH(13))
	sh.Put(r4, r4, 0, lw, "Số: "+number, docexport.Style{Size: 13, H: "center"})
	sh.Put(r4, r4, lw, W, docpdf.DateLine(place, issued), docexport.Style{Size: 13, Italic: true, H: "center"})

	// Title.
	sh.Gap(8)
	r := sh.Row(lineH(14))
	sh.Put(r, r, 0, W, "BẢNG BÁO GIÁ", docexport.Style{Size: 14, Bold: true, H: "center"})
	if q.Title != "" {
		about := "V/v: " + q.Title
		r = sh.Row(height(about, "B", 13, W))
		sh.Put(r, r, 0, W, about, docexport.Style{Size: 13, Bold: true, H: "center"})
	}
	sh.Gap(6)

	// Parties.
	customer := q.Customer.Name
	if customer == "" {
		customer = docpdf.Blank
	}
	para([]docexport.Run{{Text: "Kính gửi: "}, {Text: customer, Bold: true}}, false, "left")
	if q.Customer.Address != "" {
		text(q.Customer.Address)
	}
	if line := docexport.Join(" · ", prefixed(l.taxCode, q.Customer.TaxCode), prefixed(l.tel, q.Customer.Phone), q.Customer.Email); line != "" {
		text(line)
	}
	sh.Gap(2)
	para([]docexport.Run{{Text: "Đơn vị báo giá: "}, {Text: org, Bold: true}}, false, "left")
	if q.Issuer.Address != "" {
		text(q.Issuer.Address)
	}
	if line := docexport.Join(" · ", prefixed(l.taxCode, q.Issuer.TaxCode), prefixed(l.tel, q.Issuer.Phone), q.Issuer.Email); line != "" {
		text(line)
	}
	if line := docexport.Join(" · ", prefixed(l.contact, q.Issuer.ContactName), q.Issuer.Website); line != "" {
		text(line)
	}
	sh.Gap(3)
	para([]docexport.Run{{Text: fmt.Sprintf("%s trân trọng gửi tới Quý khách bảng báo giá cho hạng mục “%s” thuộc dự án “%s” như sau:", org, q.ModuleName, q.ProjectName)}}, true, "justify")
	sh.Gap(3)

	// Items table.
	cell := func(h string) docexport.Style { return docexport.Style{Size: 12, Grid: true, H: h} }
	pad := docexport.PtMM(2)
	heads := []string{l.no, l.item, l.unit, l.qty, l.unitPrice}
	if discount {
		heads = append(heads, l.discount)
	}
	heads = append(heads, l.amount)
	hh := 0.0
	for i, t := range heads {
		hh = math.Max(hh, height(t, "B", 12, widths[i]))
	}
	// The header row is repeated at the top of each page the table continues on.
	header := func() {
		r := sh.Row(hh + pad)
		for i, t := range heads {
			sh.Put(r, r, colX[i], colX[i+1], t, docexport.Style{Size: 12, Bold: true, Grid: true, H: "center"})
		}
	}
	sh.Keep(hh + pad + docexport.PtMM(15))
	header()
	row := func(h float64) int {
		if !sh.Fits(h) {
			sh.Break()
			header()
		}
		return sh.Row(h)
	}
	last := len(widths) - 1
	for i, it := range q.Items {
		h := height(it.Name, "", 12, widths[1])
		if it.Description != "" {
			h += height(it.Description, "I", 11, widths[1])
		}
		h = math.Max(h, height(it.Unit, "", 12, widths[2]))
		r = row(h + pad)
		sh.Put(r, r, colX[0], colX[1], i+1, cell("center"))
		runs := []docexport.Run{{Text: it.Name}}
		if it.Description != "" {
			runs = append(runs, docexport.Run{Text: "\n" + it.Description, Size: 11, Italic: true})
		}
		sh.Rich(r, r, colX[1], colX[2], runs, cell("left"))
		sh.Put(r, r, colX[2], colX[3], it.Unit, cell("center"))
		sh.Put(r, r, colX[3], colX[4], it.Quantity, docexport.Style{Size: 12, Grid: true, H: "right", NumFmt: "General"})
		sh.Put(r, r, colX[4], colX[5], money(it.UnitPrice), docexport.Style{Size: 12, Grid: true, H: "right", NumFmt: moneyFmt})
		if discount {
			pct := ""
			if it.DiscountPercent > 0 {
				pct = formatNumber(it.DiscountPercent, 2, false, lang) + "%"
			}
			sh.Put(r, r, colX[5], colX[6], pct, cell("right"))
		}
		sh.Put(r, r, colX[last], colX[last+1], money(it.LineTotal), docexport.Style{Size: 12, Grid: true, H: "right", NumFmt: moneyFmt})
	}
	total := func(label string, v float64, bold bool) {
		r := row(lineH(12) + pad)
		sh.Put(r, r, 0, colX[last], label, docexport.Style{Size: 12, Bold: bold, Grid: true, H: "right"})
		sh.Put(r, r, colX[last], W, v, docexport.Style{Size: 12, Bold: bold, Grid: true, H: "right", NumFmt: moneyFmt})
	}
	total(l.subtotal, money(q.Subtotal), false)
	if q.DiscountTotal > 0 {
		lbl := l.discountTotal
		if q.DiscountType == "percent" {
			lbl += fmt.Sprintf(" (%s%%)", formatNumber(q.DiscountPercent, 2, false, lang))
		}
		total(lbl, -money(q.DiscountTotal), false)
	}
	total(fmt.Sprintf("%s (%s%%)", l.vat, formatNumber(q.VatRate, 2, false, lang)), money(q.TaxTotal), false)
	total(strings.ToUpper(l.grand), money(q.GrandTotal), true)

	// Closing paragraphs.
	if q.AmountInWords != "" {
		sh.Gap(2)
		para([]docexport.Run{{Text: l.inWords + ": ", Bold: true}, {Text: strings.TrimSuffix(q.AmountInWords, ".") + ".", Italic: true}}, false, "left")
	}
	sh.Gap(3)
	if q.ValidUntil != "" {
		para([]docexport.Run{{Text: fmt.Sprintf("Báo giá có hiệu lực đến hết ngày %s.", docexport.FormatDate(q.ValidUntil))}}, true, "justify")
	}
	for _, b := range []struct{ title, body string }{{l.notes, q.Notes}, {l.terms, q.Terms}, {l.bank, q.Issuer.BankInfo}} {
		if strings.TrimSpace(b.body) == "" {
			continue
		}
		para([]docexport.Run{{Text: b.title + ":", Bold: true}}, false, "left")
		text(strings.ReplaceAll(b.body, "\r\n", "\n"))
		sh.Gap(1)
	}
	para([]docexport.Run{{Text: "Rất mong nhận được sự hợp tác của Quý khách. Trân trọng!"}}, true, "justify")

	// Signatures: title, hint, a 30 mm box (stamp when signed in the app), name.
	block := docexport.PtMM(8+stampH) + lineH(13)*2 + lineH(12)
	if !single {
		block += docexport.PtMM(4) + lineH(11)*3
	}
	sh.Keep(block)
	sh.Gap(8)
	rt := sh.Row(lineH(13))
	rh := sh.Row(lineH(12))
	stampTop := sh.Row(docexport.PtMM((stampH - 11.5) / 2))
	rs1 := sh.Row(docexport.PtMM(6))
	rs2 := sh.Row(docexport.PtMM(5.5))
	sh.Row(docexport.PtMM((stampH - 11.5) / 2))
	rn := sh.Row(lineH(13))
	for _, s := range slots {
		sh.Put(rt, rt, s.x, s.x+slotW, s.Title, docexport.Style{Size: 13, Bold: true, H: "center"})
		sh.Put(rh, rh, s.x, s.x+slotW, s.Hint, docexport.Style{Size: 12, Italic: true, H: "center"})
		if s.stamp {
			cx := s.x + slotW/2
			sh.Put(rs1, rs1, cx-s.sw/2, cx+s.sw/2, strings.ToUpper(s.Name), docexport.Style{Size: 10, Bold: true, H: "center", V: "bottom", Color: brand, Box: "LTR"})
			sh.Put(rs2, rs2, cx-s.sw/2, cx+s.sw/2, s.SignedAt.Format("02/01/2006"), docexport.Style{Size: 8, H: "center", V: "top", Color: brand, Box: "LRB"})
		}
		if !s.Blank && !s.SignedAt.IsZero() {
			sh.Put(rn, rn, s.x, s.x+slotW, s.Name, docexport.Style{Size: 13, Bold: true, H: "center"})
		}
	}
	if single {
		// "Nơi nhận" to the left of the director's signature.
		lx := slots[0].x
		sh.Put(rt, rt, 0, lx, "Nơi nhận:", docexport.Style{Size: 11, Bold: true, Italic: true, V: "top"})
		sh.Put(rh, rh, 0, lx, "- Như trên;", docexport.Style{Size: 11, V: "top"})
		sh.Put(stampTop, stampTop, 0, lx, "- Lưu: VT.", docexport.Style{Size: 11, V: "top"})
	} else {
		sh.Gap(4)
		for _, t := range []struct {
			s  string
			st docexport.Style
		}{{"Nơi nhận:", docexport.Style{Size: 11, Bold: true, Italic: true}}, {"- Như trên;", docexport.Style{Size: 11}}, {"- Lưu: VT.", docexport.Style{Size: 11}}} {
			r := sh.Row(lineH(11))
			sh.Put(r, r, 0, W/2, t.s, t.st)
		}
	}

	data, err := sh.Bytes()
	if err != nil {
		return nil, err
	}
	return &ExportFile{Filename: docexport.SafeFilename(q.Number) + ".xlsx", Data: data}, nil
}
