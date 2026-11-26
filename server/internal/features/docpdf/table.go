package docpdf

import "strings"

// Col is a table column. W 0 shares the width left by the fixed columns.
type Col struct {
	Head  string
	W     float64
	Align string // "L" (default), "C", "R"
}

// Cell is one table cell: main text, an optional smaller second line (descriptions, notes),
// and how many columns it spans.
type Cell struct {
	Text  string
	Sub   string
	Bold  bool
	Color *RGB
	Align string // overrides the column's
	Span  int
}

// C is a plain cell.
func C(text string) Cell { return Cell{Text: text} }

// TableStyle picks the look of the two print templates.
type TableStyle struct {
	// Admin: black borders on every cell, bold centred header (administrative documents).
	// Otherwise: brand-coloured header with white capitals, thin row separators (modern).
	Admin    bool
	Size     float64 // body font size (pt)
	SubSize  float64
	HeadSize float64
	PadX     float64
	PadY     float64
	SubStyle string // style of Sub ("I" for admin, "" for modern)
	SubColor RGB
}

// AdminTable and ModernTable are the two table looks.
func AdminTable() TableStyle {
	return TableStyle{Admin: true, Size: 12, SubSize: 11, HeadSize: 12, PadX: 1.5, PadY: 1, SubStyle: "I", SubColor: Black}
}

func ModernTable() TableStyle {
	return TableStyle{Size: 9, SubSize: 8, HeadSize: 7, PadX: 2, PadY: 1.4, SubColor: Ink2}
}

// Table draws a table at the current position across the page width. Rows never split: a
// row that does not fit goes to the next page, under a repeated header.
func (d *Doc) Table(cols []Col, rows [][]Cell, st TableStyle) {
	widths := make([]float64, len(cols))
	fixed, flex := 0.0, 0
	for i, c := range cols {
		widths[i] = c.W
		fixed += c.W
		if c.W == 0 {
			flex++
		}
	}
	if flex > 0 {
		share := (d.Width() - fixed) / float64(flex)
		for i := range widths {
			if widths[i] == 0 {
				widths[i] = share
			}
		}
	}
	lh := LineH(st.Size, 1.25)
	subLH := LineH(st.SubSize, 1.25)

	header := func() {
		d.Font("B", st.HeadSize, White)
		h := 0.0
		lines := make([][]string, len(cols))
		for i, c := range cols {
			head := c.Head
			if !st.Admin {
				head = strings.ToUpper(head)
			} else {
				d.Font("B", st.HeadSize, Black)
			}
			lines[i] = d.Wrap(head, widths[i]-2*st.PadX)
			h = max(h, float64(len(lines[i]))*LineH(st.HeadSize, 1.2)+2*st.PadY)
		}
		d.Ensure(h + lh + 2*st.PadY)
		x, y := d.Left(), d.P.GetY()
		if !st.Admin {
			d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
			d.P.Rect(x, y, d.Width(), h, "F")
		}
		for i, c := range cols {
			align := c.Align
			if st.Admin {
				align = "C"
				d.box(x, y, widths[i], h)
			}
			if align == "" {
				align = "L"
			}
			if st.Admin {
				d.Font("B", st.HeadSize, Black)
			} else {
				d.Font("B", st.HeadSize, White)
			}
			ty := y + (h-float64(len(lines[i]))*LineH(st.HeadSize, 1.2))/2
			for _, l := range lines[i] {
				d.P.SetXY(x+st.PadX, ty)
				d.P.CellFormat(widths[i]-2*st.PadX, LineH(st.HeadSize, 1.2), l, "", 0, align, false, 0, "")
				ty += LineH(st.HeadSize, 1.2)
			}
			x += widths[i]
		}
		d.P.SetXY(d.Left(), y+h)
	}

	header()
	for _, row := range rows {
		// Lay out the cells (with spans) and measure the row.
		type laid struct {
			x, w      float64
			main, sub []string
			cell      Cell
			align     string
		}
		var cells []laid
		x, ci := d.Left(), 0
		h := 0.0
		for _, c := range row {
			span := max(c.Span, 1)
			w := 0.0
			for k := 0; k < span && ci+k < len(widths); k++ {
				w += widths[ci+k]
			}
			align := c.Align
			if align == "" && ci < len(cols) {
				align = cols[ci].Align
			}
			if align == "" {
				align = "L"
			}
			style := ""
			if c.Bold {
				style = "B"
			}
			d.Font(style, st.Size, Ink)
			main := d.Wrap(c.Text, w-2*st.PadX)
			if c.Text == "" {
				main = nil
			}
			var sub []string
			if c.Sub != "" {
				d.Font(st.SubStyle, st.SubSize, st.SubColor)
				sub = d.Wrap(c.Sub, w-2*st.PadX)
			}
			ch := float64(len(main))*lh + float64(len(sub))*subLH + 2*st.PadY
			h = max(h, ch)
			cells = append(cells, laid{x: x, w: w, main: main, sub: sub, cell: c, align: align})
			x += w
			ci += span
		}
		h = max(h, lh+2*st.PadY)
		if d.P.GetY()+h > d.Bottom() {
			d.P.AddPage()
			header()
		}
		y := d.P.GetY()
		for _, c := range cells {
			if st.Admin {
				d.box(c.x, y, c.w, h)
			}
			style := ""
			if c.cell.Bold {
				style = "B"
			}
			color := Ink
			if st.Admin {
				color = Black
			}
			if c.cell.Color != nil {
				color = *c.cell.Color
			}
			ty := y + st.PadY
			d.Font(style, st.Size, color)
			for _, l := range c.main {
				d.P.SetXY(c.x+st.PadX, ty)
				d.P.CellFormat(c.w-2*st.PadX, lh, l, "", 0, c.align, false, 0, "")
				ty += lh
			}
			d.Font(st.SubStyle, st.SubSize, st.SubColor)
			for _, l := range c.sub {
				d.P.SetXY(c.x+st.PadX, ty)
				d.P.CellFormat(c.w-2*st.PadX, subLH, l, "", 0, c.align, false, 0, "")
				ty += subLH
			}
		}
		if !st.Admin {
			d.HLine(d.Left(), d.Left()+d.Width(), y+h, 0.2, Line)
		}
		d.P.SetXY(d.Left(), y+h)
	}
	d.Font("", 12, Ink)
}

func (d *Doc) box(x, y, w, h float64) {
	d.P.SetDrawColor(0, 0, 0)
	d.P.SetLineWidth(0.2)
	d.P.Rect(x, y, w, h, "D")
}
