package docexport

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Measure tells how text wraps in the document font, so rows get the height the text needs.
// Style is "", "B", "I" or "BI"; lengths are millimetres, sizes points.
type Measure interface {
	Lines(text, style string, pt, w float64) int
	Width(text, style string, pt float64) float64
}

// PageMargins in millimetres.
type PageMargins struct{ Top, Right, Bottom, Left float64 }

// Sheet is a one-sheet workbook laid out like a printed A4 page. Every block is placed by its
// horizontal position in millimetres across the text width; the sheet's columns are cut at
// every edge any block uses, so the layout lines up exactly as on paper and prints at 1:1.
type Sheet struct {
	f       *excelize.File
	sheet   string
	edges   []float64
	row     int
	styles  map[Style]int
	m       Measure
	margins PageMargins
	used    float64 // points used on the current printed page
}

// Style of a cell range. Comparable, so it doubles as the style cache key.
type Style struct {
	Size         float64
	Bold, Italic bool
	H            string // left (default), center, right, justify
	V            string // center (default), top
	Grid         bool   // thin black border on every side (tables)
	Rule         string // sides ("T", "B") with a thin black rule
	Box          string // sides ("LTRB") of a double frame in Color (signature stamp)
	Color        string // RRGGBB for text and frame; "" is black
	NumFmt       string
}

// Run is a styled piece of rich text.
type Run struct {
	Text         string
	Size         float64
	Bold, Italic bool
}

// NewSheet starts a sheet whose columns are cut at the given edges (mm from the left margin,
// within width). Edges closer than 0.8 mm to one another are merged.
func NewSheet(name string, width float64, edges []float64, m Measure, margins PageMargins) (*Sheet, error) {
	f := excelize.NewFile()
	name = SafeSheetName(name)
	if err := f.SetSheetName("Sheet1", name); err != nil {
		return nil, err
	}
	all := append([]float64{0, width}, edges...)
	sort.Float64s(all)
	var kept []float64
	for _, e := range all {
		if e < 0 || e > width {
			continue
		}
		if len(kept) > 0 && e-kept[len(kept)-1] < 0.8 {
			continue
		}
		kept = append(kept, e)
	}
	if kept[len(kept)-1] < width {
		kept[len(kept)-1] = width
	}
	s := &Sheet{f: f, sheet: name, edges: kept, styles: map[Style]int{}, m: m, margins: margins}
	for i := 1; i < len(kept); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		if err := f.SetColWidth(name, col, col, colChars(kept[i]-kept[i-1])); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// colChars converts millimetres to Excel's column width unit (characters of the default
// 11pt Calibri: 7 px per character plus 5 px padding, at 96 dpi).
func colChars(mm float64) float64 {
	px := mm * 96 / 25.4
	if px < 12 {
		return px / 12
	}
	return (px - 5) / 7
}

// PtMM converts millimetres to points (row heights).
func PtMM(mm float64) float64 { return mm * 72 / 25.4 }

func (s *Sheet) col(x float64) int {
	best, bd := 0, math.Inf(1)
	for i, e := range s.edges {
		if d := math.Abs(e - x); d < bd {
			best, bd = i, d
		}
	}
	return best
}

func (s *Sheet) cells(r0, r1 int, x0, x1 float64) (string, string) {
	c0, c1 := s.col(x0)+1, s.col(x1)
	if c1 < c0 {
		c1 = c0
	}
	a, _ := excelize.CoordinatesToCellName(c0, r0)
	b, _ := excelize.CoordinatesToCellName(c1, r1)
	return a, b
}

// pageHeight is the printable height of an A4 page in points, a little short so Excel's own
// rounding never pushes a row past the manual page breaks.
func (s *Sheet) pageHeight() float64 {
	return PtMM(297-s.margins.Top-s.margins.Bottom) * 0.97
}

// Row starts a new row of the given height in points and returns its number. Rows are never
// split: one that does not fit on the current page starts the next.
func (s *Sheet) Row(pt float64) int {
	pt = math.Min(math.Max(pt, 2), 409)
	if !s.Fits(pt) {
		s.Break()
	}
	s.row++
	s.used += pt
	_ = s.f.SetRowHeight(s.sheet, s.row, pt)
	return s.row
}

// Fits reports whether pt more points fit on the current page.
func (s *Sheet) Fits(pt float64) bool { return s.used+pt <= s.pageHeight() }

// Keep starts a new page unless pt points (a block that must stay together) fit.
func (s *Sheet) Keep(pt float64) {
	if !s.Fits(pt) {
		s.Break()
	}
}

// Break starts a new printed page at the next row.
func (s *Sheet) Break() {
	if s.used == 0 {
		return
	}
	_ = s.f.InsertPageBreak(s.sheet, fmt.Sprintf("A%d", s.row+1))
	s.used = 0
}

// Gap leaves an empty row mm high (none at the top of a page).
func (s *Sheet) Gap(mm float64) {
	if pt := PtMM(mm); s.Fits(pt) {
		s.Row(pt)
	} else {
		s.Break()
	}
}

// Grow raises a row to at least pt points.
func (s *Sheet) Grow(row int, pt float64) {
	pt = math.Min(pt, 409)
	if h, _ := s.f.GetRowHeight(s.sheet, row); pt > h {
		s.used += pt - h
		_ = s.f.SetRowHeight(s.sheet, row, pt)
	}
}

// Lines is how many lines text wraps to in w mm. Excel draws text a little wider than the PDF
// font metrics and pads cells, so the width is reduced to stay on the safe side.
func (s *Sheet) Lines(text, style string, pt, w float64) int {
	w = w*0.95 - 1.2
	if s.m != nil {
		return s.m.Lines(text, style, pt, w)
	}
	perLine := math.Max(w/(pt*0.3528*0.5), 1)
	n := 0
	for _, l := range strings.Split(text, "\n") {
		n += int(math.Max(1, math.Ceil(float64(utf8.RuneCountInString(l))/perLine)))
	}
	return n
}

// TextWidth is the width of one line of text in mm.
func (s *Sheet) TextWidth(text, style string, pt float64) float64 {
	if s.m != nil {
		return s.m.Width(text, style, pt)
	}
	return float64(utf8.RuneCountInString(text)) * pt * 0.3528 * 0.5
}

// Put writes a value (string, number or nil) into the rows r0..r1 between x0 and x1 mm,
// merging the range when it spans more than one cell.
func (s *Sheet) Put(r0, r1 int, x0, x1 float64, v any, st Style) {
	a, b := s.cells(r0, r1, x0, x1)
	if a != b {
		_ = s.f.MergeCell(s.sheet, a, b)
	}
	if v != nil {
		_ = s.f.SetCellValue(s.sheet, a, v)
	}
	_ = s.f.SetCellStyle(s.sheet, a, b, s.style(st))
}

// Rich writes rich text like Put. Runs without a size use the style's size.
func (s *Sheet) Rich(r0, r1 int, x0, x1 float64, runs []Run, st Style) {
	a, b := s.cells(r0, r1, x0, x1)
	if a != b {
		_ = s.f.MergeCell(s.sheet, a, b)
	}
	var rt []excelize.RichTextRun
	for _, r := range runs {
		if r.Text == "" {
			continue
		}
		size := r.Size
		if size == 0 {
			size = st.Size
		}
		rt = append(rt, excelize.RichTextRun{Text: r.Text, Font: &excelize.Font{Family: Font, Size: size, Bold: r.Bold, Italic: r.Italic, Color: color(st.Color)}})
	}
	if len(rt) > 0 {
		_ = s.f.SetCellRichText(s.sheet, a, rt)
	}
	_ = s.f.SetCellStyle(s.sheet, a, b, s.style(st))
}

// Font of administrative documents.
const Font = "Times New Roman"

func color(c string) string {
	if c == "" {
		return "000000"
	}
	return c
}

func (s *Sheet) style(st Style) int {
	if id, ok := s.styles[st]; ok {
		return id
	}
	size := st.Size
	if size == 0 {
		size = 13
	}
	h := st.H
	if h == "" {
		h = "left"
	}
	v := st.V
	if v == "" {
		v = "center"
	}
	es := &excelize.Style{
		Font:      &excelize.Font{Family: Font, Size: size, Bold: st.Bold, Italic: st.Italic, Color: color(st.Color)},
		Alignment: &excelize.Alignment{Horizontal: h, Vertical: v, WrapText: true},
	}
	sides := map[string]excelize.Border{}
	if st.Grid {
		for _, t := range []string{"left", "top", "right", "bottom"} {
			sides[t] = excelize.Border{Type: t, Color: "000000", Style: 1}
		}
	}
	names := map[rune]string{'L': "left", 'T': "top", 'R': "right", 'B': "bottom"}
	for _, c := range st.Rule {
		sides[names[c]] = excelize.Border{Type: names[c], Color: "000000", Style: 1}
	}
	for _, c := range st.Box {
		sides[names[c]] = excelize.Border{Type: names[c], Color: color(st.Color), Style: 6}
	}
	for _, t := range []string{"left", "top", "right", "bottom"} {
		if b, ok := sides[t]; ok {
			es.Border = append(es.Border, b)
		}
	}
	if st.NumFmt != "" {
		f := st.NumFmt
		es.CustomNumFmt = &f
	}
	id, err := s.f.NewStyle(es)
	if err != nil {
		id = 0
	}
	s.styles[st] = id
	return id
}

// Bytes renders the workbook: A4 portrait with the page margins, no gridlines, fitted to one
// page wide.
func (s *Sheet) Bytes() ([]byte, error) {
	in := func(mm float64) *float64 { v := mm / 25.4; return &v }
	zero := 0.0
	_ = s.f.SetPageMargins(s.sheet, &excelize.PageLayoutMarginsOptions{
		Top: in(s.margins.Top), Bottom: in(s.margins.Bottom), Left: in(s.margins.Left), Right: in(s.margins.Right),
		Header: &zero, Footer: &zero,
	})
	fitW, fitH := 1, 0
	_ = s.f.SetPageLayout(s.sheet, &excelize.PageLayoutOptions{Size: ptr(9), Orientation: ptr("portrait"), FitToWidth: &fitW, FitToHeight: &fitH})
	_ = s.f.SetSheetProps(s.sheet, &excelize.SheetPropsOptions{FitToPage: ptr(true)})
	_ = s.f.SetSheetView(s.sheet, 0, &excelize.ViewOptions{ShowGridLines: ptr(false)})
	buf, err := s.f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("tạo Excel: %w", err)
	}
	return buf.Bytes(), nil
}
