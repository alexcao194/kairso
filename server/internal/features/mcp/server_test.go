package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/pkg/httpx"
)

type fakeKeys struct{}

func (fakeKeys) ResolveAPIKey(_ context.Context, key string) (string, error) {
	if key == "ksk_good" {
		return "user-1", nil
	}
	return "", errors.New("bad key")
}

type seen struct {
	method, path, query, body, user string
	apiKey                          bool
}

// fakeAPI stands in for the REST mux and records what the tools call.
func fakeAPI(calls *[]seen) http.Handler {
	mux := http.NewServeMux()
	record := func(r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		uid, _ := middleware.GetUserIDFromContext(r.Context())
		*calls = append(*calls, seen{r.Method, r.URL.Path, r.URL.RawQuery, string(b), uid, middleware.IsAPIKeyRequest(r.Context())})
	}
	mux.HandleFunc("GET /v1/workspaces", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 200, map[string]any{"items": []any{map[string]any{"id": "ws1"}}})
	})
	mux.HandleFunc("POST /v1/projects/{id}/tasks", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 201, map[string]any{"id": "t1", "title": "x"})
	})
	mux.HandleFunc("PUT /v1/tasks/{id}/workers/{uid}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 200, map[string]any{})
	})
	mux.HandleFunc("GET /v1/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 200, map[string]any{"id": "t1", "workers": []any{"u2"}})
	})
	mux.HandleFunc("PATCH /v1/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.Error(w, 403, "Bạn không có quyền")
	})
	mux.HandleFunc("GET /v1/meetings/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 200, map[string]any{"items": []any{map[string]any{"id": "i1", "section": "planned", "content": "old", "tasks": []any{}}}})
	})
	mux.HandleFunc("PATCH /v1/meetings/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		httpx.JSON(w, 200, map[string]any{"ok": true})
	})
	return mux
}

func post(t *testing.T, h http.Handler, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func setup(calls *[]seen) http.Handler {
	mux := http.NewServeMux()
	NewHandler(fakeAPI(calls), fakeKeys{}).RegisterRoutes(mux)
	return mux
}

type callResult struct {
	Result struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	} `json:"result"`
}

func callTool(t *testing.T, h http.Handler, name, args string) callResult {
	t.Helper()
	rec := post(t, h, "ksk_good", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out callResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAuth(t *testing.T) {
	var calls []seen
	h := setup(&calls)
	if rec := post(t, h, "", `{"jsonrpc":"2.0","id":1,"method":"ping"}`); rec.Code != 401 {
		t.Fatalf("no key: got %d", rec.Code)
	}
	if rec := post(t, h, "ksk_bad", `{"jsonrpc":"2.0","id":1,"method":"ping"}`); rec.Code != 401 {
		t.Fatalf("bad key: got %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Fatalf("GET: got %d", rec.Code)
	}
}

func TestInitializeAndList(t *testing.T) {
	var calls []seen
	h := setup(&calls)
	rec := post(t, h, "ksk_good", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	var init struct {
		Result struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &init)
	if init.Result.ProtocolVersion != "2025-03-26" || init.Result.Capabilities["tools"] == nil {
		t.Fatalf("initialize: %s", rec.Body)
	}
	if rec := post(t, h, "ksk_good", `{"jsonrpc":"2.0","method":"notifications/initialized"}`); rec.Code != 202 {
		t.Fatalf("notification: got %d", rec.Code)
	}
	rec = post(t, h, "ksk_good", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var list struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	names := map[string]bool{}
	for _, tl := range list.Result.Tools {
		names[tl.Name] = true
		if tl.InputSchema["type"] != "object" {
			t.Errorf("%s: schema type %v", tl.Name, tl.InputSchema["type"])
		}
	}
	for _, want := range []string{"list_workspaces", "create_task", "update_task", "create_meeting", "add_meeting_items"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}
	rec = post(t, h, "ksk_good", `{"jsonrpc":"2.0","id":3,"method":"nope"}`)
	if !strings.Contains(rec.Body.String(), "-32601") {
		t.Errorf("unknown method: %s", rec.Body)
	}
}

func TestToolsRunAsKeyOwner(t *testing.T) {
	var calls []seen
	h := setup(&calls)

	if r := callTool(t, h, "list_workspaces", `{}`); r.Result.IsError || !strings.Contains(r.Result.Content[0].Text, "ws1") {
		t.Fatalf("list_workspaces: %+v", r)
	}
	if calls[0].user != "user-1" || !calls[0].apiKey || calls[0].query != "limit=100" {
		t.Fatalf("call context: %+v", calls[0])
	}

	calls = nil
	r := callTool(t, h, "create_task", `{"projectId":"p 1","title":"Fix login","priority":4,"endDate":"2026-12-01","assigneeIds":["u2"]}`)
	if r.Result.IsError {
		t.Fatalf("create_task: %+v", r)
	}
	if len(calls) != 3 || calls[0].path != "/v1/projects/p%201/tasks" && calls[0].path != "/v1/projects/p 1/tasks" {
		t.Fatalf("create_task calls: %+v", calls)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(calls[0].body), &body)
	if body["title"] != "Fix login" || body["status"] != "todo" || body["priority"] != float64(4) || body["endDate"] != "2026-12-01T00:00:00+07:00" {
		t.Fatalf("create_task body: %v", body)
	}
	if calls[1].method != http.MethodPut || calls[1].path != "/v1/tasks/t1/workers/u2" {
		t.Fatalf("assign call: %+v", calls[1])
	}

	r = callTool(t, h, "update_task", `{"taskId":"t1","moduleId":""}`)
	if !r.Result.IsError || !strings.Contains(r.Result.Content[0].Text, "403") {
		t.Fatalf("update_task should surface API error: %+v", r)
	}
	if got := calls[len(calls)-1].body; !strings.Contains(got, `"moduleId":null`) {
		t.Fatalf("clear moduleId body: %s", got)
	}

	if r := callTool(t, h, "create_task", `{"title":"x"}`); !r.Result.IsError {
		t.Fatalf("missing projectId should fail")
	}
}

func TestAddMeetingItemsKeepsExisting(t *testing.T) {
	var calls []seen
	h := setup(&calls)
	r := callTool(t, h, "add_meeting_items", `{"meetingId":"m1","items":[{"section":"planned","content":"new","taskIds":["t9"]}]}`)
	if r.Result.IsError {
		t.Fatalf("add_meeting_items: %+v", r)
	}
	patch := calls[len(calls)-1]
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal([]byte(patch.body), &body)
	if patch.method != http.MethodPatch || len(body.Items) != 2 || body.Items[0]["id"] != "i1" || body.Items[1]["content"] != "new" {
		t.Fatalf("patch body: %s", patch.body)
	}
}
