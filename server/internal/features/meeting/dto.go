package meeting

import (
	"encoding/json"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
)

// MeetingSection groups the line items of a minutes document.
type MeetingSection string

const (
	// SectionDeployed: work already done ("đã triển khai"), possibly still
	// waiting to be rolled out.
	SectionDeployed MeetingSection = "deployed"
	// SectionPlanned: work agreed to be done next ("cần triển khai").
	SectionPlanned MeetingSection = "planned"
	// SectionBacklog: noted but not scheduled yet.
	SectionBacklog MeetingSection = "backlog"
)

func (s MeetingSection) Valid() bool {
	switch s {
	case SectionDeployed, SectionPlanned, SectionBacklog:
		return true
	}
	return false
}

type MeetingItemStatus string

const (
	ItemStatusOpen MeetingItemStatus = "open"
	ItemStatusDone MeetingItemStatus = "done"
)

type ParticipantKind string

const (
	// ParticipantUser is a workspace member (UserID set).
	ParticipantUser ParticipantKind = "user"
	// ParticipantGroup is a free-typed name: a department, company or an
	// outside person.
	ParticipantGroup ParticipantKind = "group"
)

// MeetingParticipant is a person or department attending / recording the
// meeting. Name is snapshotted so the minutes print the same even after the
// member is renamed or leaves.
type MeetingParticipant struct {
	Kind   ParticipantKind `json:"kind" enums:"user,group"`
	UserID string          `json:"userId,omitempty"`
	Name   string          `json:"name"`
}

// MeetingTaskRef links a row to a task created (or picked) for it; the
// owner and due date of a row come from its tasks.
type MeetingTaskRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

// MeetingItem is one row of a minutes table.
type MeetingItem struct {
	// ID is a client-generated stable id; the server fills one in when empty.
	ID      string         `json:"id"`
	Section MeetingSection `json:"section" enums:"deployed,planned,backlog"`
	// Module is the sub-system the row is about ("phân hệ"), free text.
	Module   string `json:"module"`
	Content  string `json:"content"`
	Priority bool   `json:"priority"`
	// Status is used by deployed rows: open = waiting to be rolled out.
	Status MeetingItemStatus `json:"status" enums:"open,done"`
	Note   string            `json:"note"`
	Tasks  []MeetingTaskRef  `json:"tasks"`
}

type CreateMeetingMinutesRequest struct {
	// ProjectID scopes the minutes to a project; empty = workspace level.
	ProjectID    string               `json:"projectId,omitempty"`
	Title        string               `json:"title"`
	MeetingDate  string               `json:"meetingDate" swaggertype:"string" format:"date"`
	Location     string               `json:"location"`
	Attendees    []MeetingParticipant `json:"attendees"`
	Recorders    []MeetingParticipant `json:"recorders"`
	Summary      string               `json:"summary"`
	Items        []MeetingItem        `json:"items"`
	SectionNotes map[string]string    `json:"sectionNotes"`
}

// UpdateMeetingMinutesRequest only changes the fields it carries; `items`
// and `sectionNotes`, when sent, replace the whole list/map.
type UpdateMeetingMinutesRequest struct {
	Title        *string               `json:"title,omitempty"`
	MeetingDate  *string               `json:"meetingDate,omitempty" swaggertype:"string" format:"date"`
	Location     *string               `json:"location,omitempty"`
	Attendees    *[]MeetingParticipant `json:"attendees,omitempty"`
	Recorders    *[]MeetingParticipant `json:"recorders,omitempty"`
	Summary      *string               `json:"summary,omitempty"`
	Items        *[]MeetingItem        `json:"items,omitempty"`
	SectionNotes *map[string]string    `json:"sectionNotes,omitempty"`
}

type ListMeetingMinutesFilter struct {
	ProjectID string
	Query     string
}

type MeetingCreator struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Alias  string `json:"alias"`
	Avatar string `json:"avatar"`
}

type MeetingMinutesResponse struct {
	ID           string               `json:"id"`
	WorkspaceID  string               `json:"workspaceId"`
	ProjectID    string               `json:"projectId,omitempty"`
	Title        string               `json:"title"`
	MeetingDate  string               `json:"meetingDate" swaggertype:"string" format:"date"`
	Location     string               `json:"location"`
	Attendees    []MeetingParticipant `json:"attendees"`
	Recorders    []MeetingParticipant `json:"recorders"`
	Summary      string               `json:"summary"`
	Items        []MeetingItem        `json:"items"`
	SectionNotes map[string]string    `json:"sectionNotes"`
	CreatedBy    MeetingCreator       `json:"createdBy"`
	CreatedAt    time.Time            `json:"createdAt" swaggertype:"string" format:"date-time"`
	UpdatedAt    time.Time            `json:"updatedAt" swaggertype:"string" format:"date-time"`
}

// ---- mapping ----

func dateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(time.DateOnly)
}

func meetingResponseFromRow(row database.GetMeetingMinutesRow) MeetingMinutesResponse {
	items := []MeetingItem{}
	_ = json.Unmarshal(row.Items, &items)
	for i := range items {
		if items[i].Tasks == nil {
			items[i].Tasks = []MeetingTaskRef{}
		}
	}
	notes := map[string]string{}
	_ = json.Unmarshal(row.SectionNotes, &notes)
	attendees := []MeetingParticipant{}
	_ = json.Unmarshal(row.Attendees, &attendees)
	recorders := []MeetingParticipant{}
	_ = json.Unmarshal(row.Recorders, &recorders)
	return MeetingMinutesResponse{
		ID:           pgutil.UUIDToString(row.ID),
		WorkspaceID:  pgutil.UUIDToString(row.WorkspaceID),
		ProjectID:    pgutil.UUIDToString(row.ProjectID),
		Title:        row.Title,
		MeetingDate:  dateString(row.MeetingDate),
		Location:     row.Location,
		Attendees:    attendees,
		Recorders:    recorders,
		Summary:      row.Summary,
		Items:        items,
		SectionNotes: notes,
		CreatedBy: MeetingCreator{
			ID:     pgutil.UUIDToString(row.CreatedByID),
			Name:   row.CreatorName,
			Alias:  row.CreatorAlias,
			Avatar: row.CreatorAvatar,
		},
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// The sqlc row types for create/list/update share GetMeetingMinutesRow's
// exact shape, so they convert directly.
func meetingResponseFromCreateRow(row database.CreateMeetingMinutesRow) MeetingMinutesResponse {
	return meetingResponseFromRow(database.GetMeetingMinutesRow(row))
}

func meetingResponseFromListRow(row database.ListMeetingMinutesRow) MeetingMinutesResponse {
	return meetingResponseFromRow(database.GetMeetingMinutesRow(row))
}

func meetingResponseFromUpdateRow(row database.UpdateMeetingMinutesRow) MeetingMinutesResponse {
	return meetingResponseFromRow(database.GetMeetingMinutesRow(row))
}

func parseDate(value string) (pgtype.Date, error) {
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func textPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}
