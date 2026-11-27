package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type tool struct {
	name        string
	description string
	schema      map[string]any
	run         func(c *caller, a args) (any, error)
	// readOnly and destructive become MCP tool annotations, so clients can skip confirmation
	// for reads and insist on it for deletions.
	readOnly, destructive bool
}

// annotations are the MCP hints for a tool.
func (t tool) annotations() map[string]any {
	return map[string]any{"readOnlyHint": t.readOnly, "destructiveHint": t.destructive}
}

// fileOut is a tool result carrying a document (PDF, Excel) as an embedded resource.
type fileOut struct {
	name, mime string
	data       []byte
}

// call runs a tool and wraps the outcome as an MCP CallToolResult. Tool failures (bad input,
// permission denied) are results with isError, so the model can read and correct them.
func (h *Handler) call(ctx context.Context, t tool, raw json.RawMessage) map[string]any {
	a := args{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return textResult("arguments must be a JSON object", true)
		}
	}
	out, err := t.run(&caller{ctx: ctx, api: h.api}, a)
	if err != nil {
		return textResult(err.Error(), true)
	}
	if f, ok := out.(*fileOut); ok {
		return map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("%s (%s, %d bytes)", f.name, f.mime, len(f.data))},
				{"type": "resource", "resource": map[string]any{
					"uri": "kairso://export/" + url.PathEscape(f.name), "mimeType": f.mime,
					"blob": base64.StdEncoding.EncodeToString(f.data),
				}},
			},
			"isError": false,
		}
	}
	if s, ok := out.(string); ok {
		return textResult(s, false)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return textResult(err.Error(), true)
	}
	return textResult(string(b), false)
}

func textResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

// ---- in-process REST calls ----

type caller struct {
	ctx context.Context
	api http.Handler
}

type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
}

// do calls a REST endpoint as the API key's owner and returns the JSON body.
func (c *caller) do(method, path string, query url.Values, body any) (json.RawMessage, error) {
	rec, err := c.send(method, path, query, body)
	if err != nil {
		return nil, err
	}
	if rec.body.Len() == 0 {
		return json.RawMessage(`{"ok":true}`), nil
	}
	return json.RawMessage(rec.body.Bytes()), nil
}

// file calls a REST endpoint that answers with a document and returns it.
func (c *caller) file(path string, query url.Values, name, mime string) (*fileOut, error) {
	rec, err := c.send(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if ct := rec.header.Get("Content-Type"); ct != "" {
		mime = strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
	}
	if cd := rec.header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mimeParse(cd); err == nil && params["filename"] != "" {
			name = params["filename"]
		}
	}
	return &fileOut{name: name, mime: mime, data: rec.body.Bytes()}, nil
}

func (c *caller) send(method, path string, query url.Values, body any) (*recorder, error) {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(c.ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := &recorder{header: http.Header{}}
	c.api.ServeHTTP(rec, req)
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	if rec.status >= 400 {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		msg := strings.TrimSpace(rec.body.String())
		if json.Unmarshal(rec.body.Bytes(), &e) == nil {
			if e.Error != "" {
				msg = e.Error
			} else if e.Message != "" {
				msg = e.Message
			}
		}
		return nil, fmt.Errorf("HTTP %d: %s", rec.status, msg)
	}
	return rec, nil
}

var mimeParse = mime.ParseMediaType

func p(id string) string { return url.PathEscape(id) }

// endpoint builds a tool runner for a plain REST call. Every {name} in path is filled from the
// required argument of that name; query lists optional string/boolean arguments sent as query
// parameters; page > 0 adds pagination with that default limit; body, when set, builds the
// request body.
func endpoint(method, path string, query []string, page int, body func(a args) (any, error)) func(*caller, args) (any, error) {
	return func(c *caller, a args) (any, error) {
		target := path
		for {
			i := strings.Index(target, "{")
			if i < 0 {
				break
			}
			j := strings.Index(target[i:], "}") + i
			v, err := a.need(target[i+1 : j])
			if err != nil {
				return nil, err
			}
			target = target[:i] + p(v) + target[j+1:]
		}
		q := url.Values{}
		for _, k := range query {
			if b, ok := a[k].(bool); ok {
				q.Set(k, strconv.FormatBool(b))
			} else if v := a.str(k); v != "" {
				q.Set(k, v)
			}
		}
		if page > 0 {
			q = a.pageQuery(q, page)
		}
		var b any
		if body != nil {
			var err error
			if b, err = body(a); err != nil {
				return nil, err
			}
		}
		return c.do(method, target, q, b)
	}
}

// fields copies the given arguments that are present into body: strings trimmed, numbers and
// booleans as they are.
func (a args) fields(body map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		switch v := a[k].(type) {
		case string:
			body[k] = strings.TrimSpace(v)
		case float64, bool, []any, map[string]any:
			body[k] = v
		}
	}
	return body
}

// ---- argument helpers ----

type args map[string]any

func (a args) has(k string) bool { _, ok := a[k]; return ok && a[k] != nil }

func (a args) str(k string) string {
	switch v := a[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func (a args) need(k string) (string, error) {
	if v := a.str(k); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("missing required argument %q", k)
}

func (a args) num(k string) (int, bool) {
	switch v := a[k].(type) {
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	}
	return 0, false
}

func (a args) strs(k string) []string {
	var out []string
	switch v := a[k].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case string:
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// pageQuery copies "page" (and a capped "limit") into query parameters.
func (a args) pageQuery(q url.Values, defaultLimit int) url.Values {
	if q == nil {
		q = url.Values{}
	}
	limit := defaultLimit
	if n, ok := a.num("limit"); ok && n > 0 {
		limit = min(n, 100)
	}
	q.Set("limit", strconv.Itoa(limit))
	if n, ok := a.num("page"); ok && n > 0 {
		q.Set("page", strconv.Itoa(n))
	}
	return q
}

var vnZone = time.FixedZone("ICT", 7*3600)

// taskDate turns "YYYY-MM-DD" (Vietnam time) or an RFC 3339 timestamp into RFC 3339; "" clears.
func taskDate(v string) (any, error) {
	if v == "" {
		return nil, nil
	}
	if t, err := time.ParseInLocation(time.DateOnly, v, vnZone); err == nil {
		return t.Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.Format(time.RFC3339), nil
	}
	return nil, fmt.Errorf("invalid date %q, use YYYY-MM-DD", v)
}

// ---- schema helpers ----

func obj(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func sProp(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func iProp(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func bProp(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func enumProp(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}
func arrProp(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}

var (
	statuses     = []string{"backlog", "todo", "in_progress", "review", "done", "canceled"}
	pageProps    = map[string]any{"page": iProp("Page number, from 1"), "limit": iProp("Items per page, up to 100")}
	participants = arrProp("People or groups. Give userId (from list_members) for a member, or only name for an outside person/department.",
		obj([]string{"name"}, map[string]any{"name": sProp("Display name"), "userId": sProp("Member user ID, optional")}))
	meetingItem = obj([]string{"section", "content"}, map[string]any{
		"id":       sProp("Existing item ID; keep it when editing an item, omit for a new item"),
		"section":  enumProp("deployed = already delivered, planned = to implement next, backlog = later", "deployed", "planned", "backlog"),
		"module":   sProp("Module / area name"),
		"content":  sProp("What was discussed or requested"),
		"note":     sProp("Extra note"),
		"priority": bProp("Marked as priority (planned/backlog)"),
		"status":   enumProp("For deployed items: done = confirmed, open = waiting for update", "open", "done"),
		"taskIds":  arrProp("IDs of Kairso tasks linked to this item", map[string]any{"type": "string"}),
	})
	sectionNotes = map[string]any{"type": "object", "description": "Free notes per section, keys deployed / planned / backlog",
		"properties": map[string]any{"deployed": sProp(""), "planned": sProp(""), "backlog": sProp("")}}
)

func withPage(props map[string]any) map[string]any {
	for k, v := range pageProps {
		props[k] = v
	}
	return props
}

// ---- tools ----

func toolset() []tool {
	all := append(coreTools(), workTools()...)
	all = append(all, quoteTools()...)
	all = append(all, feedbackTools()...)
	all = append(all, reportTools()...)
	for i, t := range all {
		for _, prefix := range []string{"whoami", "list_", "get_", "search_", "export_"} {
			if strings.HasPrefix(t.name, prefix) {
				all[i].readOnly = true
			}
		}
	}
	return all
}

func coreTools() []tool {
	return []tool{
		{
			name:        "whoami",
			description: "The Kairso user this API key acts as.",
			schema:      obj(nil, map[string]any{}),
			run: func(c *caller, a args) (any, error) {
				return c.do(http.MethodGet, "/v1/users/me", nil, nil)
			},
		},
		{
			name:        "list_workspaces",
			description: "Workspaces the user belongs to. Start here to get a workspaceId.",
			schema:      obj(nil, withPage(map[string]any{})),
			run: func(c *caller, a args) (any, error) {
				return c.do(http.MethodGet, "/v1/workspaces", a.pageQuery(nil, 100), nil)
			},
		},
		{
			name:        "list_projects",
			description: "Projects in a workspace (id, alias, name).",
			schema:      obj([]string{"workspaceId"}, withPage(map[string]any{"workspaceId": sProp("Workspace ID")})),
			run: func(c *caller, a args) (any, error) {
				ws, err := a.need("workspaceId")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/v1/workspaces/"+p(ws)+"/projects", a.pageQuery(nil, 100), nil)
			},
		},
		{
			name:        "list_members",
			description: "Members of a workspace, with their user IDs (needed to assign tasks or name meeting attendees).",
			schema:      obj([]string{"workspaceId"}, withPage(map[string]any{"workspaceId": sProp("Workspace ID")})),
			run: func(c *caller, a args) (any, error) {
				ws, err := a.need("workspaceId")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/v1/workspaces/"+p(ws)+"/members", a.pageQuery(nil, 100), nil)
			},
		},
		{
			name:        "list_modules",
			description: "Modules (feature areas) of a project.",
			schema:      obj([]string{"projectId"}, withPage(map[string]any{"projectId": sProp("Project ID")})),
			run: func(c *caller, a args) (any, error) {
				pid, err := a.need("projectId")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/v1/projects/"+p(pid)+"/modules", a.pageQuery(nil, 100), nil)
			},
		},
		{
			name:        "search_tasks",
			description: "Tasks of a project, optionally filtered by text, status, module or worker.",
			schema: obj([]string{"projectId"}, withPage(map[string]any{
				"projectId": sProp("Project ID"),
				"q":         sProp("Text to search in title/code"),
				"status":    enumProp("Only this status", statuses...),
				"moduleId":  sProp("Only tasks of this module"),
				"workerId":  sProp("Only tasks assigned to this user ID"),
				"archived":  bProp("true to list archived tasks instead"),
			})),
			run: func(c *caller, a args) (any, error) {
				pid, err := a.need("projectId")
				if err != nil {
					return nil, err
				}
				q := url.Values{}
				for _, k := range []string{"q", "status", "moduleId", "workerId"} {
					if v := a.str(k); v != "" {
						q.Set(k, v)
					}
				}
				if b, ok := a["archived"].(bool); ok {
					q.Set("archived", strconv.FormatBool(b))
				}
				return c.do(http.MethodGet, "/v1/projects/"+p(pid)+"/tasks", a.pageQuery(q, 50), nil)
			},
		},
		{
			name:        "list_my_tasks",
			description: "Tasks assigned to the user in a workspace, across projects.",
			schema: obj([]string{"workspaceId"}, withPage(map[string]any{
				"workspaceId": sProp("Workspace ID"),
				"q":           sProp("Text to search"),
				"status":      enumProp("Only this status", statuses...),
				"projectId":   sProp("Only this project"),
			})),
			run: func(c *caller, a args) (any, error) {
				ws, err := a.need("workspaceId")
				if err != nil {
					return nil, err
				}
				q := url.Values{}
				for _, k := range []string{"q", "status", "projectId"} {
					if v := a.str(k); v != "" {
						q.Set(k, v)
					}
				}
				return c.do(http.MethodGet, "/v1/workspaces/"+p(ws)+"/my-tasks", a.pageQuery(q, 50), nil)
			},
		},
		{
			name:        "get_task",
			description: "Full details of one task.",
			schema:      obj([]string{"taskId"}, map[string]any{"taskId": sProp("Task ID")}),
			run: func(c *caller, a args) (any, error) {
				id, err := a.need("taskId")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/v1/tasks/"+p(id), nil, nil)
			},
		},
		{
			name:        "create_task",
			description: "Create a task in a project, optionally assigning it to members.",
			schema: obj([]string{"projectId", "title"}, map[string]any{
				"projectId":   sProp("Project ID"),
				"title":       sProp("Task title"),
				"description": sProp("Markdown description"),
				"status":      enumProp("Initial status, default todo", statuses...),
				"priority":    iProp("1 (low) .. 4 (urgent), default 2"),
				"moduleId":    sProp("Module ID"),
				"parentId":    sProp("Parent task ID, to create a subtask"),
				"startDate":   sProp("YYYY-MM-DD"),
				"endDate":     sProp("Deadline, YYYY-MM-DD"),
				"estimation":  iProp("Estimate in hours"),
				"assigneeIds": arrProp("User IDs to assign as workers", map[string]any{"type": "string"}),
			}),
			run: createTask,
		},
		{
			name:        "update_task",
			description: "Change fields of a task. Only the given fields change; pass an empty string to clear moduleId, startDate or endDate.",
			schema: obj([]string{"taskId"}, map[string]any{
				"taskId":      sProp("Task ID"),
				"title":       sProp("New title"),
				"description": sProp("New markdown description"),
				"status":      enumProp("New status", statuses...),
				"priority":    iProp("1 (low) .. 4 (urgent)"),
				"moduleId":    sProp("Module ID, empty to clear"),
				"startDate":   sProp("YYYY-MM-DD, empty to clear"),
				"endDate":     sProp("YYYY-MM-DD, empty to clear"),
				"estimation":  iProp("Estimate in hours"),
				"archived":    bProp("Archive or unarchive"),
			}),
			run: updateTask,
		},
		{
			name:        "assign_task",
			description: "Assign a member to a task as a worker, or remove them.",
			schema: obj([]string{"taskId", "userId"}, map[string]any{
				"taskId": sProp("Task ID"),
				"userId": sProp("Member user ID"),
				"remove": bProp("true to unassign instead"),
			}),
			run: func(c *caller, a args) (any, error) {
				id, err := a.need("taskId")
				if err != nil {
					return nil, err
				}
				uid, err := a.need("userId")
				if err != nil {
					return nil, err
				}
				path := "/v1/tasks/" + p(id) + "/workers/" + p(uid)
				if b, _ := a["remove"].(bool); b {
					return c.do(http.MethodDelete, path, nil, nil)
				}
				return c.do(http.MethodPut, path, nil, map[string]any{"weight": 1})
			},
		},
		{
			name:        "add_task_comment",
			description: "Post a markdown comment on a task.",
			schema:      obj([]string{"taskId", "content"}, map[string]any{"taskId": sProp("Task ID"), "content": sProp("Markdown comment")}),
			run: func(c *caller, a args) (any, error) {
				id, err := a.need("taskId")
				if err != nil {
					return nil, err
				}
				content, err := a.need("content")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodPost, "/v1/tasks/"+p(id)+"/comments", nil, map[string]any{"content": content})
			},
		},
		{
			name:        "list_meetings",
			description: "Meeting minutes of a workspace, newest first.",
			schema: obj([]string{"workspaceId"}, withPage(map[string]any{
				"workspaceId": sProp("Workspace ID"),
				"projectId":   sProp("Only minutes of this project"),
				"q":           sProp("Text to search"),
			})),
			run: func(c *caller, a args) (any, error) {
				ws, err := a.need("workspaceId")
				if err != nil {
					return nil, err
				}
				q := url.Values{}
				for _, k := range []string{"q", "projectId"} {
					if v := a.str(k); v != "" {
						q.Set(k, v)
					}
				}
				return c.do(http.MethodGet, "/v1/workspaces/"+p(ws)+"/meetings", a.pageQuery(q, 20), nil)
			},
		},
		{
			name:        "get_meeting",
			description: "Full meeting minutes, including items and their IDs.",
			schema:      obj([]string{"meetingId"}, map[string]any{"meetingId": sProp("Meeting minutes ID")}),
			run: func(c *caller, a args) (any, error) {
				id, err := a.need("meetingId")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/v1/meetings/"+p(id), nil, nil)
			},
		},
		{
			name: "create_meeting",
			description: "Create meeting minutes (biên bản họp). Items are grouped in sections: deployed (delivered, " +
				"waiting for confirmation), planned (to implement), backlog.",
			schema: obj([]string{"workspaceId", "title", "meetingDate"}, map[string]any{
				"workspaceId":  sProp("Workspace ID"),
				"projectId":    sProp("Project the meeting is about, optional"),
				"title":        sProp("Meeting title"),
				"meetingDate":  sProp("YYYY-MM-DD"),
				"location":     sProp("Place or format, e.g. Google Meet"),
				"attendees":    participants,
				"recorders":    participants,
				"summary":      sProp("Overall content / conclusions"),
				"items":        arrProp("Discussion items", meetingItem),
				"sectionNotes": sectionNotes,
			}),
			run: createMeeting,
		},
		{
			name: "update_meeting",
			description: "Change meeting minutes. Only the given fields change. items REPLACES the whole list: " +
				"call get_meeting first and send every item to keep (with its id). To only add rows use add_meeting_items.",
			schema: obj([]string{"meetingId"}, map[string]any{
				"meetingId":    sProp("Meeting minutes ID"),
				"title":        sProp("Meeting title"),
				"meetingDate":  sProp("YYYY-MM-DD"),
				"location":     sProp("Place or format"),
				"attendees":    participants,
				"recorders":    participants,
				"summary":      sProp("Overall content / conclusions"),
				"items":        arrProp("The complete list of items", meetingItem),
				"sectionNotes": sectionNotes,
			}),
			run: updateMeeting,
		},
		{
			name:        "add_meeting_items",
			description: "Append items to existing meeting minutes, keeping the current ones.",
			schema: obj([]string{"meetingId", "items"}, map[string]any{
				"meetingId": sProp("Meeting minutes ID"),
				"items":     arrProp("Items to add", meetingItem),
			}),
			run: addMeetingItems,
		},
	}
}

func createTask(c *caller, a args) (any, error) {
	pid, err := a.need("projectId")
	if err != nil {
		return nil, err
	}
	title, err := a.need("title")
	if err != nil {
		return nil, err
	}
	body := map[string]any{"title": title, "status": "todo", "priority": 2}
	if v := a.str("status"); v != "" {
		body["status"] = v
	}
	if v := a.str("description"); v != "" {
		body["description"] = v
	}
	for _, k := range []string{"moduleId", "parentId"} {
		if v := a.str(k); v != "" {
			body[k] = v
		}
	}
	if n, ok := a.num("priority"); ok {
		body["priority"] = n
	}
	if n, ok := a.num("estimation"); ok {
		body["estimation"] = n
	}
	for _, k := range []string{"startDate", "endDate"} {
		d, err := taskDate(a.str(k))
		if err != nil {
			return nil, err
		}
		if d != nil {
			body[k] = d
		}
	}
	created, err := c.do(http.MethodPost, "/v1/projects/"+p(pid)+"/tasks", nil, body)
	if err != nil {
		return nil, err
	}
	assignees := a.strs("assigneeIds")
	if len(assignees) == 0 {
		return created, nil
	}
	var t struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created, &t); err != nil || t.ID == "" {
		return created, nil
	}
	var failed []string
	for _, uid := range assignees {
		if _, err := c.do(http.MethodPut, "/v1/tasks/"+p(t.ID)+"/workers/"+p(uid), nil, map[string]any{"weight": 1}); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", uid, err))
		}
	}
	task, err := c.do(http.MethodGet, "/v1/tasks/"+p(t.ID), nil, nil)
	if err != nil {
		task = created
	}
	if len(failed) > 0 {
		return map[string]any{"task": task, "assignErrors": failed}, nil
	}
	return task, nil
}

func updateTask(c *caller, a args) (any, error) {
	id, err := a.need("taskId")
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	for _, k := range []string{"title", "description", "status"} {
		if a.has(k) {
			body[k] = a.str(k)
		}
	}
	if a.has("moduleId") {
		if v := a.str("moduleId"); v != "" {
			body["moduleId"] = v
		} else {
			body["moduleId"] = nil
		}
	}
	for _, k := range []string{"startDate", "endDate"} {
		if a.has(k) {
			d, err := taskDate(a.str(k))
			if err != nil {
				return nil, err
			}
			body[k] = d
		}
	}
	for _, k := range []string{"priority", "estimation"} {
		if n, ok := a.num(k); ok {
			body[k] = n
		}
	}
	if b, ok := a["archived"].(bool); ok {
		body["archived"] = b
	}
	if len(body) == 0 {
		return nil, errors.New("nothing to update: pass at least one field")
	}
	return c.do(http.MethodPatch, "/v1/tasks/"+p(id), nil, body)
}

// meetingItems converts tool items (with taskIds) to the API shape (tasks: [{id, code}]),
// looking up the display code of linked tasks the caller can see.
func meetingItems(c *caller, v any) ([]map[string]any, error) {
	items, err := parseMeetingItems(v)
	if err != nil {
		return nil, err
	}
	codes := map[string]string{}
	for _, it := range items {
		for _, t := range it["tasks"].([]map[string]any) {
			id, _ := t["id"].(string)
			if code, _ := t["code"].(string); code != "" || id == "" {
				continue
			}
			if _, done := codes[id]; !done {
				codes[id] = ""
				if raw, err := c.do(http.MethodGet, "/v1/tasks/"+p(id), nil, nil); err == nil {
					var task struct {
						Code string `json:"code"`
					}
					if json.Unmarshal(raw, &task) == nil {
						codes[id] = task.Code
					}
				}
			}
			t["code"] = codes[id]
		}
	}
	return items, nil
}

func parseMeetingItems(v any) ([]map[string]any, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, errors.New("items must be an array")
	}
	out := make([]map[string]any, 0, len(list))
	for i, x := range list {
		m, ok := x.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("items[%d] must be an object", i)
		}
		it := args(m)
		item := map[string]any{
			"id": it.str("id"), "section": it.str("section"), "module": it.str("module"),
			"content": it.str("content"), "note": it.str("note"), "status": it.str("status"),
		}
		if b, ok := m["priority"].(bool); ok {
			item["priority"] = b
		}
		tasks := []map[string]any{}
		for _, tid := range it.strs("taskIds") {
			tasks = append(tasks, map[string]any{"id": tid})
		}
		// Keep task links from a get_meeting response passed back unchanged.
		if raw, ok := m["tasks"].([]any); ok && len(tasks) == 0 {
			for _, t := range raw {
				if tm, ok := t.(map[string]any); ok {
					tasks = append(tasks, map[string]any{"id": args(tm).str("id"), "code": args(tm).str("code")})
				}
			}
		}
		item["tasks"] = tasks
		out = append(out, item)
	}
	return out, nil
}

func meetingParticipants(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, x := range list {
		switch m := x.(type) {
		case map[string]any:
			out = append(out, map[string]any{"name": args(m).str("name"), "userId": args(m).str("userId")})
		case string:
			out = append(out, map[string]any{"name": strings.TrimSpace(m)})
		}
	}
	return out
}

// meetingBody copies the meeting fields present in a into an API body.
func meetingBody(c *caller, a args) (map[string]any, error) {
	body := map[string]any{}
	for _, k := range []string{"title", "meetingDate", "location", "summary", "projectId"} {
		if a.has(k) {
			body[k] = a.str(k)
		}
	}
	for _, k := range []string{"attendees", "recorders"} {
		if a.has(k) {
			body[k] = meetingParticipants(a[k])
		}
	}
	if a.has("items") {
		items, err := meetingItems(c, a["items"])
		if err != nil {
			return nil, err
		}
		body["items"] = items
	}
	if a.has("sectionNotes") {
		notes := map[string]string{}
		if m, ok := a["sectionNotes"].(map[string]any); ok {
			for k := range m {
				notes[k] = args(m).str(k)
			}
		}
		body["sectionNotes"] = notes
	}
	return body, nil
}

func createMeeting(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	if _, err := a.need("title"); err != nil {
		return nil, err
	}
	if _, err := a.need("meetingDate"); err != nil {
		return nil, err
	}
	body, err := meetingBody(c, a)
	if err != nil {
		return nil, err
	}
	for _, k := range []string{"attendees", "recorders"} {
		if _, ok := body[k]; !ok {
			body[k] = []any{}
		}
	}
	if _, ok := body["items"]; !ok {
		body["items"] = []any{}
	}
	return c.do(http.MethodPost, "/v1/workspaces/"+p(ws)+"/meetings", nil, body)
}

func updateMeeting(c *caller, a args) (any, error) {
	id, err := a.need("meetingId")
	if err != nil {
		return nil, err
	}
	body, err := meetingBody(c, a)
	if err != nil {
		return nil, err
	}
	delete(body, "projectId")
	if len(body) == 0 {
		return nil, errors.New("nothing to update: pass at least one field")
	}
	return c.do(http.MethodPatch, "/v1/meetings/"+p(id), nil, body)
}

func addMeetingItems(c *caller, a args) (any, error) {
	id, err := a.need("meetingId")
	if err != nil {
		return nil, err
	}
	added, err := meetingItems(c, a["items"])
	if err != nil {
		return nil, err
	}
	raw, err := c.do(http.MethodGet, "/v1/meetings/"+p(id), nil, nil)
	if err != nil {
		return nil, err
	}
	var current struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return nil, err
	}
	items := append(current.Items, added...)
	return c.do(http.MethodPatch, "/v1/meetings/"+p(id), nil, map[string]any{"items": items})
}
