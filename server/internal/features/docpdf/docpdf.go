// Package docpdf turns the web app's print pages (quotes, meeting minutes) into PDF files on the
// server: it signs the user in with a short-lived read-only token and asks the headless browser
// renderer to print the page, so every client gets the same document.
package docpdf

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexcao194/kairso/pkg/jwt"
	"github.com/alexcao194/kairso/pkg/pdfrender"
)

const ContentType = "application/pdf"

// tokenTTL covers one render; the renderer gives up well before (pdfrender's own timeout).
const tokenTTL = 3 * time.Minute

// ErrUnavailable means the server has no PDF renderer configured.
var ErrUnavailable = errors.New("máy chủ chưa cấu hình xuất PDF")

// Renderer prints a web page as the holder of a token.
type Renderer interface {
	Render(ctx context.Context, path, token string) ([]byte, error)
}

// Exporter renders print pages as a given user. A nil *Exporter reports ErrUnavailable.
type Exporter struct {
	renderer Renderer
	tokens   jwt.TokenService
}

func New(renderer Renderer, tokens jwt.TokenService) *Exporter {
	return &Exporter{renderer: renderer, tokens: tokens}
}

// Render prints the web page at path (query included) as userID.
func (e *Exporter) Render(ctx context.Context, userID, path string) ([]byte, error) {
	if e == nil || e.renderer == nil {
		return nil, ErrUnavailable
	}
	token, err := e.tokens.GenerateRenderToken(userID, tokenTTL)
	if err != nil {
		return nil, err
	}
	return e.renderer.Render(ctx, path, token)
}

// PrintQuery copies the print options the print pages understand from the API request:
// template (modern|admin, sent as the page's "format"), sign (digital|wet), place, signer,
// merge (0|1) and fit (0|1). Anything else is dropped.
func PrintQuery(r *http.Request) url.Values {
	in := r.URL.Query()
	out := url.Values{}
	if in.Get("template") == "admin" {
		out.Set("format", "admin")
	}
	if in.Get("sign") == "wet" {
		out.Set("sign", "wet")
	}
	for _, k := range []string{"place", "signer"} {
		if v := strings.TrimSpace(in.Get(k)); v != "" && utf8.RuneCountInString(v) <= 120 {
			out.Set(k, v)
		}
	}
	for _, k := range []string{"merge", "fit"} {
		if v := in.Get(k); v == "0" || v == "1" {
			out.Set(k, v)
		}
	}
	return out
}

// PagePath joins a print page path and its query.
func PagePath(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// AllowSlowResponse lifts the server's write timeout for this response: printing a document
// can take longer than an ordinary API call.
func AllowSlowResponse(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(90 * time.Second))
}

// Write sends a PDF as a download.
func Write(w http.ResponseWriter, filename string, data []byte) {
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Status maps a render error to the HTTP status to answer with.
func Status(err error) int {
	switch {
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, pdfrender.ErrBusy), errors.Is(err, pdfrender.ErrNoBrowser):
		return http.StatusServiceUnavailable
	case errors.Is(err, pdfrender.ErrPageFailed):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
