package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var feedbackStatuses = []string{"pending", "discussing", "acknowledged", "in_progress", "resolved", "deployed", "canceled"}

const projectPath = "/v1/workspaces/{workspaceId}/projects/{projectId}"

// feedbackTools covers customer feedback on projects (góp ý / báo lỗi from the public board) and
// the questions the team asks customers there.
func feedbackTools() []tool {
	wp := func(extra map[string]any) map[string]any {
		return merge(map[string]any{"workspaceId": sProp("Workspace ID"), "projectId": sProp("Project ID")}, extra)
	}
	fb := sProp("Project feedback ID")
	return []tool{
		{
			name: "list_project_feedbacks",
			description: "Customer feedback (feature requests, improvements, bugs) of a project, newest first. " +
				"Filters take comma-separated values.",
			schema: obj([]string{"workspaceId", "projectId"}, withPage(wp(map[string]any{
				"status":       sProp("Statuses: " + strings.Join(feedbackStatuses, ", ")),
				"feedbackType": sProp("Types: feature, improvement, bug"),
				"priority":     sProp("Priorities 1..4 (4 = urgent), e.g. 3,4"),
				"q":            sProp("Text to search (submitter, feature, content, response)"),
				"hasResponse":  bProp("Only feedback with (true) or without (false) a response"),
				"sort":         sProp("e.g. priority:desc,submitted:asc. Fields: code, priority, feature, submitter, company, status, expectedFix, submitted"),
			}))),
			run: endpoint(http.MethodGet, projectPath+"/project-feedbacks",
				[]string{"status", "feedbackType", "priority", "q", "hasResponse", "sort"}, 50, nil),
		},
		{
			name: "create_project_feedback",
			description: "Record customer feedback on a project for them, e.g. from an email or a call. It " +
				"shows on the project's public feedback board.",
			schema: obj([]string{"workspaceId", "projectId", "content", "feedbackType", "featureName", "priority",
				"submitterName", "submitterCompany"}, wp(map[string]any{
				"content":          sProp("The feedback, in the customer's words"),
				"feedbackType":     enumProp("Kind", "feature", "improvement", "bug"),
				"featureName":      sProp("Feature / screen concerned"),
				"priority":         iProp("1 (low) .. 4 (urgent)"),
				"submitterName":    sProp("Customer contact who gave it"),
				"submitterCompany": sProp("Customer company"),
			})),
			run: endpoint(http.MethodPost, projectPath+"/project-feedbacks", nil, 0, func(a args) (any, error) {
				return a.fields(map[string]any{}, "content", "feedbackType", "featureName", "priority", "submitterName", "submitterCompany"), nil
			}),
		},
		{
			name:        "set_feedback_status",
			description: "Move customer feedback through triage.",
			schema: obj([]string{"feedbackId", "status"}, map[string]any{
				"feedbackId": fb, "status": enumProp("New status", feedbackStatuses...),
			}),
			run: endpoint(http.MethodPatch, "/v1/project-feedbacks/{feedbackId}/status", nil, 0, func(a args) (any, error) {
				return map[string]any{"status": a.str("status")}, nil
			}),
		},
		{
			name: "respond_to_feedback",
			description: "Set the team's answer to customer feedback, shown publicly on the feedback board, " +
				"optionally with the expected fix date. Replaces any earlier answer.",
			schema: obj([]string{"feedbackId", "response"}, map[string]any{
				"feedbackId": fb, "response": sProp("Answer to the customer"),
				"expectedFixDate": sProp("YYYY-MM-DD, optional"),
			}),
			run: endpoint(http.MethodPatch, "/v1/project-feedbacks/{feedbackId}/response", nil, 0, func(a args) (any, error) {
				body := map[string]any{"response": a.str("response")}
				d, err := taskDate(a.str("expectedFixDate"))
				if err != nil {
					return nil, err
				}
				if d != nil {
					body["expectedFixDate"] = d
				}
				return body, nil
			}),
		},
		{
			name:        "link_feedback_task",
			description: "Link a task to customer feedback (the task that handles it), or unlink it.",
			schema: obj([]string{"feedbackId", "taskId"}, map[string]any{
				"feedbackId": fb, "taskId": sProp("Task ID"), "remove": bProp("true to unlink"),
			}),
			run: func(c *caller, a args) (any, error) {
				method := http.MethodPut
				if b, _ := a["remove"].(bool); b {
					method = http.MethodDelete
				}
				return endpoint(method, "/v1/project-feedbacks/{feedbackId}/tasks/{taskId}", nil, 0, nil)(c, a)
			},
		},
		{
			name:        "list_task_feedbacks",
			description: "Customer feedback linked to a task.",
			schema:      obj([]string{"taskId"}, map[string]any{"taskId": sProp("Task ID")}),
			run:         endpoint(http.MethodGet, "/v1/tasks/{taskId}/feedbacks", nil, 0, nil),
		},
		{
			name:        "list_project_questions",
			description: "Questions the team asked the customer on the project's public board, with the customer's answers.",
			schema:      obj([]string{"workspaceId", "projectId"}, wp(nil)),
			run:         endpoint(http.MethodGet, projectPath+"/questions", nil, 0, nil),
		},
		{
			name:        "ask_project_question",
			description: "Ask the customer a question on the project's public board (they answer there).",
			schema:      obj([]string{"workspaceId", "projectId", "question"}, wp(map[string]any{"question": sProp("The question")})),
			run: endpoint(http.MethodPost, projectPath+"/questions", nil, 0, func(a args) (any, error) {
				return map[string]any{"question": a.str("question")}, nil
			}),
		},
		{
			name:        "update_project_question",
			description: "Reword a question, or delete it.",
			schema: obj([]string{"questionId"}, map[string]any{
				"questionId": sProp("Question ID"), "question": sProp("New wording"), "delete": bProp("true to delete the question"),
			}),
			run: func(c *caller, a args) (any, error) {
				if b, _ := a["delete"].(bool); b {
					return endpoint(http.MethodDelete, "/v1/project-questions/{questionId}", nil, 0, nil)(c, a)
				}
				if _, err := a.need("question"); err != nil {
					return nil, err
				}
				return endpoint(http.MethodPatch, "/v1/project-questions/{questionId}", nil, 0, func(a args) (any, error) {
					return map[string]any{"question": a.str("question")}, nil
				})(c, a)
			},
		},
	}
}

// reportTools covers end-of-day reports.
func reportTools() []tool {
	return []tool{
		{
			name: "list_daily_reports",
			description: "End-of-day reports (done / in progress / issues), newest first. Without memberId and " +
				"with mine=false, lists everyone's (needs a manager role).",
			schema: obj([]string{"workspaceId"}, withPage(map[string]any{
				"workspaceId": sProp("Workspace ID"), "from": sProp("YYYY-MM-DD"), "to": sProp("YYYY-MM-DD"),
				"mine": bProp("Only my own reports (default true)"), "memberId": sProp("Only this member (manager view)"),
			})),
			run: func(c *caller, a args) (any, error) {
				if mine, ok := a["mine"].(bool); (ok && !mine) || a.str("memberId") != "" {
					return endpoint(http.MethodGet, "/v1/workspaces/{workspaceId}/daily-reports", []string{"from", "to", "memberId"}, 30, nil)(c, a)
				}
				return endpoint(http.MethodGet, "/v1/workspaces/{workspaceId}/daily-reports/me", []string{"from", "to"}, 30, nil)(c, a)
			},
		},
		{
			name: "save_daily_report",
			description: "Write my end-of-day report for a date. Only the given parts change; the others keep " +
				"what was already saved for that day. Tip: build it from list_my_tasks and task history.",
			schema: obj([]string{"workspaceId", "date"}, map[string]any{
				"workspaceId": sProp("Workspace ID"), "date": sProp("YYYY-MM-DD"),
				"done": sProp("What was finished (markdown)"), "inProgress": sProp("What is still in progress"),
				"issues": sProp("Blockers / issues"),
			}),
			run: saveDailyReport,
		},
	}
}

func saveDailyReport(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	date, err := a.need("date")
	if err != nil {
		return nil, err
	}
	parts := []string{"done", "inProgress", "issues"}
	if !hasAny(a, parts) {
		return nil, errors.New("nothing to save: pass done, inProgress or issues")
	}
	path := "/v1/workspaces/" + p(ws) + "/daily-reports/me"
	body := map[string]any{"date": date, "done": "", "inProgress": "", "issues": ""}
	if raw, err := c.do(http.MethodGet, path, map[string][]string{"from": {date}, "to": {date}, "limit": {"1"}}, nil); err == nil {
		var page struct {
			Items []map[string]any `json:"items"`
		}
		if json.Unmarshal(raw, &page) == nil && len(page.Items) > 0 {
			args(page.Items[0]).fields(body, parts...)
		}
	}
	return c.do(http.MethodPut, path, nil, a.fields(body, parts...))
}
