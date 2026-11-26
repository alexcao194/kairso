// Package docexport builds simple one-sheet Excel documents (quotes, meeting minutes): a title,
// free text lines and bordered tables, with money and quantity cells kept numeric so the
// recipient can keep calculating in Excel.
package docexport

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const ContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// Money is a numeric cell shown with the book's money format.
type Money float64

// Qty is a numeric cell shown with up to three decimals.
type Qty float64

// Book is one worksheet filled top to bottom.
type Book struct {
	f      *excelize.File
	sheet  string
	widths []float64
	row    int
	st     styles
}

type styles struct {
	title, sub, text, bold, italic, head, cell, cellC, money, qty, totalL, totalV, totalLB, totalVB int
}

// New starts a workbook whose single sheet has the given column widths (in characters).
// moneyDecimals is the currency's minor units (0 for VND, 2 for USD).
func New(sheet string, widths []float64, moneyDecimals int) (*Book, error) {
	f := excelize.NewFile()
	sheet = SafeSheetName(sheet)
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	b := &Book{f: f, sheet: sheet, widths: widths}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(sheet, col, col, w); err != nil {
			return nil, err
		}
	}
	moneyFmt := "#,##0"
	if moneyDecimals > 0 {
		moneyFmt += "." + strings.Repeat("0", moneyDecimals)
	}
	font := func(size float64, bold, italic bool) *excelize.Font {
		return &excelize.Font{Family: "Times New Roman", Size: size, Bold: bold, Italic: italic}
	}
	border := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1},
	}
	wrap := func(h string) *excelize.Alignment {
		return &excelize.Alignment{Horizontal: h, Vertical: "top", WrapText: true}
	}
	defs := []struct {
		dst *int
		s   *excelize.Style
	}{
		{&b.st.title, &excelize.Style{Font: font(16, true, false), Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}}},
		{&b.st.sub, &excelize.Style{Font: font(11, false, true), Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}}},
		{&b.st.text, &excelize.Style{Font: font(11, false, false), Alignment: wrap("left")}},
		{&b.st.bold, &excelize.Style{Font: font(11, true, false), Alignment: wrap("left")}},
		{&b.st.italic, &excelize.Style{Font: font(11, false, true), Alignment: wrap("left")}},
		{&b.st.head, &excelize.Style{Font: font(11, true, false), Border: border, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EDEDED"}}}},
		{&b.st.cell, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("left")}},
		{&b.st.cellC, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("center")}},
		{&b.st.money, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("right"), CustomNumFmt: &moneyFmt}},
		{&b.st.qty, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("right"), CustomNumFmt: ptr("#,##0.###")}},
		{&b.st.totalL, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("right")}},
		{&b.st.totalV, &excelize.Style{Font: font(11, false, false), Border: border, Alignment: wrap("right"), CustomNumFmt: &moneyFmt}},
		{&b.st.totalLB, &excelize.Style{Font: font(11, true, false), Border: border, Alignment: wrap("right")}},
		{&b.st.totalVB, &excelize.Style{Font: font(11, true, false), Border: border, Alignment: wrap("right"), CustomNumFmt: &moneyFmt}},
	}
	for _, d := range defs {
		id, err := f.NewStyle(d.s)
		if err != nil {
			return nil, err
		}
		*d.dst = id
	}
	return b, nil
}

func ptr[T any](v T) *T { return &v }

func (b *Book) cellName(col, row int) string {
	n, _ := excelize.CoordinatesToCellName(col, row)
	return n
}

// fitHeight sets the row height so wrapped text in a cell of the given width stays visible.
func (b *Book) fitHeight(row int, text string, width float64) {
	lines := 0
	perLine := int(math.Max(width*1.3, 1))
	for _, l := range strings.Split(text, "\n") {
		lines += int(math.Max(1, math.Ceil(float64(utf8.RuneCountInString(l))/float64(perLine))))
	}
	if h, _ := b.f.GetRowHeight(b.sheet, row); float64(lines)*15 > h {
		_ = b.f.SetRowHeight(b.sheet, row, float64(lines)*15)
	}
}

func (b *Book) span(from, to int) float64 {
	w := 0.0
	for i := from; i <= to && i < len(b.widths); i++ {
		w += b.widths[i]
	}
	return w
}

// merged writes text across all columns of a new row.
func (b *Book) merged(text string, style int) {
	b.row++
	first, last := b.cellName(1, b.row), b.cellName(len(b.widths), b.row)
	_ = b.f.MergeCell(b.sheet, first, last)
	_ = b.f.SetCellValue(b.sheet, first, text)
	_ = b.f.SetCellStyle(b.sheet, first, last, style)
	b.fitHeight(b.row, text, b.span(0, len(b.widths)-1))
}

// Title is a large centered heading.
func (b *Book) Title(text string) {
	b.merged(text, b.st.title)
	_ = b.f.SetRowHeight(b.sheet, b.row, 26)
}

// Subtitle is a centered italic line under the title.
func (b *Book) Subtitle(text string) { b.merged(text, b.st.sub) }

// Line is a full-width text line.
func (b *Book) Line(text string) { b.merged(text, b.st.text) }

// Bold is a full-width bold line (a section heading).
func (b *Book) Bold(text string) { b.merged(text, b.st.bold) }

// Italic is a full-width italic line.
func (b *Book) Italic(text string) { b.merged(text, b.st.italic) }

// Field writes "label: value" when the value is not empty.
func (b *Book) Field(label, value string) {
	if strings.TrimSpace(value) != "" {
		b.Line(fmt.Sprintf("%s: %s", label, value))
	}
}

// Blank leaves an empty row.
func (b *Book) Blank() { b.row++ }

// Header writes a bordered, shaded table header row.
func (b *Book) Header(cells ...string) {
	b.row++
	for i, c := range cells {
		_ = b.f.SetCellValue(b.sheet, b.cellName(i+1, b.row), c)
		_ = b.f.SetCellStyle(b.sheet, b.cellName(i+1, b.row), b.cellName(i+1, b.row), b.st.head)
		b.fitHeight(b.row, c, b.span(i, i))
	}
}

// Center marks a text cell to be centered.
type Center string

// Row writes a bordered table row. Cells may be string, Center, int, Money, Qty or nil.
func (b *Book) Row(cells ...any) {
	b.row++
	for i, c := range cells {
		name := b.cellName(i+1, b.row)
		style := b.st.cell
		switch v := c.(type) {
		case nil:
		case string:
			_ = b.f.SetCellValue(b.sheet, name, v)
			b.fitHeight(b.row, v, b.span(i, i))
		case Center:
			_ = b.f.SetCellValue(b.sheet, name, string(v))
			style = b.st.cellC
			b.fitHeight(b.row, string(v), b.span(i, i))
		case int:
			_ = b.f.SetCellValue(b.sheet, name, v)
			style = b.st.cellC
		case Money:
			_ = b.f.SetCellValue(b.sheet, name, float64(v))
			style = b.st.money
		case Qty:
			_ = b.f.SetCellValue(b.sheet, name, float64(v))
			style = b.st.qty
		}
		_ = b.f.SetCellStyle(b.sheet, name, name, style)
	}
}

// Total writes a "label | amount" row whose label spans every column but the last.
func (b *Book) Total(label string, value Money, bold bool) {
	b.row++
	n := len(b.widths)
	first, lastLabel, valueCell := b.cellName(1, b.row), b.cellName(n-1, b.row), b.cellName(n, b.row)
	_ = b.f.MergeCell(b.sheet, first, lastLabel)
	_ = b.f.SetCellValue(b.sheet, first, label)
	_ = b.f.SetCellValue(b.sheet, valueCell, float64(value))
	ls, vs := b.st.totalL, b.st.totalV
	if bold {
		ls, vs = b.st.totalLB, b.st.totalVB
	}
	_ = b.f.SetCellStyle(b.sheet, first, lastLabel, ls)
	_ = b.f.SetCellStyle(b.sheet, valueCell, valueCell, vs)
}

// Bytes renders the workbook, set up to print on A4 portrait fitted to the page width.
func (b *Book) Bytes() ([]byte, error) {
	fitW, fitH := 1, 0
	_ = b.f.SetPageLayout(b.sheet, &excelize.PageLayoutOptions{
		Size: ptr(9), Orientation: ptr("portrait"), FitToWidth: &fitW, FitToHeight: &fitH,
	})
	_ = b.f.SetSheetProps(b.sheet, &excelize.SheetPropsOptions{FitToPage: ptr(true)})
	buf, err := b.f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SafeSheetName strips characters Excel forbids in sheet names and caps the length at 31.
func SafeSheetName(name string) string {
	out := strings.TrimSpace(strings.NewReplacer("\\", " ", "/", " ", "*", " ", "?", " ", ":", " ", "[", " ", "]", " ").Replace(name))
	if out == "" {
		out = "Sheet1"
	}
	if r := []rune(out); len(r) > 31 {
		out = string(r[:31])
	}
	return out
}

// SafeFilename makes a string usable as a file name.
func SafeFilename(s string) string {
	out := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(strings.TrimSpace(s))
	if out == "" {
		return "document"
	}
	return out
}

// MinorToMajor converts an amount in minor units (cents, đồng) to a number for a Money cell.
func MinorToMajor(v int64, minorUnits int) Money {
	return Money(float64(v) / math.Pow10(minorUnits))
}
