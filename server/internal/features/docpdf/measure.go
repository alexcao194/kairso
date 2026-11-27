package docpdf

import (
	"context"
	"sync"

	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/go-pdf/fpdf"
)

// Look returns the workspace's name and brand colour without starting a PDF (used by the Excel
// export of administrative documents). Works on a nil Exporter or without fonts.
func (e *Exporter) Look(ctx context.Context, workspaceID string) Workspace {
	ws := Workspace{Brand: DefaultBrand}
	if e == nil || e.q == nil {
		return ws
	}
	if id, err := pgutil.ParseUUID(workspaceID); err == nil {
		if row, err := e.q.GetWorkspaceByID(ctx, id); err == nil {
			ws.Name = row.Name
			ws.Brand = ParseHex(row.BrandColor.String, DefaultBrand)
		}
	}
	return ws
}

// Measurer counts the lines text wraps to in w mm, in the document font at pt points
// ("", "B", "I" or "BI"), and measures a single line's width in mm. One is shared by all
// requests: loading the fonts costs ~5 ms, measuring is cheap.
type Measurer struct {
	mu sync.Mutex
	p  *fpdf.Fpdf
}

// Measurer returns the shared text measurer with the server's document font, or nil without fonts.
func (e *Exporter) Measurer() *Measurer {
	if e == nil || e.fonts == nil {
		return nil
	}
	e.measureOnce.Do(func() {
		p := fpdf.New("P", "mm", "A4", "")
		for style, face := range e.fonts.faces {
			p.AddUTF8FontFromBytes(fontName, style, face)
		}
		p.SetCellMargin(0)
		e.measure = &Measurer{p: p}
	})
	return e.measure
}

// Lines is the number of lines text takes in w mm, keeping its own line breaks.
func (m *Measurer) Lines(text, style string, pt, w float64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.p.SetFont(fontName, style, pt)
	d := Doc{P: m.p}
	return max(len(d.Wrap(text, w)), 1)
}

// Width is the width of one line of text in mm.
func (m *Measurer) Width(text, style string, pt float64) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.p.SetFont(fontName, style, pt)
	return m.p.GetStringWidth(text)
}
