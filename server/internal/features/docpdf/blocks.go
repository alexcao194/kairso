package docpdf

import (
	"fmt"
	"strings"
	"time"
)

// Px converts a CSS pixel size of the print pages to points.
func Px(v float64) float64 { return v * 0.75 }

// AdminMargins are the Decree 30/2020 margins: top/bottom 20 mm, left 30 mm, right 15 mm.
func AdminMargins() Margins { return Margins{Top: 20, Right: 15, Bottom: 20, Left: 30} }

// ModernMargins leave room like the modern print page (14 mm sides).
func ModernMargins() Margins { return Margins{Top: 11, Right: 14, Bottom: 12, Left: 14} }

// Strings of administrative documents (always Vietnamese).
const (
	Nation   = "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"
	Motto    = "Độc lập - Tự do - Hạnh phúc"
	HintSign = "(Ký, ghi rõ họ tên)"
	HintSeal = "(Ký, ghi rõ họ tên, đóng dấu)"
	Blank    = "……………………"
)

// PlaceFromAddress is the last part of an address, the default place of the date line.
func PlaceFromAddress(address string) string {
	parts := strings.Split(address, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if p := strings.TrimSpace(parts[i]); p != "" {
			return p
		}
	}
	return ""
}

// DateLine is "<place>, ngày DD tháng MM năm YYYY".
func DateLine(place string, t time.Time) string {
	if place == "" {
		place = "……………"
	}
	return fmt.Sprintf("%s, ngày %02d tháng %02d năm %d", place, t.Day(), int(t.Month()), t.Year())
}

// AdminHeader draws the issuing body (and number) left, the national motto and date right.
func (d *Doc) AdminHeader(org, number string, withNumber bool, date time.Time, place string) {
	y0 := d.P.GetY()
	lw := d.Width() * 0.42
	rw := d.Width() - lw
	lx, rx := d.Left(), d.Left()+lw

	d.Font("B", 12, Black)
	d.P.SetXY(lx, y0)
	d.P.MultiCell(lw, LineH(12, 1.15), strings.ToUpper(org), "", "C", false)
	y := d.P.GetY() + 0.6
	d.HLine(lx+lw/3, lx+2*lw/3, y, 0.3, Black)
	if withNumber {
		if number == "" {
			number = "……/……"
		}
		d.Font("", 13, Black)
		d.P.SetXY(lx, y+2)
		d.P.MultiCell(lw, LineH(13, 1.3), "Số: "+number, "", "C", false)
	}
	leftEnd := d.P.GetY()

	d.Font("B", 12, Black)
	d.P.SetXY(rx, y0)
	d.P.MultiCell(rw, LineH(12, 1.15), Nation, "", "C", false)
	d.Font("B", 13, Black)
	d.P.SetX(rx)
	d.P.MultiCell(rw, LineH(13, 1.15), Motto, "", "C", false)
	y = d.P.GetY() + 0.6
	d.HLine(rx+rw*0.225, rx+rw*0.775, y, 0.3, Black)
	d.Font("I", 13, Black)
	d.P.SetXY(rx, y+2)
	d.P.MultiCell(rw, LineH(13, 1.3), DateLine(place, date), "", "C", false)

	d.P.SetXY(d.Left(), max(leftEnd, d.P.GetY()))
	d.Font("", 13, Black)
}

// Slot is one signature column.
type Slot struct {
	Title, Hint string
	Name        string    // signer's name ("" when not signed)
	SignedAt    time.Time // zero when not signed
	Blank       bool      // always left empty for a wet signature and seal
}

func (s Slot) signed() bool { return !s.SignedAt.IsZero() }

// AdminSignatures draws signature columns (68 mm wide). With left set, it is drawn in the space
// left of the columns (the "Nơi nhận" block); the columns then sit at the right. In wet mode the
// stamp boxes stay empty.
func (d *Doc) AdminSignatures(slots []Slot, wet bool, left func(x, y, w float64)) {
	const colW, stampH = 68.0, 30.0
	h := LineH(13, 1.3)*2 + LineH(12, 1.3) + stampH + 4
	d.Ensure(h + 8)
	d.Gap(8)
	y := d.P.GetY()
	xs := make([]float64, len(slots))
	right := d.Left() + d.Width()
	switch {
	case left != nil || len(slots) == 1:
		for i := range slots {
			xs[len(slots)-1-i] = right - float64(i+1)*colW - float64(i)*6
		}
	default: // spread across the page (justify-between)
		gap := (d.Width() - float64(len(slots))*colW) / float64(max(len(slots)-1, 1))
		for i := range slots {
			xs[i] = d.Left() + float64(i)*(colW+gap)
		}
	}
	end := y
	for i, s := range slots {
		x := xs[i]
		d.Font("B", 13, Black)
		d.P.SetXY(x, y)
		d.P.MultiCell(colW, LineH(13, 1.3), s.Title, "", "C", false)
		d.Font("I", 12, Black)
		d.P.SetX(x)
		d.P.MultiCell(colW, LineH(12, 1.3), s.Hint, "", "C", false)
		sy := d.P.GetY()
		if !wet && !s.Blank && s.signed() {
			d.Stamp(x+colW/2, sy+stampH/2, s.Name, s.SignedAt.Format("02/01/2006"), d.Brand)
		}
		if !s.Blank && s.signed() {
			d.Font("B", 13, Black)
			d.P.SetXY(x, sy+stampH)
			d.P.MultiCell(colW, LineH(13, 1.3), s.Name, "", "C", false)
		}
		end = max(end, sy+stampH+LineH(13, 1.3))
	}
	if left != nil {
		left(d.Left(), y, xs[0]-d.Left()-6)
		end = max(end, d.P.GetY())
	}
	d.P.SetXY(d.Left(), end)
	d.Font("", 13, Black)
}

// Recipients draws the "Nơi nhận" block at (x, y), w wide.
func (d *Doc) Recipients(x, y, w float64) {
	d.Font("BI", 11, Black)
	d.P.SetXY(x, y)
	d.P.MultiCell(w, LineH(11, 1.3), "Nơi nhận:", "", "L", false)
	d.Font("", 11, Black)
	for _, l := range []string{"- Như trên;", "- Lưu: VT."} {
		d.P.SetX(x)
		d.P.MultiCell(w, LineH(11, 1.3), l, "", "L", false)
	}
}

// ModernSignatures draws the signature row of the modern template: right-aligned 58 mm
// columns, stamped when signed.
func (d *Doc) ModernSignatures(slots []Slot) {
	const colW, gap, stampH = 58.0, 10.0, 24.0
	h := LineH(Px(12), 1.4) + LineH(Px(10.5), 1.4) + stampH
	d.Ensure(h + 5)
	d.Gap(5)
	y := d.P.GetY()
	x := d.Left() + d.Width() - float64(len(slots))*colW - float64(len(slots)-1)*gap
	for _, s := range slots {
		d.Font("B", Px(12), Ink)
		d.P.SetXY(x, y)
		d.P.MultiCell(colW, LineH(Px(12), 1.4), s.Title, "", "C", false)
		d.Font("I", Px(10.5), Muted)
		d.P.SetX(x)
		d.P.MultiCell(colW, LineH(Px(10.5), 1.4), s.Hint, "", "C", false)
		if s.signed() {
			d.Stamp(x+colW/2, d.P.GetY()+stampH/2, s.Name, s.SignedAt.Format("02/01/2006"), d.Brand)
		}
		x += colW + gap
	}
	d.P.SetXY(d.Left(), y+h)
}

// Logo draws the workspace logo in a bordered square (modern header).
func (d *Doc) Logo(x, y, size float64, has bool) {
	d.P.SetDrawColor(Line.R, Line.G, Line.B)
	d.P.SetLineWidth(0.25)
	d.roundedRect(x, y, size, size, 1.5)
	if has {
		pad := size * 0.1
		d.Image(LogoImage, x+pad, y+pad, size-2*pad, size-2*pad)
	}
}
