// Package mcp serves Kairso as a Model Context Protocol server (Streamable HTTP transport,
// stateless, JSON responses) so AI assistants such as Claude Code can read and create tasks
// and meeting minutes. Callers authenticate with a personal API key
// ("Authorization: Bearer ksk_..."); every tool runs through the regular REST handlers as the
// key's owner, so permissions are exactly the user's own.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/pkg/httpx"
)

const (
	serverName    = "kairso"
	serverVersion = "1.0.0"
	latestVersion = "2025-06-18"
	maxBodyBytes  = 1 << 20
)

var supportedVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `Kairso is a project management workspace (tasks, modules, meeting minutes).
Typical flow: list_workspaces → list_projects (workspaceId) → work with tasks or meetings.
IDs are UUIDs returned by the list tools. Task priority is 1..4 (4 = urgent). Task status is one
of backlog, todo, in_progress, review, done, canceled. Dates are YYYY-MM-DD.
Use list_members to find user IDs before assigning tasks or naming meeting attendees.`

// Handler serves POST /mcp.
type Handler struct {
	api   http.Handler
	keys  middleware.APIKeyResolver
	tools []tool
}

// NewHandler builds the MCP endpoint. api is the REST mux (paths without the /api prefix).
func NewHandler(api http.Handler, keys middleware.APIKeyResolver) *Handler {
	return &Handler{api: api, keys: keys, tools: toolset()}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /mcp", h.serve)
	mux.HandleFunc("GET /mcp", methodNotAllowed)
	mux.HandleFunc("DELETE /mcp", methodNotAllowed)
}

// methodNotAllowed: this server keeps no sessions and opens no server-to-client stream.
func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "POST")
	httpx.Error(w, http.StatusMethodNotAllowed, "Kairso MCP chỉ nhận POST")
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	key := middleware.BearerAPIKey(r)
	if key == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="kairso"`)
		httpx.Error(w, http.StatusUnauthorized, "Thiếu API key (Authorization: Bearer ksk_...)")
		return
	}
	userID, err := h.keys.ResolveAPIKey(r.Context(), key)
	if err != nil || userID == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="kairso", error="invalid_token"`)
		httpx.Error(w, http.StatusUnauthorized, "API key không hợp lệ hoặc đã bị thu hồi")
		return
	}
	ctx := middleware.WithAPIKeyUser(r.Context(), userID)

	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&raw); err != nil {
		writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "parse error"}})
		return
	}

	// A JSON array is a batch (allowed by the 2025-03-26 revision).
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
		var batch []rpcRequest
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeInvalidRequest, "invalid batch"}})
			return
		}
		var out []rpcResponse
		for _, req := range batch {
			if resp, ok := h.handle(ctx, req); ok {
				out = append(out, resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, out)
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeInvalidRequest, "invalid request"}})
		return
	}
	resp, ok := h.handle(ctx, req)
	if !ok {
		// Notifications and responses get no body.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// handle runs one JSON-RPC message. ok is false for notifications, which get no reply.
func (h *Handler) handle(ctx context.Context, req rpcRequest) (rpcResponse, bool) {
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return rpcResponse{}, false
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := latestVersion
		if slices.Contains(supportedVersions, p.ProtocolVersion) {
			version = p.ProtocolVersion
		}
		resp.Result = map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": serverName, "title": "Kairso", "version": serverVersion},
			"instructions":    instructions,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		list := make([]map[string]any, 0, len(h.tools))
		for _, t := range h.tools {
			list = append(list, map[string]any{"name": t.name, "description": t.description, "inputSchema": t.schema})
		}
		resp.Result = map[string]any{"tools": list}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			resp.Error = &rpcError{codeInvalidParams, "invalid params"}
			break
		}
		idx := slices.IndexFunc(h.tools, func(t tool) bool { return t.name == p.Name })
		if idx < 0 {
			resp.Error = &rpcError{codeInvalidParams, "unknown tool: " + p.Name}
			break
		}
		resp.Result = h.call(ctx, h.tools[idx], p.Arguments)
	default:
		resp.Error = &rpcError{codeMethodNotFound, "method not found: " + req.Method}
	}
	return resp, true
}
