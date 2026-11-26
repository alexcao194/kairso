package pdfrender

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// page marks itself ready only when it can see the token both in localStorage and in the cookie,
// like the web app's print pages.
const testPage = `<!doctype html><html><head><style>@page { size: A4; margin: 0 }</style></head>
<body><h1>Báo giá — Tiếng Việt có dấu</h1><script>
setTimeout(function () {
  var ok = localStorage.getItem("kairso_access_token") === "tok-123" && document.cookie.indexOf("kairso_access_token=tok-123") >= 0;
  document.documentElement.setAttribute("data-print-state", ok ? "ready" : "error");
}, 50);
</script></body></html>`

func newTestRenderer(t *testing.T) (*Renderer, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("starts Chromium")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(testPage))
	}))
	t.Cleanup(srv.Close)
	r, err := New(Config{WebURL: srv.URL, Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if _, err := r.browserCtx(); errors.Is(err, ErrNoBrowser) {
		t.Skip("no Chromium on this machine")
	}
	return r, srv.URL
}

func TestRenderPrintsReadyPage(t *testing.T) {
	r, _ := newTestRenderer(t)
	pdf, err := r.Render(context.Background(), "/print/x?a=1", "tok-123")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", pdf[:min(len(pdf), 16)])
	}
}

func TestRenderReportsPageError(t *testing.T) {
	r, _ := newTestRenderer(t)
	if _, err := r.Render(context.Background(), "/print/x", "wrong"); !errors.Is(err, ErrPageFailed) {
		t.Fatalf("want ErrPageFailed, got %v", err)
	}
}

func TestRenderRejectsForeignPaths(t *testing.T) {
	r, err := New(Config{WebURL: "http://web:3000"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"http://evil/", "//evil/x", "print"} {
		if _, err := r.Render(context.Background(), p, "t"); err == nil {
			t.Errorf("%q: want error", p)
		}
	}
}

func TestNewValidatesWebURL(t *testing.T) {
	for _, u := range []string{"", "web:3000", "ftp://web"} {
		if _, err := New(Config{WebURL: u}); err == nil {
			t.Errorf("%q: want error", u)
		}
	}
}
