package docpdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
)

const ContentType = "application/pdf"

// ErrUnavailable means the server has no font to draw PDFs with.
var ErrUnavailable = errors.New("máy chủ chưa có font Times New Roman / Liberation Serif để tạo PDF")

// Exporter creates documents with the server's fonts and the workspace's look. A nil
// *Exporter reports ErrUnavailable.
type Exporter struct {
	fonts  *Fonts
	q      *database.Queries
	client *http.Client
}

// New finds the document font (PDF_FONT_DIR, then the system) and logs which one it uses.
func New(fontDir string, q *database.Queries) *Exporter {
	fonts, err := LoadFonts(fontDir)
	if err != nil {
		slog.Error("PDF export disabled: no Times New Roman or Liberation Serif font", "fontDir", fontDir)
		return &Exporter{q: q}
	}
	if fonts.Family != "Times New Roman" {
		slog.Warn("Times New Roman not found, PDFs use a metric-compatible serif", "font", fonts.Family, "hint", "put the Times New Roman .ttf files in PDF_FONT_DIR")
	} else {
		slog.Info("PDF font", "font", fonts.Family, "dir", fonts.Source)
	}
	return &Exporter{fonts: fonts, q: q, client: &http.Client{Timeout: 5 * time.Second}}
}

// Workspace is the look of the workspace a document belongs to.
type Workspace struct {
	Name    string
	Brand   RGB
	HasLogo bool
}

// LogoImage is the name the workspace logo is registered under in a Doc.
const LogoImage = "logo"

// Begin starts a document for the workspace: fonts, margins, brand colour and logo.
func (e *Exporter) Begin(ctx context.Context, workspaceID string, m Margins) (*Doc, Workspace, error) {
	if e == nil || e.fonts == nil {
		return nil, Workspace{}, ErrUnavailable
	}
	ws := Workspace{Brand: DefaultBrand}
	var avatar string
	if id, err := pgutil.ParseUUID(workspaceID); err == nil && e.q != nil {
		if row, err := e.q.GetWorkspaceByID(ctx, id); err == nil {
			ws.Name = row.Name
			ws.Brand = ParseHex(row.BrandColor.String, DefaultBrand)
			avatar = row.Avatar.String
		}
	}
	d := newDoc(e.fonts, m, ws.Brand)
	if avatar != "" {
		if data := e.fetch(ctx, avatar); data != nil && d.AddImage(LogoImage, data) {
			ws.HasLogo = true
		}
	}
	return d, ws, nil
}

// fetch downloads a small image (the workspace logo); failures just leave the logo out.
func (e *Exporter) fetch(ctx context.Context, url string) []byte {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	res, err := e.client.Do(req)
	if err != nil {
		slog.Warn("PDF: logo not loaded", "url", url, "error", err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return nil
	}
	return data
}

// Options are the print options of the print pages, read from the API query.
type Options struct {
	Admin  bool   // template=admin: administrative (Decree 30) layout
	Wet    bool   // sign=wet: leave signature boxes empty to sign by hand
	Place  string // place in the date line (admin)
	Signer string // approver title (admin quote)
	Merge  string // "1"/"0": one signature block for preparer and approver (admin quote); "" = auto
}

// ParseOptions reads template, sign, place, signer and merge; anything else is ignored.
func ParseOptions(r *http.Request) Options {
	q := r.URL.Query()
	o := Options{Admin: q.Get("template") == "admin", Wet: q.Get("sign") == "wet"}
	clean := func(v string) string {
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) > 120 {
			return ""
		}
		return v
	}
	o.Place = clean(q.Get("place"))
	o.Signer = strings.ToUpper(clean(q.Get("signer")))
	if m := q.Get("merge"); m == "0" || m == "1" {
		o.Merge = m
	}
	return o
}

// Write sends a PDF as a download.
func Write(w http.ResponseWriter, filename string, data []byte) {
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Status maps an export error to the HTTP status to answer with.
func Status(err error) int {
	if errors.Is(err, ErrUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}
