package module

import (
	"context"
	"errors"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/internal/features/history"
	"github.com/alexcao194/kairso/pkg/pagination"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/alexcao194/kairso/pkg/ptr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	CreateModule(ctx context.Context, projectID string, req CreateModuleRequest) (*ModuleResponse, error)
	ListModules(ctx context.Context, projectAlias string, params pagination.StatsParams) (pagination.Response[ModuleResponse], error)
	GetModule(ctx context.Context, moduleID string) (*ModuleResponse, error)
	UpdateModule(ctx context.Context, moduleID, callerID string, req UpdateModuleRequest) (*ModuleResponse, error)
	ListModuleHistory(ctx context.Context, moduleID string, params pagination.Params) (pagination.Response[ModuleHistoryResponse], error)
	DeleteModule(ctx context.Context, moduleID string) error
	GetWorkspaceIDByProjectID(ctx context.Context, projectID string) (string, error)
	CheckModuleMembership(ctx context.Context, moduleID, callerID string) (bool, error)
	CheckProjectMembership(ctx context.Context, projectID, callerID string) (bool, error)

	ListSprints(ctx context.Context, moduleID string) ([]ModuleSprintResponse, error)
	GetSprint(ctx context.Context, sprintID string) (*ModuleSprintResponse, error)
	CreateSprint(ctx context.Context, moduleID string, sp ModuleSprintResponse) (*ModuleSprintResponse, error)
	UpdateSprint(ctx context.Context, sp ModuleSprintResponse) (*ModuleSprintResponse, error)
	DeleteSprint(ctx context.Context, sprintID string) error
	SyncModuleStartFromSprints(ctx context.Context, moduleID string) error
	InsertHistory(ctx context.Context, moduleID, callerID, field string, oldValue, newValue *string) error

	AddAttachment(ctx context.Context, moduleID, uploaderID string, req CreateModuleAttachmentRequest) (*ModuleAttachmentResponse, error)
	ListAttachments(ctx context.Context, moduleID string) ([]ModuleAttachmentResponse, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error
}

type repository struct {
	queries *database.Queries
	db      *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{queries: database.New(pool), db: pool}
}

func (r *repository) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	txRepo := &repository{queries: r.queries.WithTx(tx), db: r.db}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *repository) CreateModule(ctx context.Context, projectID string, req CreateModuleRequest) (*ModuleResponse, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateModule(ctx, database.CreateModuleParams{
		ProjectID: pid,
		Name:      req.Name,
		StartDate: pgutil.Timestamptz(req.StartDate),
		EndDate:   pgutil.Timestamptz(req.EndDate),
	})
	if err != nil {
		return nil, err
	}
	resp := moduleResponseFromDatabaseModule(row)
	return &resp, nil
}

func (r *repository) ListModules(ctx context.Context, projectAlias string, params pagination.StatsParams) (pagination.Response[ModuleResponse], error) {
	var empty pagination.Response[ModuleResponse]
	limit := params.LimitWithMore()
	offset := params.Offset()

	pid, err := pgutil.ParseUUID(projectAlias)
	if err != nil {
		return empty, err
	}
	fromTime, toTime, err := pgutil.MonthRange(params.Month)
	if err != nil {
		return empty, err
	}
	rows, err := r.queries.ListModulesByProjectAliasWithStats(ctx, database.ListModulesByProjectAliasWithStatsParams{
		ProjectID: pid,
		Limit:     int32(limit),
		Offset:    int32(offset),
		FromTime:  fromTime,
		ToTime:    toTime,
	})
	if err != nil {
		return empty, err
	}
	moduleIDs := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		moduleIDs = append(moduleIDs, row.ID)
	}
	sprintsByModule, err := r.listSprintsByModuleIDs(ctx, moduleIDs)
	if err != nil {
		return empty, err
	}
	items := make([]ModuleResponse, 0, len(rows))
	for _, row := range rows {
		item := moduleResponseFromDatabaseModuleWithStats(row)
		item.Sprints = sprintsByModule[item.ID]
		items = append(items, item)
	}

	hasMore := pagination.Process(&items, params.Limit)
	return pagination.NewResponse(items, hasMore, params.Page, params.Limit), nil
}

func (r *repository) listSprintsByModuleIDs(ctx context.Context, moduleIDs []pgtype.UUID) (map[string][]ModuleSprintResponse, error) {
	result := make(map[string][]ModuleSprintResponse, len(moduleIDs))
	for _, id := range moduleIDs {
		result[pgutil.UUIDToString(id)] = []ModuleSprintResponse{}
	}
	if len(moduleIDs) == 0 {
		return result, nil
	}
	rows, err := r.queries.ListModuleSprintsByModuleIDs(ctx, moduleIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := pgutil.UUIDToString(row.ModuleID)
		result[key] = append(result[key], sprintResponseFromDatabase(row))
	}
	return result, nil
}

func (r *repository) GetModule(ctx context.Context, moduleID string) (*ModuleResponse, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetModuleByID(ctx, mid)
	if err != nil {
		return nil, err
	}
	resp := moduleResponseFromDatabaseModule(row)
	sprints, err := r.listSprintsByModuleIDs(ctx, []pgtype.UUID{mid})
	if err != nil {
		return nil, err
	}
	resp.Sprints = sprints[resp.ID]
	return &resp, nil
}

func (r *repository) UpdateModule(ctx context.Context, moduleID, callerID string, req UpdateModuleRequest) (*ModuleResponse, error) {
	callerUID, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return nil, err
	}
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	txQueries := database.New(tx)
	defer tx.Rollback(ctx)
	beforeRow, err := txQueries.GetModuleByIDForUpdate(ctx, mid)
	if err != nil {
		return nil, err
	}
	before := moduleResponseFromDatabaseModule(beforeRow)

	statusPresent := req.Status != nil
	var status ModuleStatus
	if statusPresent {
		status = *req.Status
	}
	releasedPresent := false
	var releasedAt pgtype.Timestamptz
	if statusPresent && status != before.Status {
		releasedPresent = true
		if status == ModuleStatusReleased {
			releasedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}
	}

	afterRow, err := txQueries.UpdateModuleByID(ctx, database.UpdateModuleByIDParams{
		NamePresent:        req.namePresent(),
		Name:               ptr.Value(req.Name),
		StartDatePresent:   req.startDatePresent(),
		StartDate:          pgutil.Timestamptz(req.StartDate),
		EndDatePresent:     req.endDatePresent(),
		EndDate:            pgutil.Timestamptz(req.EndDate),
		ScrumStatusPresent: statusPresent,
		ScrumStatus:        string(status),
		ReleasedAtPresent:  releasedPresent,
		ReleasedAt:         releasedAt,
		ID:                 mid,
	})
	if err != nil {
		return nil, err
	}
	after := moduleResponseFromDatabaseModule(afterRow)
	afterSprints, err := (&repository{queries: txQueries, db: r.db}).listSprintsByModuleIDs(ctx, []pgtype.UUID{mid})
	if err != nil {
		return nil, err
	}
	after.Sprints = afterSprints[after.ID]
	for _, change := range []struct {
		present bool
		field   string
		old     *string
		new     *string
	}{
		{req.namePresent(), "name", stringPtr(before.Name), stringPtr(after.Name)},
		{req.startDatePresent(), "startDate", ptr.RFC3339(before.StartDate), ptr.RFC3339(after.StartDate)},
		{req.endDatePresent(), "endDate", ptr.RFC3339(before.EndDate), ptr.RFC3339(after.EndDate)},
		{statusPresent, "status", stringPtr(string(before.Status)), stringPtr(string(after.Status))},
	} {
		if change.present {
			if err := history.Insert(ctx, txQueries, "module", after.ID, callerUID, change.field, change.old, change.new); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &after, nil
}

func (r *repository) ListModuleHistory(ctx context.Context, moduleID string, params pagination.Params) (pagination.Response[ModuleHistoryResponse], error) {
	var empty pagination.Response[ModuleHistoryResponse]
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return empty, err
	}
	limit := params.LimitWithMore()
	offset := params.Offset()
	rows, err := r.queries.ListModuleHistoriesByID(ctx, database.ListModuleHistoriesByIDParams{
		EntityID: mid,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return empty, err
	}
	items := make([]ModuleHistoryResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, moduleHistoryResponseFromDatabaseHistory(row))
	}
	hasMore := pagination.Process(&items, params.Limit)
	return pagination.NewResponse(items, hasMore, params.Page, params.Limit), nil
}

func (r *repository) DeleteModule(ctx context.Context, moduleID string) error {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return err
	}
	return r.queries.SoftDeleteModule(ctx, mid)
}

func (r *repository) AddAttachment(ctx context.Context, moduleID, uploaderID string, req CreateModuleAttachmentRequest) (*ModuleAttachmentResponse, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(uploaderID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateEntityAttachment(ctx, database.CreateEntityAttachmentParams{
		EntityType:  "module",
		EntityID:    mid,
		UploaderID:  uid,
		FileName:    req.FileName,
		FileUrl:     req.FileURL,
		ContentType: pgutil.TextOrNull(req.ContentType),
		SizeBytes:   pgutil.Int8OrNull(req.SizeBytes),
	})
	if err != nil {
		return nil, err
	}
	return &ModuleAttachmentResponse{
		ID:          pgutil.UUIDToString(row.ID),
		CreatedAt:   row.CreatedAt.Time,
		ModuleID:    pgutil.UUIDToString(row.EntityID),
		UploaderID:  pgutil.UUIDToString(row.UploaderID),
		FileName:    row.FileName,
		FileURL:     row.FileUrl,
		ContentType: pgutil.Text(row.ContentType),
		SizeBytes:   pgutil.Int8Value(row.SizeBytes),
	}, nil
}

func (r *repository) ListAttachments(ctx context.Context, moduleID string) ([]ModuleAttachmentResponse, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListEntityAttachmentsByEntity(ctx, database.ListEntityAttachmentsByEntityParams{
		EntityType: "module",
		EntityID:   mid,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ModuleAttachmentResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, ModuleAttachmentResponse{
			ID:            pgutil.UUIDToString(row.ID),
			CreatedAt:     row.CreatedAt.Time,
			ModuleID:      pgutil.UUIDToString(row.EntityID),
			UploaderID:    pgutil.UUIDToString(row.UploaderID),
			UploaderAlias: row.UploaderAlias,
			UploaderName:  row.UploaderName,
			FileName:      row.FileName,
			FileURL:       row.FileUrl,
			ContentType:   pgutil.Text(row.ContentType),
			SizeBytes:     pgutil.Int8Value(row.SizeBytes),
		})
	}
	return items, nil
}

func (r *repository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	aid, err := pgutil.ParseUUID(attachmentID)
	if err != nil {
		return err
	}
	return r.queries.DeleteEntityAttachment(ctx, database.DeleteEntityAttachmentParams{
		ID:         aid,
		EntityType: "module",
	})
}

func (r *repository) GetWorkspaceIDByProjectID(ctx context.Context, projectID string) (string, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return "", err
	}
	wid, err := r.queries.GetWorkspaceIDByProjectID(ctx, pid)
	if err != nil {
		return "", err
	}
	return pgutil.UUIDToString(wid), nil
}

func stringPtr(value string) *string {
	return &value
}

func (r *repository) ListSprints(ctx context.Context, moduleID string) ([]ModuleSprintResponse, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	m, err := r.listSprintsByModuleIDs(ctx, []pgtype.UUID{mid})
	if err != nil {
		return nil, err
	}
	return m[moduleID], nil
}

func (r *repository) GetSprint(ctx context.Context, sprintID string) (*ModuleSprintResponse, error) {
	sid, err := pgutil.ParseUUID(sprintID)
	if err != nil {
		return nil, ErrSprintNotFound
	}
	row, err := r.queries.GetModuleSprint(ctx, sid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSprintNotFound
		}
		return nil, err
	}
	resp := sprintResponseFromDatabase(row)
	return &resp, nil
}

func parseDate(v string) (pgtype.Date, error) {
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func (r *repository) CreateSprint(ctx context.Context, moduleID string, sp ModuleSprintResponse) (*ModuleSprintResponse, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return nil, err
	}
	start, err := parseDate(sp.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(sp.EndDate)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateModuleSprint(ctx, database.CreateModuleSprintParams{
		ModuleID: mid, Name: sp.Name, Goal: sp.Goal, StartDate: start, EndDate: end,
		Status: database.ModuleSprintStatus(sp.Status), ReviewNote: sp.ReviewNote, RetroNote: sp.RetroNote,
	})
	if err != nil {
		return nil, err
	}
	resp := sprintResponseFromDatabase(row)
	return &resp, nil
}

func (r *repository) UpdateSprint(ctx context.Context, sp ModuleSprintResponse) (*ModuleSprintResponse, error) {
	sid, err := pgutil.ParseUUID(sp.ID)
	if err != nil {
		return nil, ErrSprintNotFound
	}
	start, err := parseDate(sp.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(sp.EndDate)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateModuleSprint(ctx, database.UpdateModuleSprintParams{
		ID: sid, Name: sp.Name, Goal: sp.Goal, StartDate: start, EndDate: end,
		Status: database.ModuleSprintStatus(sp.Status), ReviewNote: sp.ReviewNote, RetroNote: sp.RetroNote,
	})
	if err != nil {
		return nil, err
	}
	resp := sprintResponseFromDatabase(row)
	return &resp, nil
}

func (r *repository) DeleteSprint(ctx context.Context, sprintID string) error {
	sid, err := pgutil.ParseUUID(sprintID)
	if err != nil {
		return ErrSprintNotFound
	}
	return r.queries.DeleteModuleSprint(ctx, sid)
}

func (r *repository) SyncModuleStartFromSprints(ctx context.Context, moduleID string) error {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return err
	}
	return r.queries.SyncModuleStartFromSprints(ctx, mid)
}

func (r *repository) InsertHistory(ctx context.Context, moduleID, callerID, field string, oldValue, newValue *string) error {
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return err
	}
	return history.Insert(ctx, r.queries, "module", moduleID, uid, field, oldValue, newValue)
}

func (r *repository) CheckModuleMembership(ctx context.Context, moduleID, callerID string) (bool, error) {
	mid, err := pgutil.ParseUUID(moduleID)
	if err != nil {
		return false, err
	}
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return false, err
	}
	return r.queries.CheckModuleMembership(ctx, database.CheckModuleMembershipParams{ModuleID: mid, UserID: uid})
}

func (r *repository) CheckProjectMembership(ctx context.Context, projectID, callerID string) (bool, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return false, err
	}
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return false, err
	}
	return r.queries.CheckProjectMembership(ctx, database.CheckProjectMembershipParams{ProjectID: pid, UserID: uid})
}
