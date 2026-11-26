// Package pdfrender prints pages of the Kairso web app to A4 PDF with headless Chromium, so the
// server can hand out the same document the browser print pages show (quotes, meeting minutes)
// without depending on the user's browser.
//
// The renderer opens WebURL+path in a fresh incognito tab, signed in with a short-lived token
// the caller minted, waits until the page marks itself ready (`<html data-print-state="ready">`)
// and prints it using the page's own `@page` size and margins.
package pdfrender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// TokenKey is where the web app keeps its access token (localStorage and cookie).
const TokenKey = "kairso_access_token"

var (
	// ErrPageFailed means the print page loaded but reported that it could not show the document.
	ErrPageFailed = errors.New("trang in không tải được tài liệu")
	// ErrNoBrowser means Chromium could not be started or reached.
	ErrNoBrowser = errors.New("không khởi động được trình tạo PDF (Chromium)")
	// ErrBusy means too many documents are being rendered at once.
	ErrBusy = errors.New("máy chủ đang bận tạo PDF, vui lòng thử lại")
)

type Config struct {
	// WebURL is the web app's base URL as reachable from the server (http://kairso-web:3000…).
	WebURL string
	// ChromeURL, when set, is the DevTools endpoint of a Chromium running elsewhere
	// (ws://chrome:9222). Otherwise a local Chromium is started on first use.
	ChromeURL string
	// ChromePath overrides the Chromium binary; empty finds chromium/google-chrome on PATH.
	ChromePath string
	// MaxConcurrent caps the tabs rendering at the same time (default 2).
	MaxConcurrent int
	// Timeout bounds one render, page load included (default 45s).
	Timeout time.Duration
}

type Renderer struct {
	cfg     Config
	origin  string
	sem     chan struct{}
	mu      sync.Mutex
	browser context.Context
	stop    context.CancelFunc
}

func New(cfg Config) (*Renderer, error) {
	u, err := url.Parse(strings.TrimRight(cfg.WebURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("pdfrender: invalid web URL %q", cfg.WebURL)
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 45 * time.Second
	}
	cfg.WebURL = u.String()
	return &Renderer{cfg: cfg, origin: u.Scheme + "://" + u.Host, sem: make(chan struct{}, cfg.MaxConcurrent)}, nil
}

// Close stops the browser started by the renderer, if any.
func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stop != nil {
		r.stop()
		r.browser, r.stop = nil, nil
	}
}

func (r *Renderer) print(tab context.Context, path, token, seed string) ([]byte, error) {
	// Opens the tab.
	if err := chromedp.Do(tab); err != nil {
		return nil, err
	}
	// The web app reads the token from localStorage (API calls) and from a cookie (route guard).
	if _, err := chromedp.Call(tab, network.SetCookie, network.SetCookieParams{
		Name: TokenKey, Value: token, URL: r.origin, Path: "/",
	}); err != nil {
		return nil, err
	}
	if _, err := chromedp.Call(tab, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: seed}); err != nil {
		return nil, err
	}
	if err := chromedp.Do(tab,
		// A desktop-wide viewport, so the A4 sheet (210mm ≈ 794px) is laid out at full width.
		chromedp.EmulateViewport(1280, 1800),
		chromedp.Navigate(r.cfg.WebURL+path),
	); err != nil {
		return nil, err
	}
	state, err := chromedp.Run(tab, chromedp.Poll[string](
		`document.documentElement.getAttribute("data-print-state")`,
		chromedp.WithPollingInterval(150*time.Millisecond),
		chromedp.WithPollingTimeout(r.cfg.Timeout),
	))
	if err != nil {
		return nil, err
	}
	if state != "ready" {
		return nil, ErrPageFailed
	}
	// The page's @page rule sets the size and margins; A4 with no margin is the fallback.
	return chromedp.Run(tab, chromedp.PrintToPDF(
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFMargin(0),
		chromedp.PDFPreferCSSPageSize(),
		chromedp.PDFPrintBackground(),
	))
}

// reset drops browser if it is still the shared one.
func (r *Renderer) reset(browser context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.browser == browser && r.stop != nil {
		r.stop()
		r.browser, r.stop = nil, nil
	}
}

// browserCtx returns the shared browser, starting it (again) when needed.
func (r *Renderer) browserCtx() (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.browser != nil && r.browser.Err() == nil {
		return r.browser, nil
	}
	var alloc context.Context
	var cancelAlloc context.CancelFunc
	if r.cfg.ChromeURL != "" {
		alloc, cancelAlloc = remote.NewAllocator(context.Background(), r.cfg.ChromeURL)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.NoSandbox, // containers run as root, where Chromium's sandbox cannot start
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("font-render-hinting", "none"),
			chromedp.Flag("hide-scrollbars", true),
		)
		if r.cfg.ChromePath != "" {
			opts = append(opts, chromedp.ExecPath(r.cfg.ChromePath))
		}
		alloc, cancelAlloc = chromedp.NewExecAllocator(context.Background(), opts...)
	}
	browser, cancelBrowser := chromedp.NewContext(alloc)
	// Start the browser now so a missing binary is reported here, not mid-render.
	if err := chromedp.Do(browser); err != nil {
		cancelBrowser()
		cancelAlloc()
		slog.Error("pdfrender: start chromium", "error", err)
		return nil, ErrNoBrowser
	}
	r.browser = browser
	r.stop = func() { cancelBrowser(); cancelAlloc() }
	return browser, nil
}

// Render prints the web page at path (with its query) as the holder of token.
func (r *Renderer) Render(ctx context.Context, path, token string) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, fmt.Errorf("pdfrender: path must be absolute: %q", path)
	}
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(r.cfg.Timeout):
		return nil, ErrBusy
	}

	browser, err := r.browserCtx()
	if err != nil {
		return nil, err
	}
	// Each document gets its own incognito context, so tokens and storage never leak between renders.
	tab, cancelTab := chromedp.NewContext(browser, chromedp.WithNewBrowserContext())
	defer cancelTab()
	tab, cancelTimeout := context.WithTimeout(tab, r.cfg.Timeout)
	defer cancelTimeout()
	stopWatch := context.AfterFunc(ctx, cancelTab) // the HTTP caller went away
	defer stopWatch()

	tokenJS, _ := json.Marshal(token)
	seed := fmt.Sprintf(`try { localStorage.setItem(%q, %s); } catch (e) {}`, TokenKey, tokenJS)

	pdf, err := r.print(tab, path, token, seed)
	if err != nil {
		if errors.Is(err, ErrPageFailed) {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) || tab.Err() != nil {
			return nil, fmt.Errorf("pdfrender: %s did not become ready within %s", path, r.cfg.Timeout)
		}
		slog.Warn("pdfrender: render failed", "path", path, "error", err)
		r.reset(browser) // the browser may have died; start a fresh one next time
		return nil, fmt.Errorf("pdfrender: %w", err)
	}
	return pdf, nil
}
