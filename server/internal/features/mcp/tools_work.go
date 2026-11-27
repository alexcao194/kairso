package mcp

import (
	"errors"
	"net/http"
)

// workTools covers the rest of task work: labels, comments, history, attachments, fault holders,
// modules and sprints.
func workTools() []tool {
	task := sProp("Task ID")
	return []tool{
		{
			name:        "delete_task",
			description: "Delete a task permanently. Prefer update_task with archived=true unless the task was created by mistake.",
			schema:      obj([]string{"taskId"}, map[string]any{"taskId": task}),
			run:         endpoint(http.MethodDelete, "/v1/tasks/{taskId}", nil, 0, nil),
			destructive: true,
		},
		{
			name:        "list_task_comments",
			description: "Comments on a task, oldest first.",
			schema:      obj([]string{"taskId"}, map[string]any{"taskId": task}),
			run:         endpoint(http.MethodGet, "/v1/tasks/{taskId}/comments", nil, 0, nil),
		},
		{
			name:        "get_task_history",
			description: "Change history of a task (who changed which field, from what to what), newest first.",
			schema:      obj([]string{"taskId"}, withPage(map[string]any{"taskId": task})),
			run:         endpoint(http.MethodGet, "/v1/tasks/{taskId}/history", nil, 50, nil),
		},
		{
			name:        "list_task_attachments",
			description: "Files attached to a task.",
			schema:      obj([]string{"taskId"}, map[string]any{"taskId": task}),
			run:         endpoint(http.MethodGet, "/v1/tasks/{taskId}/attachments", nil, 0, nil),
		},
		{
			name:        "add_task_attachment",
			description: "Attach a file that is already online (a URL) to a task.",
			schema: obj([]string{"taskId", "fileUrl", "fileName"}, map[string]any{
				"taskId": task, "fileUrl": sProp("Public or signed URL of the file"), "fileName": sProp("File name to show"),
				"contentType": sProp("MIME type, e.g. application/pdf"), "sizeBytes": iProp("Size in bytes, if known"),
			}),
			run: endpoint(http.MethodPost, "/v1/tasks/{taskId}/attachments", nil, 0, func(a args) (any, error) {
				return a.fields(map[string]any{}, "fileUrl", "fileName", "contentType", "sizeBytes"), nil
			}),
		},
		{
			name: "set_fault_holder",
			description: "Record a member as responsible for a fault on a task (e.g. a bug they caused or a " +
				"returned task), or remove them. Used by performance reviews.",
			schema: obj([]string{"taskId", "userId"}, map[string]any{
				"taskId": task, "userId": sProp("Member user ID"),
				"weight": iProp("Share of responsibility, default 1"), "remove": bProp("true to remove instead"),
			}),
			run: func(c *caller, a args) (any, error) {
				run := endpoint(http.MethodPut, "/v1/tasks/{taskId}/fault-holders/{userId}", nil, 0, func(a args) (any, error) {
					w, ok := a.num("weight")
					if !ok || w <= 0 {
						w = 1
					}
					return map[string]any{"weight": w}, nil
				})
				if b, _ := a["remove"].(bool); b {
					run = endpoint(http.MethodDelete, "/v1/tasks/{taskId}/fault-holders/{userId}", nil, 0, nil)
				}
				return run(c, a)
			},
		},
		{
			name:        "list_labels",
			description: "Task labels of a project (id, name, color).",
			schema:      obj([]string{"projectId"}, withPage(map[string]any{"projectId": sProp("Project ID")})),
			run:         endpoint(http.MethodGet, "/v1/projects/{projectId}/labels", nil, 100, nil),
		},
		{
			name:        "create_label",
			description: "Create a task label in a project. Check list_labels first to reuse an existing one.",
			schema: obj([]string{"projectId", "name"}, map[string]any{
				"projectId": sProp("Project ID"), "name": sProp("Label name"), "color": sProp("Hex colour, e.g. #E5484D"),
			}),
			run: endpoint(http.MethodPost, "/v1/projects/{projectId}/labels", nil, 0, func(a args) (any, error) {
				return a.fields(map[string]any{"color": "#8B8D98"}, "name", "color"), nil
			}),
		},
		{
			name:        "label_task",
			description: "Put a label on a task, or take it off.",
			schema: obj([]string{"taskId", "labelId"}, map[string]any{
				"taskId": task, "labelId": sProp("Label ID from list_labels"), "remove": bProp("true to take the label off"),
			}),
			run: func(c *caller, a args) (any, error) {
				method := http.MethodPut
				if b, _ := a["remove"].(bool); b {
					method = http.MethodDelete
				}
				return endpoint(method, "/v1/tasks/{taskId}/labels/{labelId}", nil, 0, nil)(c, a)
			},
		},
		{
			name:        "get_module",
			description: "One module with its dates, status, progress and sprints.",
			schema:      obj([]string{"moduleId"}, map[string]any{"moduleId": sProp("Module ID")}),
			run:         endpoint(http.MethodGet, "/v1/modules/{moduleId}", nil, 0, nil),
		},
		{
			name:        "create_module",
			description: "Create a module (feature area / phase) in a project.",
			schema: obj([]string{"projectId", "name"}, map[string]any{
				"projectId": sProp("Project ID"), "name": sProp("Module name"),
				"startDate": sProp("YYYY-MM-DD"), "endDate": sProp("YYYY-MM-DD"),
			}),
			run: endpoint(http.MethodPost, "/v1/projects/{projectId}/modules", nil, 0, func(a args) (any, error) {
				body := map[string]any{"name": a.str("name")}
				return body, dates(a, body, false)
			}),
		},
		{
			name:        "update_module",
			description: "Change a module. Only the given fields change; pass an empty string to clear a date.",
			schema: obj([]string{"moduleId"}, map[string]any{
				"moduleId": sProp("Module ID"), "name": sProp("New name"),
				"status":    enumProp("New status", "backlog", "in_progress", "done", "released"),
				"startDate": sProp("YYYY-MM-DD, empty to clear"), "endDate": sProp("YYYY-MM-DD, empty to clear"),
			}),
			run: endpoint(http.MethodPatch, "/v1/modules/{moduleId}", nil, 0, func(a args) (any, error) {
				body := a.fields(map[string]any{}, "name", "status")
				if err := dates(a, body, true); err != nil {
					return nil, err
				}
				if len(body) == 0 {
					return nil, errors.New("nothing to update: pass at least one field")
				}
				return body, nil
			}),
		},
		{
			name:        "create_sprint",
			description: "Add a sprint to a module.",
			schema: obj([]string{"moduleId"}, map[string]any{
				"moduleId": sProp("Module ID"), "name": sProp("Sprint name, default Sprint N"), "goal": sProp("Sprint goal"),
				"startDate": sProp("YYYY-MM-DD"), "endDate": sProp("YYYY-MM-DD"),
			}),
			run: endpoint(http.MethodPost, "/v1/modules/{moduleId}/sprints", nil, 0, func(a args) (any, error) {
				return a.fields(map[string]any{}, "name", "goal", "startDate", "endDate"), nil
			}),
		},
		{
			name:        "update_sprint",
			description: "Change a sprint (status, dates, goal, review and retrospective notes). Only the given fields change.",
			schema: obj([]string{"moduleId", "sprintId"}, map[string]any{
				"moduleId": sProp("Module ID"), "sprintId": sProp("Sprint ID from get_module"),
				"name": sProp("Name"), "goal": sProp("Goal"), "startDate": sProp("YYYY-MM-DD"), "endDate": sProp("YYYY-MM-DD"),
				"status":     enumProp("Status", "planned", "active", "completed"),
				"reviewNote": sProp("Sprint review notes"), "retroNote": sProp("Retrospective notes"),
			}),
			run: endpoint(http.MethodPatch, "/v1/modules/{moduleId}/sprints/{sprintId}", nil, 0, func(a args) (any, error) {
				body := a.fields(map[string]any{}, "name", "goal", "startDate", "endDate", "status", "reviewNote", "retroNote")
				if len(body) == 0 {
					return nil, errors.New("nothing to update: pass at least one field")
				}
				return body, nil
			}),
		},
	}
}

// dates copies startDate and endDate as RFC 3339 timestamps; with clear, an empty value is sent
// as null to clear the date.
func dates(a args, body map[string]any, clear bool) error {
	for _, k := range []string{"startDate", "endDate"} {
		if !a.has(k) {
			continue
		}
		d, err := taskDate(a.str(k))
		if err != nil {
			return err
		}
		if d != nil || clear {
			body[k] = d
		}
	}
	return nil
}
