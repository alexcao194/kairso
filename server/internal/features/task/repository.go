package task

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pagination"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	CreateTask(ctx context.Context, projectAlias, userID string, req CreateTaskRequest) (*TaskResponse, error)
	ListTasks(ctx context.Context, projectAlias, viewerID string, filter ListTasksParams, params pagination.Params) (TaskListResponse, error)
	GetTask(ctx context.Context, taskID string) (*TaskResponse, error)
	UpdateTask(ctx context.Context, taskID, callerID string, req UpdateTaskRequest) (*TaskResponse, error)
	ListTaskHistory(ctx context.Context, taskID string, params pagination.Params) (pagination.Response[TaskHistoryResponse], error)
	DeleteTask(ctx context.Context, taskID string) error
	// CheckTaskMembership reports whether callerID holds any role on the
	// task's project/workspace — the baseline gate for id-based task ops.
	CheckTaskMembership(ctx context.Context, taskID, callerID string) (bool, error)
	// CheckProjectMembership reports whether callerID holds any role on the
	// project/workspace — the gate for project-scoped task ops (list/create).
	CheckProjectMembership(ctx context.Context, projectID, callerID string) (bool, error)
	// CheckLabelMembership gates label-by-id ops to members of the label's project.
	CheckLabelMembership(ctx context.Context, labelID, callerID string) (bool, error)

	AddAttachment(ctx context.Context, taskID, uploaderID string, req CreateTaskAttachmentRequest) (*TaskAttachmentResponse, error)
	ListAttachments(ctx context.Context, taskID string) ([]TaskAttachmentResponse, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error

	CreateTaskComment(ctx context.Context, taskID, authorID, content string, attachments []CreateTaskCommentAttachment) (*TaskCommentResponse, error)
	ListTaskComments(ctx context.Context, taskID string) ([]TaskCommentResponse, error)
	GetTaskComment(ctx context.Context, commentID string) (*TaskCommentResponse, error)
	// UpdateTaskComment edits text only — see UpdateTaskCommentRequest.
	UpdateTaskComment(ctx context.Context, commentID, content string) (*TaskCommentResponse, error)
	// DeleteTaskComment also removes any entity_attachments rows the
	// comment owns (no FK cascade — entity_attachments is polymorphic).
	DeleteTaskComment(ctx context.Context, commentID string) error
	// ListTaskCommentAuthorIDs is the distinct set of everyone who has ever
	// commented on the task — used together with ListTaskWorkerUserIDs to
	// decide who a new comment's notification fans out to.
	ListTaskCommentAuthorIDs(ctx context.Context, taskID string) ([]string, error)
	MarkTaskCommentsRead(ctx context.Context, taskID, userID string) error
	CountUnreadTaskComments(ctx context.Context, taskID, viewerID string) (int, error)

	CreateTaskLabel(ctx context.Context, projectAlias, userID string, req CreateTaskLabelRequest) (*TaskLabelResponse, error)
	ListTaskLabels(ctx context.Context, projectAlias string, params pagination.Params) (pagination.Response[TaskLabelResponse], error)
	ListTaskLabelsByProjectIDs(ctx context.Context, projectIDs []string) ([]TaskLabelResponse, error)
	UpdateTaskLabel(ctx context.Context, labelID string, req UpdateTaskLabelRequest) (*TaskLabelResponse, error)
	// DeleteTaskLabel returns the parent projectID so callers can broadcast a
	// realtime event without an extra round trip.
	DeleteTaskLabel(ctx context.Context, labelID string) (string, error)
	AttachTaskLabel(ctx context.Context, taskID, labelID string) error
	DetachTaskLabel(ctx context.Context, taskID, labelID string) error

	GetUserIDByAlias(ctx context.Context, userAlias string) (string, error)
	GetUserAliasByID(ctx context.Context, userID string) (string, error)
	AssignTaskWorker(ctx context.Context, taskID, userID string, weight int) (*TaskWorkerResponse, error)
	RemoveTaskWorker(ctx context.Context, taskID, userID string) error

	AssignTaskFaultHolder(ctx context.Context, taskID, userID string, weight int) (*TaskFaultHolderResponse, error)
	RemoveTaskFaultHolder(ctx context.Context, taskID, userID string) error

	ListMyTasks(ctx context.Context, workspaceID, userID string, filter ListMyTasksParams, params pagination.Params) (TaskListResponse, error)
	CountMyActiveTasks(ctx context.Context, workspaceID, userID string) (int64, error)
	// ListThreads/CountThreads back the "Threads" nav destination — tasks
	// with unread comment activity for the caller, see db/queries/thread.sql.
	ListThreads(ctx context.Context, workspaceID, userID string, params pagination.Params) (pagination.Response[ThreadResponse], error)
	CountThreads(ctx context.Context, workspaceID, userID string) (int64, error)
	GetWorkspaceIDByProjectID(ctx context.Context, projectID string) (string, error)
	InsertEntityHistory(ctx context.Context, entityType, entityID, changedByID, field string, oldValue, newValue *string) error

	ClaimDueTaskDeadlineReminders(ctx context.Context, windowStart, windowEnd time.Time) ([]TaskDeadlineReminder, error)
	ListTaskWorkerUserIDs(ctx context.Context, taskID string) ([]string, error)
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

func (r *repository) CreateTask(ctx context.Context, projectAlias, userID string, req CreateTaskRequest) (*TaskResponse, error) {
	pid, err := r.projectIDByAlias(ctx, projectAlias)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	moduleID, err := nullableUUID(req.ModuleID)
	if err != nil {
		return nil, err
	}
	parentID, err := nullableUUID(req.ParentID)
	if err != nil {
		return nil, err
	}
	faultHolderID, err := nullableUUID(req.FaultHolderID)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	txQueries := database.New(tx)

	sequenceNo, err := txQueries.IncrementProjectTaskSequence(ctx, pid)
	if err != nil {
		return nil, err
	}

	taskRow, err := txQueries.CreateTask(ctx, database.CreateTaskParams{
		ProjectID:     pid,
		ModuleID:      pgtypeUUIDValue(moduleID),
		CreatedByID:   uid,
		ParentID:      pgtypeUUIDValue(parentID),
		Title:         req.Title,
		Description:   req.Description,
		Status:        database.TaskStatus(req.Status),
		Priority:      int32(*req.Priority),
		StartDate:     nullableTimeValue(req.StartDate),
		EndDate:       nullableTimeValue(req.EndDate),
		Estimation:    int32(req.Estimation),
		Archived:      req.Archived,
		FaultHolderID: pgtypeUUIDValue(faultHolderID),
		SequenceNo:    sequenceNo,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	task := taskResponseFromCreateTaskRow(taskRow)
	return r.hydrateTask(ctx, &task, userID)
}

// buildTaskListConditions returns a fresh argument accumulator and WHERE
// conditions for filter, scoped to project pid. Called once per query
// execution (never shared/reused across two queries) so each query gets its
// own independently-numbered $N placeholders.
func buildTaskListConditions(pid pgtype.UUID, filter ListTasksParams) (*sqlArgs, []string) {
	a := &sqlArgs{}
	conditions := []string{
		fmt.Sprintf("t.project_id = %s::uuid", a.bind(pid)),
		"t.deleted_at IS NULL",
	}
	if filter.ModuleID != "" {
		conditions = append(conditions, fmt.Sprintf("t.module_id = %s::uuid", a.bind(filter.ModuleID)))
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("t.status = %s::task_status", a.bind(filter.Status)))
	}
	if filter.Archived != "" {
		conditions = append(conditions, fmt.Sprintf("t.archived = %s::boolean", a.bind(filter.Archived)))
	}
	if filter.ParentID != "" {
		conditions = append(conditions, fmt.Sprintf("t.parent_id = %s::uuid", a.bind(filter.ParentID)))
	}
	if filter.CreatedByID != "" {
		conditions = append(conditions, fmt.Sprintf("t.created_by_id = %s::uuid", a.bind(filter.CreatedByID)))
	}
	if filter.Priority != "" {
		conditions = append(conditions, fmt.Sprintf("t.priority = %s::integer", a.bind(filter.Priority)))
	}
	if filter.Estimation != "" {
		conditions = append(conditions, fmt.Sprintf("t.estimation = %s::integer", a.bind(filter.Estimation)))
	}
	if filter.StartDateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("t.start_date >= %s::timestamptz", a.bind(filter.StartDateFrom)))
	}
	if filter.StartDateTo != "" {
		conditions = append(conditions, fmt.Sprintf("t.start_date <= %s::timestamptz", a.bind(filter.StartDateTo)))
	}
	if filter.EndDateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("t.end_date >= %s::timestamptz", a.bind(filter.EndDateFrom)))
	}
	if filter.EndDateTo != "" {
		conditions = append(conditions, fmt.Sprintf("t.end_date <= %s::timestamptz", a.bind(filter.EndDateTo)))
	}
	if filter.LabelID != "" {
		labelArg := a.bind(filter.LabelID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM task_label_tasks tlt
			INNER JOIN task_labels l ON l.id = tlt.label_id AND l.deleted_at IS NULL
			WHERE tlt.task_id = t.id AND l.id = %s::uuid
		)`, labelArg))
	}
	if filter.WorkerID != "" {
		workerArg := a.bind(filter.WorkerID)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM task_workers tw
			WHERE tw.task_id = t.id AND tw.user_id = %s::uuid
		)`, workerArg))
	}
	if filter.Query != "" {
		queryArg := a.bind(filter.Query)
		// The last OR arm matches by code/sequence number — e.g. pasting in
		// "#KAIRSO-42" or just "42" finds the task with that sequence_no,
		// ignoring the project-name prefix which isn't stored on the row.
		conditions = append(conditions, fmt.Sprintf(
			`(unaccent(lower(t.title)) LIKE '%%' || unaccent(lower(%[1]s)) || '%%' OR unaccent(lower(COALESCE(t.description, ''))) LIKE '%%' || unaccent(lower(%[1]s)) || '%%' OR
			  (regexp_replace(%[1]s, '\D', '', 'g') != '' AND t.sequence_no::text = regexp_replace(%[1]s, '\D', '', 'g')))`,
			queryArg,
		))
	}
	return a, conditions
}

// taskListStats aggregates COUNT/estimation over every row matching
// fromClause+conditions, with no LIMIT/OFFSET — used to report totals over
// the whole filtered set alongside a paginated task list.
func (r *repository) taskListStats(ctx context.Context, fromClause string, a *sqlArgs, conditions []string) (TaskListStats, error) {
	var stats TaskListStats
	query := fmt.Sprintf(`
		SELECT COUNT(*)::int, COALESCE(SUM(t.estimation), 0)::int
		FROM %s
		WHERE %s
	`, fromClause, strings.Join(conditions, " AND "))
	err := r.db.QueryRow(ctx, query, a.values...).Scan(&stats.TotalCount, &stats.TotalEstimation)
	return stats, err
}

func (r *repository) ListTasks(ctx context.Context, projectAlias, viewerID string, filter ListTasksParams, params pagination.Params) (TaskListResponse, error) {
	var empty TaskListResponse
	limit := params.LimitWithMore()
	offset := params.Offset()

	pid, err := pgutil.ParseUUID(projectAlias)
	if err != nil {
		return empty, err
	}

	a, conditions := buildTaskListConditions(pid, filter)
	orderBy := buildTaskOrderByClause(filter.SortRules)
	limitArg := a.bind(int32(limit))
	offsetArg := a.bind(int32(offset))

	query := fmt.Sprintf(`
		SELECT t.id, t.created_at, t.updated_at, t.deleted_at, t.project_id, t.module_id, t.created_by_id, t.parent_id,
		       t.title, t.description, t.status, t.priority, t.start_date, t.end_date, t.first_completed_at, t.estimation, t.archived, t.return_count, t.fault_holder_id, t.sequence_no
		FROM tasks t
		INNER JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		LEFT JOIN modules m ON m.id = t.module_id AND m.deleted_at IS NULL
		WHERE %s
		%s
		LIMIT %s OFFSET %s
	`, strings.Join(conditions, " AND "), orderBy, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, query, a.values...)
	if err != nil {
		return empty, err
	}
	defer rows.Close()

	tasks := make([]TaskResponse, 0)
	for rows.Next() {
		var row taskListRow
		if err := rows.Scan(
			&row.ID, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.ProjectID, &row.ModuleID, &row.CreatedByID, &row.ParentID,
			&row.Title, &row.Description, &row.Status, &row.Priority, &row.StartDate, &row.EndDate, &row.FirstCompletedAt, &row.Estimation, &row.Archived, &row.ReturnCount, &row.FaultHolderID, &row.SequenceNo,
		); err != nil {
			return empty, err
		}
		tasks = append(tasks, taskResponseFromListRow(row))
	}
	if err := rows.Err(); err != nil {
		return empty, err
	}
	rows.Close()

	// Hydrate the whole page in one batched pass (see hydrateTasks) instead
	// of once per row. Every row here belongs to the same project, so the
	// Code prefix only needs one name lookup for the whole page.
	if len(tasks) > 0 {
		taskPtrs := make([]*TaskResponse, len(tasks))
		for i := range tasks {
			taskPtrs[i] = &tasks[i]
		}
		if err := r.hydrateTasks(ctx, taskPtrs, viewerID); err != nil {
			return empty, err
		}
		projectName, err := r.projectNameByID(ctx, projectAlias)
		if err != nil {
			return empty, err
		}
		for i := range tasks {
			tasks[i].Code = Code(projectName, tasks[i].SequenceNo)
		}
	}

	statsArgs, statsConditions := buildTaskListConditions(pid, filter)
	stats, err := r.taskListStats(ctx, "tasks t", statsArgs, statsConditions)
	if err != nil {
		return empty, err
	}

	hasMore := pagination.Process(&tasks, params.Limit)
	return TaskListResponse{
		Response: pagination.NewResponse(tasks, hasMore, params.Page, params.Limit),
		Stats:    stats,
	}, nil
}

func (r *repository) GetTask(ctx context.Context, taskID string) (*TaskResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	taskRow, err := r.queries.GetTaskByID(ctx, tid)
	if err != nil {
		return nil, err
	}
	task := taskResponseFromGetTaskByIDRow(taskRow)
	// No caller identity available here (single-task fetch, e.g. the deep-
	// link fallback) — UnreadCommentCount stays 0 rather than threading a
	// viewer through every GetTask call site for this one field.
	return r.hydrateTask(ctx, &task, "")
}

func (r *repository) UpdateTask(ctx context.Context, taskID, callerID string, req UpdateTaskRequest) (*TaskResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	callerUID, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return nil, err
	}
	moduleID, err := nullableUUID(req.ModuleID)
	if err != nil {
		return nil, err
	}
	parentID, err := nullableUUID(req.ParentID)
	if err != nil {
		return nil, err
	}
	faultHolderID, err := nullableUUID(req.FaultHolderID)
	if err != nil {
		return nil, err
	}
	var status *string
	if req.Status != nil {
		value := string(*req.Status)
		status = &value
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	txQueries := database.New(tx)
	defer tx.Rollback(ctx)
	beforeRow, err := txQueries.GetTaskByIDForUpdate(ctx, tid)
	if err != nil {
		return nil, err
	}
	before := taskResponseFromGetTaskByIDForUpdateRow(beforeRow)

	var needsAuthCheck bool
	if req.returnCountPresent() && req.ReturnCount != nil && *req.ReturnCount != before.ReturnCount {
		needsAuthCheck = true
	}
	if req.endDatePresent() && !equalTimePtr(req.EndDate, before.EndDate) {
		needsAuthCheck = true
	}
	if req.estimationPresent() && req.Estimation != nil && *req.Estimation != before.Estimation {
		needsAuthCheck = true
	}
	if req.priorityPresent() && req.Priority != nil && *req.Priority != before.Priority {
		needsAuthCheck = true
	}

	if needsAuthCheck {
		isAuthorized, err := r.queries.CheckTaskAuth(ctx, database.CheckTaskAuthParams{TaskID: tid, UserID: callerUID})
		if err != nil {
			return nil, err
		}
		if !isAuthorized {
			return nil, fmt.Errorf("%w: chỉ quản trị viên hoặc chủ workspace mới được sửa số lần trả lại, hạn, ước tính và độ ưu tiên", ErrForbidden)
		}
	}

	taskRow, err := txQueries.UpdateTask(ctx, database.UpdateTaskParams{
		ModuleIDPresent:      req.moduleIDPresent(),
		ModuleID:             pgtypeUUIDValue(moduleID),
		ParentIDPresent:      req.parentIDPresent(),
		ParentID:             pgtypeUUIDValue(parentID),
		TitlePresent:         req.titlePresent(),
		Title:                stringValue(req.Title),
		DescriptionPresent:   req.descriptionPresent(),
		Description:          stringValue(req.Description),
		StatusPresent:        req.statusPresent(),
		Status:               stringValue(status),
		PriorityPresent:      req.priorityPresent(),
		Priority:             intValue(req.Priority),
		StartDatePresent:     req.startDatePresent(),
		StartDate:            nullableTimeValue(req.StartDate),
		EndDatePresent:       req.endDatePresent(),
		EndDate:              nullableTimeValue(req.EndDate),
		EstimationPresent:    req.estimationPresent(),
		Estimation:           intValue(req.Estimation),
		ArchivedPresent:      req.archivedPresent(),
		Archived:             boolValue(req.Archived),
		ReturnCountPresent:   req.returnCountPresent(),
		ReturnCount:          intValue(req.ReturnCount),
		FaultHolderIDPresent: req.faultHolderIDPresent(),
		FaultHolderID:        pgtypeUUIDValue(faultHolderID),
		ID:                   tid,
	})
	if err != nil {
		return nil, err
	}
	task := taskResponseFromUpdateTaskRow(taskRow)
	for _, change := range []struct {
		present bool
		field   string
		old     *string
		new     *string
	}{
		{req.moduleIDPresent(), "moduleId", uuidStringPtr(before.ModuleID), uuidStringPtr(task.ModuleID)},
		{req.parentIDPresent(), "parentId", uuidStringPtr(before.ParentID), uuidStringPtr(task.ParentID)},
		{req.titlePresent(), "title", stringPtr(before.Title), stringPtr(task.Title)},
		{req.descriptionPresent(), "description", nullableStringPtr(before.Description), nullableStringPtr(task.Description)},
		{req.statusPresent(), "status", stringPtr(string(before.Status)), stringPtr(string(task.Status))},
		{req.priorityPresent(), "priority", intPtrString(before.Priority), intPtrString(task.Priority)},
		{req.startDatePresent(), "startDate", timePtrString(before.StartDate), timePtrString(task.StartDate)},
		{req.endDatePresent(), "endDate", timePtrString(before.EndDate), timePtrString(task.EndDate)},
		{req.estimationPresent(), "estimation", intPtrString(before.Estimation), intPtrString(task.Estimation)},
		{req.archivedPresent(), "archived", boolPtrString(before.Archived), boolPtrString(task.Archived)},
		{req.returnCountPresent(), "return_count", intPtrString(before.ReturnCount), intPtrString(task.ReturnCount)},
		{req.faultHolderIDPresent(), "fault_holder_id", uuidStringPtr(before.FaultHolderID), uuidStringPtr(task.FaultHolderID)},
	} {
		if change.present {
			if err := insertHistory(ctx, txQueries, "task", task.ID, callerUID, change.field, change.old, change.new); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.hydrateTask(ctx, &task, callerID)
}

func (r *repository) ListTaskHistory(ctx context.Context, taskID string, params pagination.Params) (pagination.Response[TaskHistoryResponse], error) {
	var empty pagination.Response[TaskHistoryResponse]
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return empty, err
	}
	limit := params.LimitWithMore()
	offset := params.Offset()
	rows, err := r.queries.ListTaskHistories(ctx, database.ListTaskHistoriesParams{ID: tid, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return empty, err
	}
	items := make([]TaskHistoryResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, taskHistoryResponseFromListRow(row))
	}
	hasMore := pagination.Process(&items, params.Limit)
	return pagination.NewResponse(items, hasMore, params.Page, params.Limit), nil
}

func (r *repository) AddAttachment(ctx context.Context, taskID, uploaderID string, req CreateTaskAttachmentRequest) (*TaskAttachmentResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(uploaderID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateEntityAttachment(ctx, database.CreateEntityAttachmentParams{
		EntityType:  "task",
		EntityID:    tid,
		UploaderID:  uid,
		FileName:    req.FileName,
		FileUrl:     req.FileURL,
		ContentType: pgtypeTextFromString(req.ContentType),
		SizeBytes:   pgtypeInt8(req.SizeBytes),
	})
	if err != nil {
		return nil, err
	}
	return &TaskAttachmentResponse{
		ID:          pgutil.UUIDToString(row.ID),
		CreatedAt:   row.CreatedAt.Time,
		TaskID:      pgutil.UUIDToString(row.EntityID),
		UploaderID:  pgutil.UUIDToString(row.UploaderID),
		FileName:    row.FileName,
		FileURL:     row.FileUrl,
		ContentType: textString(row.ContentType),
		SizeBytes:   int8Value(row.SizeBytes),
	}, nil
}

func (r *repository) ListAttachments(ctx context.Context, taskID string) ([]TaskAttachmentResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListEntityAttachmentsByEntity(ctx, database.ListEntityAttachmentsByEntityParams{
		EntityType: "task",
		EntityID:   tid,
	})
	if err != nil {
		return nil, err
	}
	items := make([]TaskAttachmentResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, TaskAttachmentResponse{
			ID:            pgutil.UUIDToString(row.ID),
			CreatedAt:     row.CreatedAt.Time,
			TaskID:        pgutil.UUIDToString(row.EntityID),
			UploaderID:    pgutil.UUIDToString(row.UploaderID),
			UploaderAlias: row.UploaderAlias,
			UploaderName:  row.UploaderName,
			FileName:      row.FileName,
			FileURL:       row.FileUrl,
			ContentType:   textString(row.ContentType),
			SizeBytes:     int8Value(row.SizeBytes),
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
		EntityType: "task",
	})
}

// entityTypeTaskComment tags rows in the shared entity_attachments table as
// belonging to a task_comments row (entity_id = that comment's id) — same
// polymorphic reuse as "task"/"module" attachments, just a new entity kind.
const entityTypeTaskComment = "task_comment"

func (r *repository) CreateTaskComment(ctx context.Context, taskID, authorID, content string, attachments []CreateTaskCommentAttachment) (*TaskCommentResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	aid, err := pgutil.ParseUUID(authorID)
	if err != nil {
		return nil, err
	}

	// Transactional so a comment never ends up with only some of its
	// attachments saved if one insert fails partway through.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)

	row, err := q.CreateTaskComment(ctx, database.CreateTaskCommentParams{
		TaskID:   tid,
		AuthorID: aid,
		Content:  content,
	})
	if err != nil {
		return nil, err
	}

	respAttachments := make([]TaskCommentAttachmentResponse, 0, len(attachments))
	for _, a := range attachments {
		arow, err := q.CreateEntityAttachment(ctx, database.CreateEntityAttachmentParams{
			EntityType:  entityTypeTaskComment,
			EntityID:    row.ID,
			UploaderID:  aid,
			FileName:    a.FileName,
			FileUrl:     a.FileURL,
			ContentType: pgtypeTextFromString(a.ContentType),
			SizeBytes:   pgtypeInt8(a.SizeBytes),
		})
		if err != nil {
			return nil, err
		}
		respAttachments = append(respAttachments, TaskCommentAttachmentResponse{
			ID:          pgutil.UUIDToString(arow.ID),
			CreatedAt:   arow.CreatedAt.Time,
			CommentID:   pgutil.UUIDToString(arow.EntityID),
			UploaderID:  pgutil.UUIDToString(arow.UploaderID),
			FileName:    arow.FileName,
			FileURL:     arow.FileUrl,
			ContentType: textString(arow.ContentType),
			SizeBytes:   int8Value(arow.SizeBytes),
		})
	}

	// Persist mentions (not just re-derive them from content on every read)
	// so "tasks I'm mentioned in" is an indexed join — see task_comment_mentions
	// and Repository.ListThreads.
	if err := r.syncCommentMentions(ctx, q, row.ID, tid, content); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	resp := &TaskCommentResponse{
		ID:               pgutil.UUIDToString(row.ID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
		TaskID:           pgutil.UUIDToString(row.TaskID),
		AuthorID:         pgutil.UUIDToString(row.AuthorID),
		Content:          row.Content,
		MentionedUserIDs: parseMentionedUserIDs(row.Content),
		Attachments:      respAttachments,
	}
	// ListTaskComments joins to the user table for author display info; this
	// single-row insert doesn't, so without this the comment the FE just
	// posted would render with no author name until the next full reload —
	// same reasoning as workflow.repository.CreateComment.
	if user, err := r.queries.GetUserByID(ctx, aid); err == nil {
		resp.AuthorAlias = user.Alias
		resp.AuthorName = user.Name
		resp.AuthorAvatar = textString(user.Avatar)
	}
	return resp, nil
}

// syncCommentMentions replaces commentID's task_comment_mentions rows with
// whatever @[Name](userId) tokens are in content right now — called on both
// create (where delete is a harmless no-op) and edit (where it can add or
// remove mentions), so ListThreads' "am I mentioned" join always reflects
// the comment's current content, not just what it said when first posted.
// q lets this run inside the caller's own transaction.
func (r *repository) syncCommentMentions(ctx context.Context, q *database.Queries, commentID, taskID pgtype.UUID, content string) error {
	if err := q.DeleteTaskCommentMentionsByComment(ctx, commentID); err != nil {
		return err
	}
	for _, userID := range parseMentionedUserIDs(content) {
		uid, err := pgutil.ParseUUID(userID)
		if err != nil {
			// Malformed id inside an otherwise well-formed mention token —
			// shouldn't happen, not worth failing the whole comment over.
			continue
		}
		if err := q.CreateTaskCommentMention(ctx, database.CreateTaskCommentMentionParams{
			CommentID: commentID,
			TaskID:    taskID,
			UserID:    uid,
		}); err != nil {
			return err
		}
	}
	return nil
}

// listCommentAttachments fetches the entity_attachments rows a comment
// owns, shared by every place that builds a TaskCommentResponse.
func (r *repository) listCommentAttachments(ctx context.Context, commentID pgtype.UUID) ([]TaskCommentAttachmentResponse, error) {
	arows, err := r.queries.ListEntityAttachmentsByEntity(ctx, database.ListEntityAttachmentsByEntityParams{
		EntityType: entityTypeTaskComment,
		EntityID:   commentID,
	})
	if err != nil {
		return nil, err
	}
	attachments := make([]TaskCommentAttachmentResponse, 0, len(arows))
	for _, arow := range arows {
		attachments = append(attachments, TaskCommentAttachmentResponse{
			ID:          pgutil.UUIDToString(arow.ID),
			CreatedAt:   arow.CreatedAt.Time,
			CommentID:   pgutil.UUIDToString(arow.EntityID),
			UploaderID:  pgutil.UUIDToString(arow.UploaderID),
			FileName:    arow.FileName,
			FileURL:     arow.FileUrl,
			ContentType: textString(arow.ContentType),
			SizeBytes:   int8Value(arow.SizeBytes),
		})
	}
	return attachments, nil
}

func (r *repository) ListTaskComments(ctx context.Context, taskID string) ([]TaskCommentResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskCommentsByTask(ctx, tid)
	if err != nil {
		return nil, err
	}
	items := make([]TaskCommentResponse, 0, len(rows))
	for _, row := range rows {
		attachments, err := r.listCommentAttachments(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, TaskCommentResponse{
			ID:               pgutil.UUIDToString(row.ID),
			CreatedAt:        row.CreatedAt.Time,
			UpdatedAt:        row.UpdatedAt.Time,
			TaskID:           pgutil.UUIDToString(row.TaskID),
			AuthorID:         pgutil.UUIDToString(row.AuthorID),
			AuthorAlias:      row.AuthorAlias,
			AuthorName:       row.AuthorName,
			AuthorAvatar:     textString(row.AuthorAvatar),
			Content:          row.Content,
			MentionedUserIDs: parseMentionedUserIDs(row.Content),
			Attachments:      attachments,
		})
	}
	return items, nil
}

func (r *repository) ListTaskCommentAuthorIDs(ctx context.Context, taskID string) ([]string, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskCommentAuthorIDs(ctx, tid)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, id := range rows {
		ids = append(ids, pgutil.UUIDToString(id))
	}
	return ids, nil
}

func (r *repository) GetTaskComment(ctx context.Context, commentID string) (*TaskCommentResponse, error) {
	cid, err := pgutil.ParseUUID(commentID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetTaskCommentByID(ctx, cid)
	if err != nil {
		return nil, err
	}
	attachments, err := r.listCommentAttachments(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return &TaskCommentResponse{
		ID:               pgutil.UUIDToString(row.ID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
		TaskID:           pgutil.UUIDToString(row.TaskID),
		AuthorID:         pgutil.UUIDToString(row.AuthorID),
		AuthorAlias:      row.AuthorAlias,
		AuthorName:       row.AuthorName,
		AuthorAvatar:     textString(row.AuthorAvatar),
		Content:          row.Content,
		MentionedUserIDs: parseMentionedUserIDs(row.Content),
		Attachments:      attachments,
	}, nil
}

func (r *repository) UpdateTaskComment(ctx context.Context, commentID, content string) (*TaskCommentResponse, error) {
	cid, err := pgutil.ParseUUID(commentID)
	if err != nil {
		return nil, err
	}

	// Transactional so a comment's mentions never end up out of sync with
	// its just-saved content if the mention re-sync below fails partway.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)

	row, err := q.UpdateTaskCommentByID(ctx, database.UpdateTaskCommentByIDParams{
		ID:      cid,
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	if err := r.syncCommentMentions(ctx, q, row.ID, row.TaskID, content); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	attachments, err := r.listCommentAttachments(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	resp := &TaskCommentResponse{
		ID:               pgutil.UUIDToString(row.ID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
		TaskID:           pgutil.UUIDToString(row.TaskID),
		AuthorID:         pgutil.UUIDToString(row.AuthorID),
		Content:          row.Content,
		MentionedUserIDs: parseMentionedUserIDs(row.Content),
		Attachments:      attachments,
	}
	// UpdateTaskCommentByID doesn't join to users (single-row update, same
	// as CreateTaskComment) — enrich separately so the FE doesn't lose the
	// author name/avatar it was already showing for this comment.
	if user, err := r.queries.GetUserByID(ctx, row.AuthorID); err == nil {
		resp.AuthorAlias = user.Alias
		resp.AuthorName = user.Name
		resp.AuthorAvatar = textString(user.Avatar)
	}
	return resp, nil
}

// DeleteTaskComment removes the comment and everything it owns
// (entity_attachments has no FK to task_comments — it's polymorphic —
// so those rows need an explicit delete or they'd become orphaned).
func (r *repository) DeleteTaskComment(ctx context.Context, commentID string) error {
	cid, err := pgutil.ParseUUID(commentID)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)
	if err := q.DeleteEntityAttachmentsByEntity(ctx, database.DeleteEntityAttachmentsByEntityParams{
		EntityType: entityTypeTaskComment,
		EntityID:   cid,
	}); err != nil {
		return err
	}
	if err := q.DeleteTaskCommentByID(ctx, cid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MarkTaskCommentsRead records that userID has seen taskID's comment thread
// as of now — see task_comment_reads. Called as a side effect of
// ListComments; "opening the thread" is what counts as reading it.
func (r *repository) MarkTaskCommentsRead(ctx context.Context, taskID, userID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.MarkTaskCommentsRead(ctx, database.MarkTaskCommentsReadParams{
		TaskID: tid,
		UserID: uid,
	})
}

// CountUnreadTaskComments is how many of taskID's comments were posted by
// someone other than viewerID after viewerID's last read watermark (or all
// of them, if viewerID has never opened the thread).
func (r *repository) CountUnreadTaskComments(ctx context.Context, taskID, viewerID string) (int, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return 0, err
	}
	vid, err := pgutil.ParseUUID(viewerID)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.CountUnreadTaskComments(ctx, database.CountUnreadTaskCommentsParams{
		TaskID: tid,
		UserID: vid,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *repository) DeleteTask(ctx context.Context, taskID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	return r.queries.SoftDeleteTask(ctx, tid)
}

func (r *repository) CheckTaskMembership(ctx context.Context, taskID, callerID string) (bool, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return false, err
	}
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return false, err
	}
	return r.queries.CheckTaskMembership(ctx, database.CheckTaskMembershipParams{TaskID: tid, UserID: uid})
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

func (r *repository) CheckLabelMembership(ctx context.Context, labelID, callerID string) (bool, error) {
	lid, err := pgutil.ParseUUID(labelID)
	if err != nil {
		return false, err
	}
	uid, err := pgutil.ParseUUID(callerID)
	if err != nil {
		return false, err
	}
	return r.queries.CheckLabelMembership(ctx, database.CheckLabelMembershipParams{LabelID: lid, UserID: uid})
}

func (r *repository) CreateTaskLabel(ctx context.Context, projectAlias, userID string, req CreateTaskLabelRequest) (*TaskLabelResponse, error) {
	pid, err := r.projectIDByAlias(ctx, projectAlias)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateTaskLabel(ctx, database.CreateTaskLabelParams{ProjectID: pid, CreatedByID: uid, Name: req.Name, Color: req.Color})
	if err != nil {
		return nil, err
	}
	resp := taskLabelResponseFromCreateRow(row)
	return &resp, nil
}

func (r *repository) ListTaskLabels(ctx context.Context, projectAlias string, params pagination.Params) (pagination.Response[TaskLabelResponse], error) {
	var empty pagination.Response[TaskLabelResponse]

	pid, err := pgutil.ParseUUID(projectAlias)
	if err != nil {
		return empty, err
	}
	rows, err := r.queries.ListTaskLabelsByProject(ctx, pid)
	if err != nil {
		return empty, err
	}
	items := make([]TaskLabelResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, taskLabelResponseFromListProjectRow(row))
	}
	hasMore := pagination.Process(&items, params.Limit)
	return pagination.NewResponse(items, hasMore, params.Page, params.Limit), nil
}

// ListTaskLabelsByProjectIDs is ListTaskLabels' unpaginated batch form —
// every label across all of projectIDs in one round trip, for callers that
// would otherwise call ListTaskLabels once per project (e.g. the My Tasks
// screen).
func (r *repository) ListTaskLabelsByProjectIDs(ctx context.Context, projectIDs []string) ([]TaskLabelResponse, error) {
	if len(projectIDs) == 0 {
		return []TaskLabelResponse{}, nil
	}
	ids := make([]pgtype.UUID, 0, len(projectIDs))
	for _, id := range projectIDs {
		pid, err := pgutil.ParseUUID(id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, pid)
	}
	rows, err := r.queries.ListTaskLabelsByProjectIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]TaskLabelResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, taskLabelResponseFromListProjectRow(row))
	}
	return items, nil
}

func (r *repository) UpdateTaskLabel(ctx context.Context, labelID string, req UpdateTaskLabelRequest) (*TaskLabelResponse, error) {
	lid, err := pgutil.ParseUUID(labelID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateTaskLabel(ctx, database.UpdateTaskLabelParams{ID: lid, Name: req.Name, Color: req.Color})
	if err != nil {
		return nil, err
	}
	resp := taskLabelResponseFromUpdateRow(row)
	return &resp, nil
}

func (r *repository) DeleteTaskLabel(ctx context.Context, labelID string) (string, error) {
	lid, err := pgutil.ParseUUID(labelID)
	if err != nil {
		return "", err
	}
	pid, err := r.queries.SoftDeleteTaskLabel(ctx, lid)
	if err != nil {
		return "", err
	}
	return pgutil.UUIDToString(pid), nil
}

func (r *repository) AttachTaskLabel(ctx context.Context, taskID, labelID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	lid, err := pgutil.ParseUUID(labelID)
	if err != nil {
		return err
	}
	return r.queries.AttachTaskLabel(ctx, database.AttachTaskLabelParams{TaskID: tid, LabelID: lid})
}

func (r *repository) DetachTaskLabel(ctx context.Context, taskID, labelID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	lid, err := pgutil.ParseUUID(labelID)
	if err != nil {
		return err
	}
	return r.queries.DetachTaskLabel(ctx, database.DetachTaskLabelParams{TaskID: tid, LabelID: lid})
}

func (r *repository) GetUserIDByAlias(ctx context.Context, userAlias string) (string, error) {
	row, err := r.queries.GetUserByAlias(ctx, userAlias)
	if err != nil {
		return "", err
	}
	return pgutil.UUIDToString(row.ID), nil
}

func (r *repository) GetUserAliasByID(ctx context.Context, userID string) (string, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return "", err
	}
	row, err := r.queries.GetUserByID(ctx, uid)
	if err != nil {
		return "", err
	}
	return row.Alias, nil
}

// AssignTaskWorker assigns a worker by user ID (upsert).
func (r *repository) AssignTaskWorker(ctx context.Context, taskID, userID string, weight int) (*TaskWorkerResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	if weight <= 0 {
		weight = 1
	}
	row, err := r.queries.AssignTaskWorker(ctx, database.AssignTaskWorkerParams{
		TaskID: tid,
		UserID: uid,
		Weight: int32(weight),
	})
	if err != nil {
		return nil, err
	}
	resp := taskWorkerResponseFromAssignRow(row)
	return &resp, nil
}

// RemoveTaskWorker removes a worker by user ID.
func (r *repository) RemoveTaskWorker(ctx context.Context, taskID, userID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.RemoveTaskWorker(ctx, database.RemoveTaskWorkerParams{TaskID: tid, UserID: uid})
}

// AssignTaskFaultHolder assigns a fault holder by user ID (upsert).
func (r *repository) AssignTaskFaultHolder(ctx context.Context, taskID, userID string, weight int) (*TaskFaultHolderResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	if weight <= 0 {
		weight = 1
	}
	row, err := r.queries.AssignTaskFaultHolder(ctx, database.AssignTaskFaultHolderParams{
		TaskID: tid,
		UserID: uid,
		Weight: int32(weight),
	})
	if err != nil {
		return nil, err
	}
	resp := taskFaultHolderResponseFromAssignRow(row)
	return &resp, nil
}

// RemoveTaskFaultHolder removes a fault holder by user ID.
func (r *repository) RemoveTaskFaultHolder(ctx context.Context, taskID, userID string) error {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.RemoveTaskFaultHolder(ctx, database.RemoveTaskFaultHolderParams{TaskID: tid, UserID: uid})
}

// hydrateTask fills in everything about resp that isn't in the tasks table
// row itself. viewerID additionally scopes UnreadCommentCount to that
// caller — pass "" to skip it (leaves the zero value) where no caller
// identity is available or the count isn't worth the extra query.
func (r *repository) hydrateTask(ctx context.Context, resp *TaskResponse, viewerID string) (*TaskResponse, error) {
	labels, err := r.listLabelsForTask(ctx, resp.ID)
	if err != nil {
		return nil, err
	}
	workers, err := r.listWorkersForTask(ctx, resp.ID)
	if err != nil {
		return nil, err
	}
	faultHolders, err := r.listFaultHoldersForTask(ctx, resp.ID)
	if err != nil {
		return nil, err
	}
	projectName, err := r.projectNameByID(ctx, resp.ProjectID)
	if err != nil {
		return nil, err
	}
	tid, err := pgutil.ParseUUID(resp.ID)
	if err != nil {
		return nil, err
	}
	feedbackCount, err := r.queries.CountFeedbacksByTaskID(ctx, tid)
	if err != nil {
		return nil, err
	}
	resp.Labels = labels
	resp.Workers = workers
	resp.FaultHolders = faultHolders
	resp.Code = Code(projectName, resp.SequenceNo)
	resp.LinkedFeedbackCount = int(feedbackCount)
	if viewerID != "" {
		if unread, err := r.CountUnreadTaskComments(ctx, resp.ID, viewerID); err == nil {
			resp.UnreadCommentCount = unread
		}
	}
	return resp, nil
}

func (r *repository) projectNameByID(ctx context.Context, projectID string) (string, error) {
	pid, err := pgutil.ParseUUID(projectID)
	if err != nil {
		return "", err
	}
	name, err := r.queries.GetProjectNameByID(ctx, pid)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (r *repository) listLabelsForTask(ctx context.Context, taskID string) ([]TaskLabelResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskLabelsByTask(ctx, tid)
	if err != nil {
		return nil, err
	}
	labels := make([]TaskLabelResponse, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, taskLabelResponseFromListTaskRow(row))
	}
	return labels, nil
}

func (r *repository) listWorkersForTask(ctx context.Context, taskID string) ([]TaskWorkerResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskWorkersWithUser(ctx, database.ListTaskWorkersWithUserParams{TaskID: tid, Limit: 1000, Offset: 0})
	if err != nil {
		return nil, err
	}
	workers := make([]TaskWorkerResponse, 0, len(rows))
	for _, row := range rows {
		workers = append(workers, taskWorkerResponseFromListRow(row))
	}
	return workers, nil
}

func (r *repository) listFaultHoldersForTask(ctx context.Context, taskID string) ([]TaskFaultHolderResponse, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskFaultHoldersWithUser(ctx, database.ListTaskFaultHoldersWithUserParams{TaskID: tid, Limit: 1000, Offset: 0})
	if err != nil {
		return nil, err
	}
	faultHolders := make([]TaskFaultHolderResponse, 0, len(rows))
	for _, row := range rows {
		faultHolders = append(faultHolders, taskFaultHolderResponseFromListRow(row))
	}
	return faultHolders, nil
}

// hydrateTasks is hydrateTask's batched counterpart: instead of 5-6
// sequential round trips per task, it fires one batched query per concern
// (labels/workers/faultHolders/feedback count/unread count) for the whole
// page, run concurrently, then does a single in-memory pass to assign
// results back onto each resps[i]. This is the fix for the N+1 pattern
// hydrateTask has when called inside a per-row scan loop — see ListTasks,
// ListMyTasks, ListThreads, all of which now call this once per page
// instead of hydrateTask once per row.
//
// Unlike hydrateTask, this does NOT set Code — that needs a project name,
// and callers of hydrateTasks span either one project (ListTasks, which can
// resolve the name once for the whole page) or many (ListMyTasks,
// ListThreads, which resolve names via projectNamesByIDs) — so callers set
// Code themselves after this returns.
func (r *repository) hydrateTasks(ctx context.Context, resps []*TaskResponse, viewerID string) error {
	if len(resps) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, 0, len(resps))
	for _, resp := range resps {
		tid, err := pgutil.ParseUUID(resp.ID)
		if err != nil {
			return err
		}
		ids = append(ids, tid)
	}

	var (
		labelsByTask        map[string][]TaskLabelResponse
		workersByTask       map[string][]TaskWorkerResponse
		faultHoldersByTask  map[string][]TaskFaultHolderResponse
		feedbackCountByTask map[string]int
		unreadByTask        map[string]int
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		labelsByTask, err = r.listLabelsForTasks(gctx, ids)
		return err
	})
	g.Go(func() (err error) {
		workersByTask, err = r.listWorkersForTasks(gctx, ids)
		return err
	})
	g.Go(func() (err error) {
		faultHoldersByTask, err = r.listFaultHoldersForTasks(gctx, ids)
		return err
	})
	g.Go(func() (err error) {
		feedbackCountByTask, err = r.countFeedbacksForTasks(gctx, ids)
		return err
	})
	if viewerID != "" {
		g.Go(func() (err error) {
			unreadByTask, err = r.countUnreadCommentsForTasks(gctx, ids, viewerID)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	for _, resp := range resps {
		resp.Labels = labelsByTask[resp.ID]
		resp.Workers = workersByTask[resp.ID]
		resp.FaultHolders = faultHoldersByTask[resp.ID]
		resp.LinkedFeedbackCount = feedbackCountByTask[resp.ID]
		if viewerID != "" {
			resp.UnreadCommentCount = unreadByTask[resp.ID]
		}
	}
	return nil
}

// listLabelsForTasks batches listLabelsForTask across taskIDs. Every ID
// gets an entry in the result (defaulting to a non-nil empty slice, not a
// missing key) so callers get the same "[]" — never "null" — JSON shape
// for a task with no labels as the single-task path produces.
func (r *repository) listLabelsForTasks(ctx context.Context, taskIDs []pgtype.UUID) (map[string][]TaskLabelResponse, error) {
	result := make(map[string][]TaskLabelResponse, len(taskIDs))
	for _, id := range taskIDs {
		result[pgutil.UUIDToString(id)] = []TaskLabelResponse{}
	}
	rows, err := r.queries.ListTaskLabelsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := pgutil.UUIDToString(row.TaskID)
		result[key] = append(result[key], taskLabelResponseFromBatchRow(row))
	}
	return result, nil
}

// listWorkersForTasks batches listWorkersForTask across taskIDs — see
// listLabelsForTasks for the "every ID gets a non-nil entry" contract.
func (r *repository) listWorkersForTasks(ctx context.Context, taskIDs []pgtype.UUID) (map[string][]TaskWorkerResponse, error) {
	result := make(map[string][]TaskWorkerResponse, len(taskIDs))
	for _, id := range taskIDs {
		result[pgutil.UUIDToString(id)] = []TaskWorkerResponse{}
	}
	rows, err := r.queries.ListTaskWorkersWithUserByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := pgutil.UUIDToString(row.TaskID)
		result[key] = append(result[key], taskWorkerResponseFromBatchRow(row))
	}
	return result, nil
}

// listFaultHoldersForTasks batches listFaultHoldersForTask across taskIDs —
// see listLabelsForTasks for the "every ID gets a non-nil entry" contract.
func (r *repository) listFaultHoldersForTasks(ctx context.Context, taskIDs []pgtype.UUID) (map[string][]TaskFaultHolderResponse, error) {
	result := make(map[string][]TaskFaultHolderResponse, len(taskIDs))
	for _, id := range taskIDs {
		result[pgutil.UUIDToString(id)] = []TaskFaultHolderResponse{}
	}
	rows, err := r.queries.ListTaskFaultHoldersWithUserByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := pgutil.UUIDToString(row.TaskID)
		result[key] = append(result[key], taskFaultHolderResponseFromBatchRow(row))
	}
	return result, nil
}

// countFeedbacksForTasks batches CountFeedbacksByTaskID across taskIDs. A
// task absent from the query result (zero linked feedbacks) is simply
// absent from the map — callers read it with a plain map index, which
// yields the int zero value.
func (r *repository) countFeedbacksForTasks(ctx context.Context, taskIDs []pgtype.UUID) (map[string]int, error) {
	rows, err := r.queries.CountFeedbacksByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		result[pgutil.UUIDToString(row.TaskID)] = int(row.Count)
	}
	return result, nil
}

// countUnreadCommentsForTasks batches CountUnreadTaskComments across
// taskIDs for a single fixed viewer.
func (r *repository) countUnreadCommentsForTasks(ctx context.Context, taskIDs []pgtype.UUID, viewerID string) (map[string]int, error) {
	vid, err := pgutil.ParseUUID(viewerID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.CountUnreadTaskCommentsByTaskIDs(ctx, database.CountUnreadTaskCommentsByTaskIDsParams{
		TaskIds: taskIDs,
		UserID:  vid,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		result[pgutil.UUIDToString(row.TaskID)] = int(row.Count)
	}
	return result, nil
}

// projectNamesByIDs batches projectNameByID across projectIDs — used by
// list endpoints that span multiple projects (ListMyTasks, ListThreads) to
// resolve every row's Code in one query instead of once per task. A project
// absent from the result (e.g. deleted between the task query and this one)
// simply yields "" from a plain map index.
func (r *repository) projectNamesByIDs(ctx context.Context, projectIDs []pgtype.UUID) (map[string]string, error) {
	rows, err := r.queries.GetProjectNamesByIDs(ctx, projectIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[pgutil.UUIDToString(row.ID)] = row.Name
	}
	return result, nil
}

// assignTaskCodes sets Code on every element of resps, resolving project
// names for however many distinct projects they span in one batched query
// (via projectNamesByIDs) rather than one lookup per task. Used by
// ListMyTasks/ListThreads, whose rows aren't all from the same project the
// way ListTasks's are.
func (r *repository) assignTaskCodes(ctx context.Context, resps []*TaskResponse) error {
	if len(resps) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	projectIDs := make([]pgtype.UUID, 0, len(resps))
	for _, resp := range resps {
		if _, ok := seen[resp.ProjectID]; ok {
			continue
		}
		seen[resp.ProjectID] = struct{}{}
		pid, err := pgutil.ParseUUID(resp.ProjectID)
		if err != nil {
			return err
		}
		projectIDs = append(projectIDs, pid)
	}
	names, err := r.projectNamesByIDs(ctx, projectIDs)
	if err != nil {
		return err
	}
	for _, resp := range resps {
		resp.Code = Code(names[resp.ProjectID], resp.SequenceNo)
	}
	return nil
}

func insertHistory(ctx context.Context, txQueries *database.Queries, entityType, entityID string, changedByID pgtype.UUID, field string, oldValue, newValue *string) error {
	if equalStringPtr(oldValue, newValue) {
		return nil
	}
	oldValue = resolveHistoryDisplayValue(ctx, txQueries, field, oldValue)
	newValue = resolveHistoryDisplayValue(ctx, txQueries, field, newValue)
	eid, err := pgutil.ParseUUID(entityID)
	if err != nil {
		return err
	}
	return txQueries.InsertEntityHistory(ctx, database.InsertEntityHistoryParams{
		EntityType:  entityType,
		EntityID:    eid,
		ChangedByID: changedByID,
		Field:       field,
		OldValue:    pgtypeText(oldValue),
		NewValue:    pgtypeText(newValue),
	})
}

// resolveHistoryDisplayValue turns an ID stored in a history field into a
// human-readable label (module name, task title, or user display name) at
// write time, so history entries never surface a raw UUID to the client.
// Resolving here (rather than when the history is later read) means the
// label reflects what the referenced entity was called at the moment of the
// change, and a later rename can't retroactively rewrite past history.
func resolveHistoryDisplayValue(ctx context.Context, q *database.Queries, field string, value *string) *string {
	if value == nil {
		return nil
	}
	id, err := pgutil.ParseUUID(*value)
	if err != nil {
		return value
	}
	switch field {
	case "moduleId":
		if m, err := q.GetModuleByID(ctx, id); err == nil {
			return &m.Name
		}
	case "parentId":
		if t, err := q.GetTaskByID(ctx, id); err == nil {
			return &t.Title
		}
	case "fault_holder_id", "workers", "fault_holders":
		if u, err := q.GetUserByID(ctx, id); err == nil {
			return &u.Name
		}
	}
	return value
}

func (r *repository) InsertEntityHistory(ctx context.Context, entityType, entityID, changedByID, field string, oldValue, newValue *string) error {
	uid, err := pgutil.ParseUUID(changedByID)
	if err != nil {
		return err
	}
	return insertHistory(ctx, r.queries, entityType, entityID, uid, field, oldValue, newValue)
}

func stringPtr(value string) *string {
	return &value
}

func nullableStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func uuidStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPtrString(value int) *string {
	formatted := strconv.Itoa(value)
	return &formatted
}

func boolPtrString(value bool) *string {
	formatted := strconv.FormatBool(value)
	return &formatted
}

func timePtrString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func equalStringPtr(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func (r *repository) projectIDByAlias(ctx context.Context, projectAlias string) (pgtype.UUID, error) {
	return pgutil.ParseUUID(projectAlias)
}

// buildMyTaskListConditions returns a fresh argument accumulator and WHERE
// conditions for filter, scoped to workspace wid and worker uid. Called once
// per query execution — see buildTaskListConditions.
func buildMyTaskListConditions(wid, uid pgtype.UUID, filter ListMyTasksParams) (*sqlArgs, []string) {
	a := &sqlArgs{}
	conditions := []string{
		fmt.Sprintf("p.workspace_id = %s::uuid", a.bind(wid)),
		fmt.Sprintf("tw.user_id = %s::uuid", a.bind(uid)),
		"t.deleted_at IS NULL",
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("t.status = %s::task_status", a.bind(filter.Status)))
	}
	if filter.Archived != "" {
		conditions = append(conditions, fmt.Sprintf("t.archived = %s::boolean", a.bind(filter.Archived)))
	}
	if filter.Priority != "" {
		conditions = append(conditions, fmt.Sprintf("t.priority = %s::integer", a.bind(filter.Priority)))
	}
	if filter.Query != "" {
		queryArg := a.bind(filter.Query)
		conditions = append(conditions, fmt.Sprintf(
			`(unaccent(lower(t.title)) LIKE '%%' || unaccent(lower(%[1]s)) || '%%' OR
			  (regexp_replace(%[1]s, '\D', '', 'g') != '' AND t.sequence_no::text = regexp_replace(%[1]s, '\D', '', 'g')))`,
			queryArg,
		))
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, fmt.Sprintf("t.project_id = %s::uuid", a.bind(filter.ProjectID)))
	}
	if filter.StartDateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("t.start_date >= %s::timestamptz", a.bind(filter.StartDateFrom)))
	}
	if filter.StartDateTo != "" {
		conditions = append(conditions, fmt.Sprintf("t.start_date <= %s::timestamptz", a.bind(filter.StartDateTo)))
	}
	if filter.EndDateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("t.end_date >= %s::timestamptz", a.bind(filter.EndDateFrom)))
	}
	if filter.EndDateTo != "" {
		conditions = append(conditions, fmt.Sprintf("t.end_date <= %s::timestamptz", a.bind(filter.EndDateTo)))
	}
	return a, conditions
}

const myTaskListFromClause = `tasks t
	INNER JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
	INNER JOIN task_workers tw ON tw.task_id = t.id`

func (r *repository) ListMyTasks(ctx context.Context, workspaceID, userID string, filter ListMyTasksParams, params pagination.Params) (TaskListResponse, error) {
	var empty TaskListResponse
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return empty, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return empty, err
	}

	a, conditions := buildMyTaskListConditions(wid, uid, filter)
	orderBy := buildTaskOrderByClause(filter.SortRules)
	limitArg := a.bind(int32(params.LimitWithMore()))
	offsetArg := a.bind(int32(params.Offset()))

	query := fmt.Sprintf(`
		SELECT t.id, t.created_at, t.updated_at, t.deleted_at, t.project_id, t.module_id, t.created_by_id, t.parent_id,
		       t.title, t.description, t.status, t.priority, t.start_date, t.end_date, t.first_completed_at, t.estimation, t.archived, t.return_count, t.fault_holder_id, t.sequence_no
		FROM tasks t
		INNER JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
		INNER JOIN task_workers tw ON tw.task_id = t.id
		LEFT JOIN modules m ON m.id = t.module_id AND m.deleted_at IS NULL
		WHERE %s
		%s
		LIMIT %s OFFSET %s
	`, strings.Join(conditions, " AND "), orderBy, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, query, a.values...)
	if err != nil {
		return empty, err
	}
	defer rows.Close()

	tasks := make([]TaskResponse, 0)
	for rows.Next() {
		var row taskListRow
		if err := rows.Scan(
			&row.ID, &row.CreatedAt, &row.UpdatedAt, &row.DeletedAt, &row.ProjectID, &row.ModuleID, &row.CreatedByID, &row.ParentID,
			&row.Title, &row.Description, &row.Status, &row.Priority, &row.StartDate, &row.EndDate, &row.FirstCompletedAt, &row.Estimation, &row.Archived, &row.ReturnCount, &row.FaultHolderID, &row.SequenceNo,
		); err != nil {
			return empty, err
		}
		tasks = append(tasks, taskResponseFromListRow(row))
	}
	if err := rows.Err(); err != nil {
		return empty, err
	}
	rows.Close()

	// Hydrate the whole page in one batched pass (see hydrateTasks). Unlike
	// ListTasks, rows here span multiple projects, so Code needs a name
	// lookup per distinct project instead of a single one for the page.
	if len(tasks) > 0 {
		taskPtrs := make([]*TaskResponse, len(tasks))
		for i := range tasks {
			taskPtrs[i] = &tasks[i]
		}
		if err := r.hydrateTasks(ctx, taskPtrs, userID); err != nil {
			return empty, err
		}
		if err := r.assignTaskCodes(ctx, taskPtrs); err != nil {
			return empty, err
		}
	}

	statsArgs, statsConditions := buildMyTaskListConditions(wid, uid, filter)
	stats, err := r.taskListStats(ctx, myTaskListFromClause, statsArgs, statsConditions)
	if err != nil {
		return empty, err
	}

	hasMore := pagination.Process(&tasks, params.Limit)
	return TaskListResponse{
		Response: pagination.NewResponse(tasks, hasMore, params.Page, params.Limit),
		Stats:    stats,
	}, nil
}

func (r *repository) CountMyActiveTasks(ctx context.Context, workspaceID, userID string) (int64, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return 0, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return 0, err
	}
	return r.queries.CountMyActiveTasksByWorkspace(
		ctx,
		database.CountMyActiveTasksByWorkspaceParams{
			WorkspaceID: wid,
			UserID:      uid,
		},
	)
}

// threadResponseFromRow builds the embedded TaskResponse the same way
// taskResponseFromListRow does (same source columns, see thread.sql), plus
// the two thread-specific fields the LATERAL join/EXISTS in ListThreads
// compute per row.
func threadResponseFromRow(row database.ListThreadsRow) ThreadResponse {
	task := taskResponseFromListRow(taskListRow{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		ProjectID: row.ProjectID, ModuleID: row.ModuleID, CreatedByID: row.CreatedByID, ParentID: row.ParentID,
		Title: row.Title, Description: row.Description, Status: row.Status, Priority: row.Priority,
		StartDate: row.StartDate, EndDate: row.EndDate, FirstCompletedAt: row.FirstCompletedAt,
		Estimation: row.Estimation, Archived: row.Archived, ReturnCount: row.ReturnCount,
		FaultHolderID: row.FaultHolderID, SequenceNo: row.SequenceNo,
	})
	return ThreadResponse{
		TaskResponse: task,
		Mentioned:    row.Mentioned,
		LatestComment: &ThreadLatestCommentResponse{
			AuthorID:     pgutil.UUIDToString(row.LatestCommentAuthorID),
			AuthorAlias:  row.LatestCommentAuthorAlias,
			AuthorName:   row.LatestCommentAuthorName,
			AuthorAvatar: textString(row.LatestCommentAuthorAvatar),
			Content:      row.LatestCommentContent,
			CreatedAt:    row.LatestCommentCreatedAt.Time,
		},
	}
}

func (r *repository) ListThreads(ctx context.Context, workspaceID, userID string, params pagination.Params) (pagination.Response[ThreadResponse], error) {
	var empty pagination.Response[ThreadResponse]
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return empty, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return empty, err
	}

	rows, err := r.queries.ListThreads(ctx, database.ListThreadsParams{
		UserID:      uid,
		WorkspaceID: wid,
		OffsetCount: int32(params.Offset()),
		LimitCount:  int32(params.LimitWithMore()),
	})
	if err != nil {
		return empty, err
	}

	threads := make([]ThreadResponse, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, threadResponseFromRow(row))
	}

	if len(threads) > 0 {
		taskPtrs := make([]*TaskResponse, len(threads))
		for i := range threads {
			taskPtrs[i] = &threads[i].TaskResponse
		}
		if err := r.hydrateTasks(ctx, taskPtrs, userID); err != nil {
			return empty, err
		}
		if err := r.assignTaskCodes(ctx, taskPtrs); err != nil {
			return empty, err
		}
	}

	hasMore := pagination.Process(&threads, params.Limit)
	return pagination.NewResponse(threads, hasMore, params.Page, params.Limit), nil
}

func (r *repository) CountThreads(ctx context.Context, workspaceID, userID string) (int64, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return 0, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return 0, err
	}
	return r.queries.CountThreads(ctx, database.CountThreadsParams{
		WorkspaceID: wid,
		UserID:      uid,
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

func (r *repository) ClaimDueTaskDeadlineReminders(ctx context.Context, windowStart, windowEnd time.Time) ([]TaskDeadlineReminder, error) {
	rows, err := r.queries.ClaimDueTaskDeadlineReminders(ctx, database.ClaimDueTaskDeadlineRemindersParams{
		WindowStart: pgtype.Timestamptz{Time: windowStart, Valid: true},
		WindowEnd:   pgtype.Timestamptz{Time: windowEnd, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	items := make([]TaskDeadlineReminder, 0, len(rows))
	for _, row := range rows {
		items = append(items, TaskDeadlineReminder{
			TaskID:      pgutil.UUIDToString(row.TaskID),
			Title:       row.Title,
			ProjectID:   pgutil.UUIDToString(row.ProjectID),
			WorkspaceID: pgutil.UUIDToString(row.WorkspaceID),
			EndDate:     row.EndDate.Time,
		})
	}
	return items, nil
}

func (r *repository) ListTaskWorkerUserIDs(ctx context.Context, taskID string) ([]string, error) {
	tid, err := pgutil.ParseUUID(taskID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListTaskWorkerUserIDs(ctx, tid)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, pgutil.UUIDToString(row))
	}
	return items, nil
}

func pgtypeTextFromString(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func pgtypeInt8(value int64) pgtype.Int8 {
	if value == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func int8Value(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}
