package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/alexcao194/kairso/pkg/httpx"
)

// TestToolset checks every tool's schema: unique names, required arguments declared as
// properties, and every {placeholder} the runner fills declared too.
func TestToolset(t *testing.T) {
	seen := map[string]bool{}
	for _, tl := range toolset() {
		if seen[tl.name] {
			t.Errorf("duplicate tool %s", tl.name)
		}
		seen[tl.name] = true
		props, _ := tl.schema["properties"].(map[string]any)
		req, _ := tl.schema["required"].([]string)
		for _, r := range req {
			if _, ok := props[r]; !ok {
				t.Errorf("%s: required %q is not a property", tl.name, r)
			}
		}
		if tl.destructive && tl.readOnly {
			t.Errorf("%s: both read-only and destructive", tl.name)
		}
	}
	if len(seen) < 50 {
		t.Errorf("only %d tools", len(seen))
	}
}

type rest struct {
	calls []seen
	mux   *http.ServeMux
}

func (r *rest) on(pattern string, status int, out any) {
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		r.calls = append(r.calls, seen{method: req.Method, path: req.URL.Path, query: req.URL.RawQuery, body: string(b)})
		if data, ok := out.([]byte); ok {
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="BG-001.pdf"`)
			_, _ = w.Write(data)
			return
		}
		httpx.JSON(w, status, out)
	})
}

func newRest() (*rest, http.Handler) {
	r := &rest{mux: http.NewServeMux()}
	mux := http.NewServeMux()
	NewHandler(r.mux, fakeKeys{}).RegisterRoutes(mux)
	return r, mux
}

func body(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("body %q: %v", s, err)
	}
	return m
}

func TestUpdateQuoteMergesDraft(t *testing.T) {
	r, h := newRest()
	r.on("GET /v1/workspaces/{ws}/quotes/{id}", 200, map[string]any{
		"id": "q1", "editable": true, "status": "draft", "title": "Old", "currency": "VND", "language": "vi",
		"issueDate": "2026-11-01", "validUntil": "2026-12-01", "discountType": "amount", "discountPercent": 0,
		"discountAmount": 0, "vatRate": 8, "notes": "n", "terms": "t", "customerId": "c1",
		"items": []any{map[string]any{"id": "i1", "name": "Web", "quantity": 1, "unitPrice": 1000, "unit": "gói", "lineTotal": 1000}},
	})
	r.on("PUT /v1/workspaces/{ws}/quotes/{id}", 200, map[string]any{"id": "q1"})
	res := callTool(t, h, "update_quote", `{"workspaceId":"w1","quoteId":"q1","title":"New","vatRate":10}`)
	if res.Result.IsError {
		t.Fatal(res.Result.Content[0].Text)
	}
	put := body(t, r.calls[1].body)
	if put["title"] != "New" || put["vatRate"] != 10.0 || put["terms"] != "t" || put["customerId"] != "c1" {
		t.Errorf("merged body: %v", put)
	}
	items := put["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["unitPrice"] != 1000.0 || items[0].(map[string]any)["lineTotal"] != nil {
		t.Errorf("items kept: %v", items)
	}

	// Locked quotes are refused before any write.
	r2, h2 := newRest()
	r2.on("GET /v1/workspaces/{ws}/quotes/{id}", 200, map[string]any{"editable": false, "status": "sent"})
	if res := callTool(t, h2, "update_quote", `{"workspaceId":"w1","quoteId":"q1","title":"x"}`); !res.Result.IsError ||
		!strings.Contains(res.Result.Content[0].Text, "new_quote_version") {
		t.Errorf("locked: %+v", res)
	}
	if len(r2.calls) != 1 {
		t.Errorf("calls on locked quote: %v", r2.calls)
	}
}

func TestCreateQuoteFillsItems(t *testing.T) {
	r, h := newRest()
	r.on("POST /v1/workspaces/{ws}/quotes", 201, map[string]any{"id": "q9"})
	r.on("GET /v1/workspaces/{ws}/quotes/{id}", 200, map[string]any{"id": "q9", "editable": true, "discountType": "amount", "items": []any{}})
	r.on("PUT /v1/workspaces/{ws}/quotes/{id}", 200, map[string]any{"id": "q9", "ok": true})
	res := callTool(t, h, "create_quote", `{"workspaceId":"w1","moduleId":"m1","items":[{"name":"Thiết kế","unitPrice":5000000}]}`)
	if res.Result.IsError || len(r.calls) != 3 || r.calls[2].path != "/v1/workspaces/w1/quotes/q9" {
		t.Fatalf("%+v %v", res, r.calls)
	}
	it := body(t, r.calls[2].body)["items"].([]any)[0].(map[string]any)
	if it["quantity"] != 1.0 || it["name"] != "Thiết kế" {
		t.Errorf("item: %v", it)
	}
}

func TestExportQuoteReturnsFile(t *testing.T) {
	r, h := newRest()
	r.on("GET /v1/workspaces/{ws}/quotes/{id}/pdf", 200, []byte("%PDF-1.4 test"))
	rec := post(t, h, "ksk_good", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"export_quote","arguments":{"workspaceId":"w1","quoteId":"q1","template":"admin","merge":true}}}`)
	var out struct {
		Result struct {
			Content []struct {
				Type     string `json:"type"`
				Resource struct {
					MimeType string `json:"mimeType"`
					Blob     string `json:"blob"`
				} `json:"resource"`
			} `json:"content"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Result.Content) != 2 || out.Result.Content[1].Type != "resource" ||
		out.Result.Content[1].Resource.MimeType != "application/pdf" || out.Result.Content[1].Resource.Blob != "JVBERi0xLjQgdGVzdA==" {
		t.Fatalf("export: %s", rec.Body)
	}
	if q := r.calls[0].query; !strings.Contains(q, "template=admin") || !strings.Contains(q, "merge=1") {
		t.Errorf("query %q", q)
	}
}

func TestSaveDailyReportKeepsOtherParts(t *testing.T) {
	r, h := newRest()
	r.on("GET /v1/workspaces/{ws}/daily-reports/me", 200, map[string]any{"items": []any{
		map[string]any{"date": "2026-11-27", "done": "A", "inProgress": "B", "issues": "C"},
	}})
	r.on("PUT /v1/workspaces/{ws}/daily-reports/me", 200, map[string]any{"ok": true})
	if res := callTool(t, h, "save_daily_report", `{"workspaceId":"w1","date":"2026-11-27","issues":"none"}`); res.Result.IsError {
		t.Fatal(res.Result.Content[0].Text)
	}
	put := body(t, r.calls[1].body)
	if put["done"] != "A" || put["inProgress"] != "B" || put["issues"] != "none" {
		t.Errorf("put %v", put)
	}
}

func TestEndpointFillsPathAndQuery(t *testing.T) {
	r, h := newRest()
	r.on("GET /v1/workspaces/{ws}/projects/{pid}/project-feedbacks", 200, map[string]any{"items": []any{}})
	r.on("DELETE /v1/tasks/{id}/labels/{lid}", 200, nil)
	r.on("PATCH /v1/modules/{id}", 200, map[string]any{})
	callTool(t, h, "list_project_feedbacks", `{"workspaceId":"w 1","projectId":"p1","status":"pending,discussing","hasResponse":false}`)
	callTool(t, h, "label_task", `{"taskId":"t1","labelId":"l1","remove":true}`)
	callTool(t, h, "update_module", `{"moduleId":"m1","endDate":""}`)
	if c := r.calls[0]; c.path != "/v1/workspaces/w 1/projects/p1/project-feedbacks" ||
		!strings.Contains(c.query, "hasResponse=false") || !strings.Contains(c.query, "status=pending%2Cdiscussing") {
		t.Errorf("list: %+v", c)
	}
	if c := r.calls[1]; c.method != http.MethodDelete || c.path != "/v1/tasks/t1/labels/l1" {
		t.Errorf("label: %+v", c)
	}
	if c := r.calls[2]; c.body != `{"endDate":null}` {
		t.Errorf("module clear: %+v", c)
	}
	if res := callTool(t, h, "get_module", `{}`); !res.Result.IsError || !strings.Contains(res.Result.Content[0].Text, "moduleId") {
		t.Errorf("missing arg: %+v", res)
	}
}
