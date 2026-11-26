// Package docpdf draws A4 documents (quotes, meeting minutes) as PDF on the server, in Times New
// Roman, close to the web app's print pages: text runs, wrapped tables that break across pages,
// signature stamps and the workspace's logo and brand colour.
package docpdf

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

// RGB is a colour.
type RGB struct{ R, G, B int }

// Colours of the print pages (app/globals.css, --doc-*).
var (
	Black        = RGB{0, 0, 0}
	White        = RGB{255, 255, 255}
	Ink          = RGB{0x1F, 0x23, 0x28}
	Ink2         = RGB{0x3D, 0x43, 0x4B}
	Muted        = RGB{0x5F, 0x66, 0x70}
	Line         = RGB{0xEC, 0xE7, 0xE7}
	DefaultBrand = RGB{0xC8, 0x10, 0x2E}
)

// ParseHex reads #RRGGBB, or returns def.
func ParseHex(s string, def RGB) RGB {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return RGB{int(v >> 16 & 0xFF), int(v >> 8 & 0xFF), int(v & 0xFF)}
}

// Tint mixes c with white; f=0 is c, f=1 is white.
func (c RGB) Tint(f float64) RGB {
	mix := func(v int) int { return int(math.Round(float64(v) + (255-float64(v))*f)) }
	return RGB{mix(c.R), mix(c.G), mix(c.B)}
}

// Margins of the page in millimetres.
type Margins struct{ Top, Right, Bottom, Left float64 }

// Doc is one PDF being drawn. Lengths are millimetres, font sizes points.
type Doc struct {
	P      *fpdf.Fpdf
	Brand  RGB
	family string
	m      Margins
}

const fontName = "doc"

func newDoc(fonts *Fonts, m Margins, brand RGB) *Doc {
	p := fpdf.New("P", "mm", "A4", "")
	for style, face := range fonts.faces {
		p.AddUTF8FontFromBytes(fontName, style, face)
	}
	p.SetMargins(m.Left, m.Top, m.Right)
	p.SetAutoPageBreak(true, m.Bottom)
	p.SetCellMargin(0)
	p.SetCreator("Kairso", true)
	d := &Doc{P: p, Brand: brand, family: fonts.Family, m: m}
	d.Font("", 12, Ink)
	return d
}

// Start adds the first page. Call it after setting any page header (watermark).
func (d *Doc) Start() {
	d.P.AddPage()
	d.Font("", 12, Ink)
}

// Left is the left margin; Width the width between the margins.
func (d *Doc) Left() float64 { return d.m.Left }
func (d *Doc) Width() float64 {
	w, _ := d.P.GetPageSize()
	return w - d.m.Left - d.m.Right
}

// Bottom is the lowest Y content may reach on a page.
func (d *Doc) Bottom() float64 {
	_, h := d.P.GetPageSize()
	return h - d.m.Bottom
}

// Font selects the style ("", "B", "I", "BI"), size in points and colour.
func (d *Doc) Font(style string, pt float64, c RGB) {
	d.P.SetFont(fontName, style, pt)
	d.P.SetTextColor(c.R, c.G, c.B)
}

// LineH is a comfortable line height in mm for a font size in points.
func LineH(pt float64, leading float64) float64 { return pt * 0.3528 * leading }

// Ensure starts a new page unless h mm fit below the current position.
func (d *Doc) Ensure(h float64) {
	if d.P.GetY()+h > d.Bottom() {
		d.P.AddPage()
	}
}

// Gap moves down by h mm.
func (d *Doc) Gap(h float64) { d.P.SetY(d.P.GetY() + h) }

// Wrap splits text into lines that fit w mm in the current font, keeping its line breaks.
func (d *Doc) Wrap(text string, w float64) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, d.P.SplitText(para, w)...)
	}
	return out
}

// Run is a piece of text in one style, for Runs.
type Run struct {
	Text  string
	Style string
	Size  float64
	Color RGB
}

// R is a regular run in ink.
func R(text string, size float64) Run { return Run{Text: text, Size: size, Color: Ink} }

// Runs writes styled text that wraps at the margins, starting indent mm into the first line.
func (d *Doc) Runs(lh, indent float64, runs ...Run) {
	d.P.SetX(d.m.Left + indent)
	for _, r := range runs {
		if r.Text == "" {
			continue
		}
		d.Font(r.Style, r.Size, r.Color)
		d.P.Write(lh, r.Text)
	}
	d.P.Ln(lh)
}

// Text writes a wrapped block of one style at x, w wide, aligned "L", "C", "R" or "J".
func (d *Doc) Text(x, w float64, text string, align string, lh float64) {
	d.P.SetX(x)
	d.P.MultiCell(w, lh, text, "", align, false)
}

// Centered writes one wrapped, centred block across the page.
func (d *Doc) Centered(text, style string, pt float64, c RGB, lh float64) {
	d.Font(style, pt, c)
	d.Text(d.m.Left, d.Width(), text, "C", lh)
}

// HLine draws a horizontal rule.
func (d *Doc) HLine(x1, x2, y, width float64, c RGB) {
	d.P.SetDrawColor(c.R, c.G, c.B)
	d.P.SetLineWidth(width)
	d.P.Line(x1, y, x2, y)
}

// Watermark prints big rotated text in the middle of every page (draft quotes).
func (d *Doc) Watermark(text string, c RGB) {
	d.P.SetHeaderFuncMode(func() {
		w, h := d.P.GetPageSize()
		d.P.TransformBegin()
		d.P.TransformRotate(30, w/2, h/2)
		d.Font("B", 80, c)
		tw := d.P.GetStringWidth(text)
		d.P.Text(w/2-tw/2, h/2+10, text)
		d.P.TransformEnd()
		d.Font("", 12, Ink)
	}, true)
}

// Stamp draws the in-app signature stamp (name and date in a double, slightly tilted frame)
// centred on (cx, cy).
func (d *Doc) Stamp(cx, cy float64, name, date string, c RGB) {
	name = strings.ToUpper(name)
	d.Font("B", 10, c)
	nw := d.P.GetStringWidth(name)
	d.Font("", 8, c)
	dw := d.P.GetStringWidth(date)
	w := math.Max(nw, dw) + 9
	h := 11.5
	x, y := cx-w/2, cy-h/2
	d.P.TransformBegin()
	d.P.TransformRotate(2, cx, cy)
	d.P.SetDrawColor(c.R, c.G, c.B)
	d.P.SetLineWidth(0.6)
	d.roundedRect(x, y, w, h, 1.5)
	d.P.SetLineWidth(0.25)
	d.roundedRect(x+0.9, y+0.9, w-1.8, h-1.8, 1)
	d.Font("B", 10, c)
	d.P.Text(cx-nw/2, y+5, name)
	d.Font("", 8, c)
	d.P.Text(cx-dw/2, y+9, date)
	d.P.TransformEnd()
}

// Image draws a registered image (see AddImage) fitted into a w×h box at (x, y).
func (d *Doc) Image(name string, x, y, w, h float64) {
	info := d.P.GetImageInfo(name)
	if info == nil {
		return
	}
	iw, ih := info.Width(), info.Height()
	if iw <= 0 || ih <= 0 {
		return
	}
	s := math.Min(w/iw, h/ih)
	dw, dh := iw*s, ih*s
	d.P.ImageOptions(name, x+(w-dw)/2, y+(h-dh)/2, dw, dh, false, fpdf.ImageOptions{}, 0, "")
}

// AddImage registers PNG/JPEG/GIF bytes under name; false when the format is not supported.
func (d *Doc) AddImage(name string, data []byte) bool {
	var typ string
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		typ = "PNG"
	case bytes.HasPrefix(data, []byte("\xFF\xD8")):
		typ = "JPG"
	case bytes.HasPrefix(data, []byte("GIF8")):
		typ = "GIF"
	default:
		return false
	}
	d.P.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: typ}, bytes.NewReader(data))
	if d.P.Err() {
		d.P.ClearError()
		return false
	}
	return true
}

// Bytes finishes the document.
func (d *Doc) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.P.Output(&buf); err != nil {
		return nil, fmt.Errorf("tạo PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// roundedRect strokes a rounded rectangle. fpdf's RoundedRect opens a graphics state ("q") it
// never closes, which would leak transforms (the tilted stamp) into everything drawn after;
// close it here.
func (d *Doc) roundedRect(x, y, w, h, r float64) {
	d.P.RoundedRect(x, y, w, h, r, "1234", "D")
	d.P.RawWriteStr("Q\n")
}
