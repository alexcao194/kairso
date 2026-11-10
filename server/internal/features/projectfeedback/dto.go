package projectfeedback

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pagination"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListFeedbacksFilter narrows a feedback board/list — every field optional.
type ListFeedbacksFilter struct {
	// Priorities filters to any of these 1..4 values; empty means "any".
	Priorities []int32
	// HasResponse filters to responded-only (true) or unresponded-only
	// (false); nil means "any".
	HasResponse *bool
	// Statuses filters to any of these triage statuses; empty means "any".
	Statuses []FeedbackStatus
	// FeedbackTypes filters to any of these feedback types; empty means "any".
	FeedbackTypes []FeedbackType
	// Query free-text searches submitter name/company, feature name, and content.
	Query string
	// SortRules orders the result — see sort.go. Empty means
	// defaultFeedbackSortRules().
	SortRules []FeedbackSortRule
}

// FeedbackListStats aggregates over every feedback matching the current
// filter (not just the current page) — mirrors task.TaskListStats.
type FeedbackListStats struct {
	TotalCount int `json:"totalCount"`
}

// ProjectFeedbackListResponse is a paginated feedback list plus
// FeedbackListStats over the full filtered set.
type ProjectFeedbackListResponse struct {
	pagination.Response[ProjectFeedbackResponse]
	Stats FeedbackListStats `json:"stats"`
}

// UserSummary is the minimal author info shown for who left a Response.
type UserSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Alias  string `json:"alias"`
	Avatar string `json:"avatar,omitempty"`
}

// LinkedTaskSummary is deliberately minimal — public visitors only ever see
// a linked task's code and status, never its title/assignees/etc.
type LinkedTaskSummary struct {
	Code   string `json:"code"`
	Status string `json:"status"`
}

// FeedbackStatus is the triage state a project member sets on a feedback
// item — independent of whether a Response has been written or a task
// linked (those stay optional signals; status is the explicit one).
type FeedbackStatus string

const (
	FeedbackStatusPending      FeedbackStatus = "pending"
	FeedbackStatusDiscussing   FeedbackStatus = "discussing"
	FeedbackStatusAcknowledged FeedbackStatus = "acknowledged"
	FeedbackStatusInProgress   FeedbackStatus = "in_progress"
	FeedbackStatusResolved     FeedbackStatus = "resolved"
	FeedbackStatusDeployed     FeedbackStatus = "deployed"
	FeedbackStatusCanceled     FeedbackStatus = "canceled"
)

func (s FeedbackStatus) Valid() bool {
	switch s {
	case FeedbackStatusPending, FeedbackStatusDiscussing,
		FeedbackStatusAcknowledged, FeedbackStatusInProgress,
		FeedbackStatusResolved, FeedbackStatusDeployed,
		FeedbackStatusCanceled:
		return true
	default:
		return false
	}
}

// FeedbackType is the category of a feedback item, set by the submitter.
type FeedbackType string

const (
	FeedbackTypeFeature     FeedbackType = "feature"
	FeedbackTypeImprovement FeedbackType = "improvement"
	FeedbackTypeBug         FeedbackType = "bug"
)

func (t FeedbackType) Valid() bool {
	switch t {
	case FeedbackTypeFeature, FeedbackTypeImprovement, FeedbackTypeBug:
		return true
	default:
		return false
	}
}

type CreateProjectFeedbackRequest struct {
	SubmitterName    string   `json:"submitterName"`
	SubmitterCompany string   `json:"submitterCompany"`
	FeatureName      string   `json:"featureName"`
	Content          string   `json:"content"`
	Priority         *int     `json:"priority"`
	FeedbackType     string   `json:"feedbackType"`
	Images           []string `json:"images,omitempty"`
}

// UpdateProjectFeedbackRequest lets a project member freely edit a
// feedback's own fields (not the member-only Response) — any member, not
// just admin/holder.
type UpdateProjectFeedbackRequest struct {
	SubmitterName    string   `json:"submitterName"`
	SubmitterCompany string   `json:"submitterCompany"`
	FeatureName      string   `json:"featureName"`
	Content          string   `json:"content"`
	Priority         *int     `json:"priority"`
	FeedbackType     string   `json:"feedbackType"`
	Images           []string `json:"images,omitempty"`
}

// UpdateFeedbackStatusRequest sets the triage status — a lightweight
// sibling of UpdateProjectFeedbackRequest so an inline table dropdown can
// change just this one field.
type UpdateFeedbackStatusRequest struct {
	Status FeedbackStatus `json:"status"`
}

// UpdateFeedbackResponseRequest sets (or clears, with an empty string) the
// "Response" shown to public visitors — member-only.
type UpdateFeedbackResponseRequest struct {
	Response        string     `json:"response"`
	ExpectedFixDate *time.Time `json:"expectedFixDate,omitempty" swaggertype:"string" format:"date-time"`
}

// FeedbackSummaryResponse is a lightweight count used to badge the
// project's "Feedback" feature entry — how many items are still in the
// "pending" triage status, without loading the full list.
type FeedbackSummaryResponse struct {
	PendingCount int `json:"pendingCount"`
}

type ProjectFeedbackResponse struct {
	ID               string              `json:"id"`
	CreatedAt        time.Time           `json:"createdAt" swaggertype:"string" format:"date-time"`
	ProjectID        string              `json:"projectId"`
	SubmitterName    string              `json:"submitterName"`
	SubmitterCompany string              `json:"submitterCompany"`
	FeatureName      string              `json:"featureName"`
	Content          string              `json:"content"`
	Priority         int                 `json:"priority"`
	Status           FeedbackStatus      `json:"status"`
	FeedbackType     FeedbackType        `json:"feedbackType"`
	Images           []string            `json:"images"`
	Response         string              `json:"response,omitempty"`
	ExpectedFixDate  *time.Time          `json:"expectedFixDate,omitempty" swaggertype:"string" format:"date-time"`
	RespondedAt      *time.Time          `json:"respondedAt,omitempty" swaggertype:"string" format:"date-time"`
	RespondedBy      *UserSummary        `json:"respondedBy,omitempty"`
	Tasks            []LinkedTaskSummary `json:"tasks"`
	// SequenceNo is only the raw input to Code below — no client needs the
	// bare number once Code is resolved, so it stays off the wire.
	SequenceNo int `json:"-"`
	// Code is a human-readable, copyable identifier derived from the
	// project name and SequenceNo, e.g. "#KAIRSO-F7" — resolved in hydrate
	// rather than stored, mirroring task.Code/task.FeedbackCode.
	Code string `json:"code"`
}

// FeedbackSettingsResponse is the member-facing view of a project's public
// feedback link — Token is omitted (zero value) until the link has been
// enabled at least once.
type FeedbackSettingsResponse struct {
	Enabled            bool   `json:"enabled"`
	Token              string `json:"token,omitempty"`
	ShowModuleTimeline bool   `json:"showModuleTimeline"`
}

type UpdateFeedbackSettingsRequest struct {
	ShowModuleTimeline *bool `json:"showModuleTimeline,omitempty"`
}

// PublicProjectResponse is what an anonymous visitor sees before anything
// else loads — just enough to render the page header, never internal IDs.
type PublicProjectResponse struct {
	ProjectName        string `json:"projectName"`
	Enabled            bool   `json:"enabled"`
	ShowModuleTimeline bool   `json:"showModuleTimeline"`
}

// PublicModuleResponse is a lightweight, public-safe view of a project module
// — no internal IDs, just the Scrum timeline the public board draws:
// status, sprints and the target/actual release date.
type PublicModuleResponse struct {
	Name      string     `json:"name"`
	Status    string     `json:"status" enums:"backlog,in_progress,done,released"`
	StartDate *time.Time `json:"startDate,omitempty" swaggertype:"string" format:"date-time"`
	// EndDate is the target release date.
	EndDate    *time.Time             `json:"endDate,omitempty" swaggertype:"string" format:"date-time"`
	ReleasedAt *time.Time             `json:"releasedAt,omitempty" swaggertype:"string" format:"date-time"`
	Sprints    []PublicSprintResponse `json:"sprints"`
}

// PublicSprintResponse deliberately leaves out the Sprint Goal and the
// Review/Retrospective notes — those are internal to the team.
type PublicSprintResponse struct {
	Name      string `json:"name"`
	StartDate string `json:"startDate" swaggertype:"string" format:"date"`
	EndDate   string `json:"endDate" swaggertype:"string" format:"date"`
	Status    string `json:"status" enums:"planned,active,completed"`
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func publicModulesFromDatabase(modules []database.ListPublicModulesByProjectIDRow, sprints []database.ListPublicSprintsByModuleIDsRow) []PublicModuleResponse {
	byModule := make(map[[16]byte][]PublicSprintResponse, len(modules))
	for _, sp := range sprints {
		byModule[sp.ModuleID.Bytes] = append(byModule[sp.ModuleID.Bytes], PublicSprintResponse{
			Name:      sp.Name,
			StartDate: sp.StartDate.Time.Format(time.DateOnly),
			EndDate:   sp.EndDate.Time.Format(time.DateOnly),
			Status:    string(sp.Status),
		})
	}
	out := make([]PublicModuleResponse, 0, len(modules))
	for _, m := range modules {
		list := byModule[m.ID.Bytes]
		if list == nil {
			list = []PublicSprintResponse{}
		}
		out = append(out, PublicModuleResponse{
			Name:       m.Name,
			Status:     string(m.ScrumStatus),
			StartDate:  timePtr(m.StartDate),
			EndDate:    timePtr(m.EndDate),
			ReleasedAt: timePtr(m.ReleasedAt),
			Sprints:    list,
		})
	}
	return out
}

func normalizeCreateFeedbackRequest(req *CreateProjectFeedbackRequest) error {
	req.SubmitterName = strings.TrimSpace(req.SubmitterName)
	req.SubmitterCompany = strings.TrimSpace(req.SubmitterCompany)
	req.FeatureName = strings.TrimSpace(req.FeatureName)
	req.Content = strings.TrimSpace(req.Content)
	if req.SubmitterName == "" {
		return fmt.Errorf("submitterName là bắt buộc")
	}
	if req.SubmitterCompany == "" {
		return fmt.Errorf("submitterCompany là bắt buộc")
	}
	if req.FeatureName == "" {
		return fmt.Errorf("featureName là bắt buộc")
	}
	if req.Content == "" {
		return fmt.Errorf("Nội dung là bắt buộc")
	}
	if req.Priority == nil {
		return fmt.Errorf("priority là bắt buộc")
	}
	if *req.Priority < 1 || *req.Priority > 4 {
		return fmt.Errorf("priority phải là 1 (thấp), 2 (vừa), 3 (cao) hoặc 4 (gấp)")
	}
	if req.FeedbackType == "" {
		return fmt.Errorf("feedbackType là bắt buộc")
	}
	if !FeedbackType(req.FeedbackType).Valid() {
		return fmt.Errorf("feedbackType phải là feature, improvement hoặc bug")
	}
	return nil
}

func normalizeUpdateFeedbackRequest(req *UpdateProjectFeedbackRequest) error {
	req.SubmitterName = strings.TrimSpace(req.SubmitterName)
	req.SubmitterCompany = strings.TrimSpace(req.SubmitterCompany)
	req.FeatureName = strings.TrimSpace(req.FeatureName)
	req.Content = strings.TrimSpace(req.Content)
	if req.SubmitterName == "" {
		return fmt.Errorf("submitterName là bắt buộc")
	}
	if req.SubmitterCompany == "" {
		return fmt.Errorf("submitterCompany là bắt buộc")
	}
	if req.FeatureName == "" {
		return fmt.Errorf("featureName là bắt buộc")
	}
	if req.Content == "" {
		return fmt.Errorf("Nội dung là bắt buộc")
	}
	if req.Priority == nil {
		return fmt.Errorf("priority là bắt buộc")
	}
	if *req.Priority < 1 || *req.Priority > 4 {
		return fmt.Errorf("priority phải là 1 (thấp), 2 (vừa), 3 (cao) hoặc 4 (gấp)")
	}
	if req.FeedbackType == "" {
		return fmt.Errorf("feedbackType là bắt buộc")
	}
	if !FeedbackType(req.FeedbackType).Valid() {
		return fmt.Errorf("feedbackType phải là feature, improvement hoặc bug")
	}
	var cleanImages []string
	for _, img := range req.Images {
		if u := strings.TrimSpace(img); u != "" {
			cleanImages = append(cleanImages, u)
		}
	}
	req.Images = cleanImages
	return nil
}

func feedbackResponseFromRow(row feedbackRow) ProjectFeedbackResponse {
	return ProjectFeedbackResponse{
		ID:               pgutil.UUIDToString(row.ID),
		CreatedAt:        row.CreatedAt,
		ProjectID:        pgutil.UUIDToString(row.ProjectID),
		SubmitterName:    row.SubmitterName,
		SubmitterCompany: row.SubmitterCompany,
		FeatureName:      row.FeatureName,
		Content:          row.Content,
		Priority:         int(row.Priority),
		Status:           FeedbackStatus(row.Status),
		FeedbackType:     FeedbackType(row.FeedbackType),
		Response:         textString(row.Response),
		ExpectedFixDate:  timestamptzPtr(row.ExpectedFixDate),
		RespondedAt:      timestamptzPtr(row.RespondedAt),
		Images:           []string{},
		Tasks:            []LinkedTaskSummary{},
		SequenceNo:       int(row.SequenceNo),
	}
}

// feedbackRow is the flat set of columns shared by every project_feedbacks
// row-returning query (create/list/get/update) — sqlc gives each its own
// named Row type with identical shapes, so this lets the mapper above be
// written once.
type feedbackRow struct {
	ID               pgtype.UUID
	CreatedAt        time.Time
	ProjectID        pgtype.UUID
	SubmitterName    string
	SubmitterCompany string
	FeatureName      string
	Content          string
	Priority         int32
	Status           string
	FeedbackType     string
	Response         pgtype.Text
	ExpectedFixDate  pgtype.Timestamptz
	RespondedAt      pgtype.Timestamptz
	RespondedByID    pgtype.UUID
	SequenceNo       int32
}

func feedbackRowFromCreate(row database.CreateProjectFeedbackRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func feedbackRowFromListByTask(row database.ListFeedbacksByTaskIDRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func feedbackRowFromGet(row database.GetProjectFeedbackByIDRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func feedbackRowFromUpdate(row database.UpdateProjectFeedbackResponseRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func feedbackRowFromUpdateFeedback(row database.UpdateProjectFeedbackRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func feedbackRowFromUpdateStatus(row database.UpdateProjectFeedbackStatusRow) feedbackRow {
	return feedbackRow{
		ID: row.ID, CreatedAt: row.CreatedAt.Time, ProjectID: row.ProjectID,
		SubmitterName: row.SubmitterName, SubmitterCompany: row.SubmitterCompany,
		FeatureName: row.FeatureName, Content: row.Content, Priority: row.Priority, Status: row.Status,
		FeedbackType: row.FeedbackType,
		Response:     row.Response, ExpectedFixDate: row.ExpectedFixDate, RespondedAt: row.RespondedAt, RespondedByID: row.RespondedByID,
		SequenceNo: row.SequenceNo,
	}
}

func textString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
