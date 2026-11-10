package attendance

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/workspace"
	"github.com/alexcao194/kairso/pkg/pagination"
	"github.com/alexcao194/kairso/pkg/realtime"
)

var (
	ErrForbidden                  = errors.New("Bạn không có quyền thực hiện thao tác này")
	ErrAttendanceValidationFailed = errors.New("Xác thực chấm công thất bại")
	ErrInvalidWorkSchedule        = errors.New("Lịch làm việc không hợp lệ")
	ErrInvalidDailyReport         = errors.New("Báo cáo cuối ngày không hợp lệ")
)

// attendanceGracePeriod là mức du di khi tính đi muộn / về sớm: vào trễ hoặc
// về sớm trong vòng 5 phút so với ca đăng ký thì không bị tính.
const attendanceGracePeriod = 5 * time.Minute

type AttendanceSettingService interface {
	GetSettings(ctx context.Context, workspaceAlias, callerID string) (*AttendanceSettingResponse, error)
	UpsertSettings(ctx context.Context, workspaceAlias, callerID string, req UpsertAttendanceSettingRequest) (*AttendanceSettingResponse, error)
}

type AttendanceService interface {
	CheckStatus(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceStatusResponse, error)
	CheckIn(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceRecordResponse, error)
	CheckOut(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceRecordResponse, error)
	ListRecords(ctx context.Context, workspaceAlias, callerID string, filter AttendanceRecordFilter, params pagination.Params) (pagination.Response[AttendanceRecordResponse], error)
	DeleteRecord(ctx context.Context, workspaceAlias, callerID, recordID string) error
	// UpsertSession is the session-shaped counterpart to CreateRecord/
	// UpdateRecord/DeleteRecord — it lets a holder or admin add/correct/
	// clear a check-in and check-out together as one atomic write.
	UpsertSession(ctx context.Context, workspaceAlias, callerID string, req UpsertAttendanceSessionRequest) (*AttendanceSessionResponse, error)
}

type WorkScheduleService interface {
	UpsertWorkSchedules(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkSchedulesRequest) ([]WorkScheduleDayResponse, error)
	ListMySchedules(ctx context.Context, workspaceAlias, callerID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error)
	// UpsertMemberWorkSchedules lets a workspace HOLDER or ADMIN set
	// another member's registered work schedule.
	UpsertMemberWorkSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpsertMemberWorkSchedulesRequest) ([]WorkScheduleDayResponse, error)
	// ListMemberSchedules lets a HOLDER or ADMIN review a specific member's
	// schedule days (any status), e.g. to find days awaiting approval.
	ListMemberSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error)
	// ListPendingSchedules lets a HOLDER or ADMIN review pending schedule
	// days across every member in the workspace in one call, instead of
	// checking ListMemberSchedules member by member.
	ListPendingSchedules(ctx context.Context, workspaceAlias, callerID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error)
	// ApproveSchedules/RejectSchedules let a HOLDER or ADMIN decide on
	// pending self-registered schedule days by ID; only approved days feed
	// reminders and statistics.
	ApproveSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpdateWorkScheduleStatusRequest) ([]WorkScheduleDayResponse, error)
	RejectSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpdateWorkScheduleStatusRequest) ([]WorkScheduleDayResponse, error)
	// ApproveAllPendingSchedules lets a HOLDER or ADMIN approve every
	// pending self-registered day for a member in one action ("Apply
	// all"), sending a single notification instead of one per day.
	ApproveAllPendingSchedules(ctx context.Context, workspaceAlias, callerID, memberID string) ([]WorkScheduleDayResponse, error)
}

// AttendanceCorrectionRequestService lets a member submit a backdated
// check-in/check-out they forgot to record live, and lets a workspace
// HOLDER or ADMIN review it. Unlike work-schedule approvals, a decision
// here has a real side effect (creating/deleting an attendance_records row)
// that must stay in sync even when the reviewer changes their mind later —
// see decideCheckRequest.
type AttendanceCorrectionRequestService interface {
	Submit(ctx context.Context, workspaceAlias, callerID string, req SubmitAttendanceCorrectionRequest) (*AttendanceCorrectionRequestResponse, error)
	// List lets a HOLDER or ADMIN review every request in the workspace,
	// optionally filtered by status, across every member — the
	// Pending/Approved/Rejected management screen.
	List(ctx context.Context, workspaceAlias, callerID string, filter AttendanceCorrectionRequestFilter, params pagination.Params) (pagination.Response[AttendanceCorrectionRequestResponse], error)
	// ListMine lets any member see the status of their own submitted
	// requests.
	ListMine(ctx context.Context, workspaceAlias, callerID string, filter AttendanceCorrectionRequestFilter, params pagination.Params) (pagination.Response[AttendanceCorrectionRequestResponse], error)
	Approve(ctx context.Context, workspaceAlias, callerID, requestID string) (*AttendanceCorrectionRequestResponse, error)
	Reject(ctx context.Context, workspaceAlias, callerID, requestID string) (*AttendanceCorrectionRequestResponse, error)
}

type WorkWeekService interface {
	GetWorkWeek(ctx context.Context, workspaceAlias, callerID string) (*WorkWeekResponse, error)
	UpsertWorkWeek(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error)
	// GetMyWorkWeek / UpsertMyWorkWeek manage the caller's own personal
	// default weekly schedule (any member may save/load their own).
	GetMyWorkWeek(ctx context.Context, workspaceAlias, callerID string) (*WorkWeekResponse, error)
	UpsertMyWorkWeek(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error)
}

// DailyReportService quản lý báo cáo cuối ngày: nhân viên tự lưu/sửa báo cáo
// của mình theo ngày (checkout nhiều lần thì nạp lại bản cũ), còn HOLDER/ADMIN
// xem được báo cáo của mọi thành viên.
type DailyReportService interface {
	UpsertMyDailyReport(ctx context.Context, workspaceAlias, callerID string, req UpsertDailyReportRequest) (*DailyReportResponse, error)
	ListMyDailyReports(ctx context.Context, workspaceAlias, callerID string, filter DailyReportFilter, params pagination.Params) (pagination.Response[DailyReportResponse], error)
	// ListDailyReports lets a HOLDER or ADMIN read every member's daily
	// reports (optionally filtered by member) across a date range.
	ListDailyReports(ctx context.Context, workspaceAlias, callerID string, filter DailyReportFilter, params pagination.Params) (pagination.Response[DailyReportResponse], error)
}

type AttendanceStatisticsService interface {
	GetMonthlySummary(ctx context.Context, workspaceAlias, callerID string, filter AttendanceStatisticsFilter) (*AttendanceStatisticsSummaryResponse, error)
	GetMyAttendanceSummary(ctx context.Context, workspaceAlias, callerID string, filter AttendanceStatisticsFilter) (*MyAttendanceStatisticsResponse, error)
}

type TaskStatisticsService interface {
	GetTaskStatistics(ctx context.Context, workspaceAlias, callerID string, filter TaskStatisticsFilter) (*TaskStatisticsSummaryResponse, error)
	GetMyTaskStatistics(ctx context.Context, workspaceAlias, callerID string, filter TaskStatisticsFilter) (*MyTaskStatisticsResponse, error)
}

type ScheduleNotificationPublisher interface {
	PublishWorkScheduleRegistration(ctx context.Context, event ScheduleNotificationEvent) error
	PublishWorkScheduleDecision(ctx context.Context, event ScheduleNotificationEvent) error
	PublishAttendanceReminder(ctx context.Context, event AttendanceReminderEvent) error
	PublishDailyReportReminder(ctx context.Context, event DailyReportReminderEvent) error
	PublishCheckRequestSubmitted(ctx context.Context, event CheckRequestNotificationEvent) error
	PublishCheckRequestDecision(ctx context.Context, event CheckRequestNotificationEvent) error
}

type Repository interface {
	// ExecTx runs fn against a repository whose writes share one
	// transaction — see UpsertSession, which writes a check-in and
	// check-out record together and must not leave only one committed.
	ExecTx(ctx context.Context, fn func(Repository) error) error
	GetCallerRole(ctx context.Context, workspaceAlias, callerID string) (workspace.WorkspaceRole, error)
	CheckProjectManageAuth(ctx context.Context, projectID, callerID string) (bool, error)
	GetAttendanceSettings(ctx context.Context, workspaceAlias string) (*AttendanceSettingResponse, error)
	UpsertAttendanceSettings(ctx context.Context, workspaceAlias string, req UpsertAttendanceSettingRequest) (*AttendanceSettingResponse, error)
	CreateAttendanceRecord(ctx context.Context, workspaceAlias, memberID string, req createAttendanceRecordParams) (*AttendanceRecordResponse, error)
	ListAttendanceRecords(ctx context.Context, workspaceAlias, callerID string, filter AttendanceRecordFilter, params pagination.Params) (pagination.Response[AttendanceRecordResponse], error)
	GetAttendanceRecord(ctx context.Context, recordID string) (*AttendanceRecordResponse, error)
	UpdateAttendanceRecord(ctx context.Context, recordID string, recordedAt time.Time) (*AttendanceRecordResponse, error)
	DeleteAttendanceRecord(ctx context.Context, recordID string) error
	CreateCheckRequest(ctx context.Context, workspaceAlias, memberID string, params createCheckRequestParams) (*AttendanceCorrectionRequestResponse, error)
	GetCheckRequest(ctx context.Context, requestID string) (*AttendanceCorrectionRequestResponse, error)
	ListCheckRequests(ctx context.Context, workspaceAlias string, filter AttendanceCorrectionRequestFilter, params pagination.Params) (pagination.Response[AttendanceCorrectionRequestResponse], error)
	DecideCheckRequest(ctx context.Context, requestID string, status AttendanceCorrectionStatus, decidedBy string, resultCheckInRecordID, resultCheckOutRecordID *string) (*AttendanceCorrectionRequestResponse, error)
	ListCheckRequestNotificationRecipients(ctx context.Context, workspaceAlias, memberID string) ([]string, error)
	ReplaceWorkSchedule(ctx context.Context, workspaceAlias, memberID, registeredBy string, req UpsertWorkScheduleRequest, status WorkScheduleStatus, weekNote string) (*WorkScheduleDayResponse, error)
	ListWorkSchedules(ctx context.Context, workspaceAlias, callerID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error)
	UpdateWorkScheduleDayStatus(ctx context.Context, workspaceAlias, memberID string, dayIDs []string, status WorkScheduleStatus) ([]WorkScheduleDayResponse, error)
	ApproveAllPendingSchedules(ctx context.Context, workspaceAlias, memberID string) ([]WorkScheduleDayResponse, error)
	ListWorkspaceMembersForStatistics(ctx context.Context, workspaceAlias string, memberID string) ([]WorkspaceMemberBriefResponse, error)
	ListWorkspaceScheduleStats(ctx context.Context, workspaceAlias string, from, to time.Time, memberID string) ([]WorkScheduleDayResponse, error)
	ListWorkspaceAttendanceStats(ctx context.Context, workspaceAlias string, from, to time.Time, memberID string) ([]AttendanceRecordResponse, error)
	SumAssignedTaskHours(ctx context.Context, workspaceAlias, memberID string, from, to time.Time) (float64, error)
	ListScheduleNotificationRecipients(ctx context.Context, workspaceAlias, memberID string) ([]string, error)
	GetMemberName(ctx context.Context, memberID string) (string, error)
	GetTaskStatsByWorkspace(ctx context.Context, workspaceAlias string, filter TaskStatisticsFilter, from, to time.Time) ([]TaskStatsMemberResponse, error)
	GetTaskCycleTimeByWorkspace(ctx context.Context, workspaceAlias string, filter TaskStatisticsFilter, from, to time.Time) (avgDays float64, sampleSize int64, err error)
	GetWorkWeek(ctx context.Context, workspaceAlias string) (*WorkWeekResponse, error)
	ReplaceWorkWeek(ctx context.Context, workspaceAlias string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error)
	GetMemberWorkWeek(ctx context.Context, workspaceAlias, memberID string) (*WorkWeekResponse, error)
	ReplaceMemberWorkWeek(ctx context.Context, workspaceAlias, memberID string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error)
	ClaimDueAttendanceReminders(ctx context.Context, windowStart, windowEnd time.Time) ([]AttendanceReminder, error)
	UpsertDailyReport(ctx context.Context, workspaceAlias, memberID string, req UpsertDailyReportRequest) (*DailyReportResponse, error)
	ListDailyReports(ctx context.Context, workspaceAlias string, filter DailyReportFilter, params pagination.Params) (pagination.Response[DailyReportResponse], error)
	ClaimDueDailyReportReminders(ctx context.Context, windowStart, windowEnd time.Time) ([]DailyReportReminder, error)

	SumInitiativeEstimates(ctx context.Context, workspaceAlias, memberID string, from, to time.Time) (map[string]float64, error)
	SumLateLogEstimates(ctx context.Context, workspaceAlias, memberID string, from, to time.Time) (map[string]float64, error)
	SumFeedbackEstimates(ctx context.Context, workspaceAlias, memberID string, from, to time.Time) (map[string]float64, error)
}

type AttendanceReminderService interface {
	RunDue(ctx context.Context, now time.Time) error
	Start(ctx context.Context)
}

type services struct {
	repo      Repository
	publisher ScheduleNotificationPublisher
	realtime  realtime.Broadcaster
}

// SetRealtime wires the WebSocket broadcaster in; it's separate from the
// constructors so existing test/reminder call sites don't also need one.
func (s *services) SetRealtime(rt realtime.Broadcaster) {
	s.realtime = rt
}

func (s *services) broadcast(ctx context.Context, workspaceID, eventType, entityID, actorID string) {
	if s.realtime == nil || workspaceID == "" {
		return
	}
	s.realtime.Broadcast(realtime.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		EntityID:    entityID,
		ActorID:     actorID,
		ActorConnID: middleware.GetConnIDFromContext(ctx),
	})
}

func NewAttendanceReminderService(repo Repository, publisher ScheduleNotificationPublisher) AttendanceReminderService {
	return &services{repo: repo, publisher: publisher}
}

func (s *services) RunDue(ctx context.Context, now time.Time) error {
	target := now.UTC().Add(5 * time.Minute).Truncate(time.Minute)
	reminders, err := s.repo.ClaimDueAttendanceReminders(ctx, target, target.Add(time.Minute-time.Nanosecond))
	if err != nil {
		return err
	}
	for _, reminder := range reminders {
		event := AttendanceReminderEvent{
			RecipientID: reminder.MemberID, WorkspaceID: reminder.WorkspaceID,
			SessionID: reminder.SessionID, ScheduleDate: reminder.Date,
		}
		switch reminder.ReminderType {
		case AttendanceTypeCheckIn:
			event.Type = NotificationTypeAttendanceCheckInReminder
			event.SessionTime = reminder.StartAt.Format(time.RFC3339)
		case AttendanceTypeCheckOut:
			event.Type = NotificationTypeAttendanceCheckOutReminder
			event.SessionTime = reminder.EndAt.Format(time.RFC3339)
		default:
			continue
		}
		if err := s.publisher.PublishAttendanceReminder(ctx, event); err != nil {
			return err
		}
	}
	// Nhắc "chưa nộp báo cáo cuối ngày": bắn ở ca tan muộn nhất trừ 5 phút
	// nếu tới mốc đó mà thành viên vẫn chưa có báo cáo cho ngày hôm đó.
	reports, err := s.repo.ClaimDueDailyReportReminders(ctx, target, target.Add(time.Minute-time.Nanosecond))
	if err != nil {
		return err
	}
	for _, rep := range reports {
		if err := s.publisher.PublishDailyReportReminder(ctx, DailyReportReminderEvent{
			RecipientID: rep.MemberID,
			WorkspaceID: rep.WorkspaceID,
			Date:        rep.Date,
			LastEndAt:   rep.LastEndAt.Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *services) Start(ctx context.Context) {
	go func() {
		run := func() {
			if err := s.RunDue(ctx, time.Now()); err != nil {
				fmt.Printf("attendance reminder failed: %v\n", err)
			}
		}
		run()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func NewServices(repo Repository, publisher ScheduleNotificationPublisher) (*services, AttendanceSettingService, AttendanceService, WorkScheduleService, AttendanceStatisticsService, TaskStatisticsService, WorkWeekService, AttendanceCorrectionRequestService, DailyReportService) {
	s := &services{repo: repo, publisher: publisher}
	return s, s, s, s, s, s, s, s, s
}

func (s *services) GetSettings(ctx context.Context, workspaceAlias, callerID string) (*AttendanceSettingResponse, error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	return s.repo.GetAttendanceSettings(ctx, workspaceAlias)
}

func (s *services) UpsertSettings(ctx context.Context, workspaceAlias, callerID string, req UpsertAttendanceSettingRequest) (*AttendanceSettingResponse, error) {
	if err := normalizeAttendanceSettingRequest(&req); err != nil {
		return nil, err
	}
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	settings, err := s.repo.UpsertAttendanceSettings(ctx, workspaceAlias, req)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		s.broadcast(ctx, settings.WorkspaceID, "attendance_settings.updated", settings.WorkspaceID, callerID)
	}
	return settings, nil
}

func (s *services) CheckIn(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceRecordResponse, error) {
	return s.check(ctx, workspaceAlias, callerID, AttendanceTypeCheckIn, req)
}

func (s *services) CheckOut(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceRecordResponse, error) {
	return s.check(ctx, workspaceAlias, callerID, AttendanceTypeCheckOut, req)
}

// getNextAction determines whether [now] would count as a check-in or a
// check-out for callerID, by alternating off the last record at or before
// that instant on the same Vietnam-local calendar day. The upper bound
// (To: now) matters for backdated calls (attendance correction requests) —
// without it, a *later* record from the same day (e.g. today's live 5pm
// checkout) would wrongly be treated as "the last one" when computing the
// type for e.g. a 9am correction. It's a no-op for the live check-in/status
// call sites below, which always pass now = the current instant itself, so
// there's never a later same-day record to wrongly prefer.
func (s *services) getNextAction(ctx context.Context, workspaceAlias, callerID string, now time.Time) (AttendanceType, error) {
	nowVN := now.UTC().In(vietnamLocation())
	startOfDayVN := vietnamTime(nowVN.Year(), nowVN.Month(), nowVN.Day(), 0, 0)
	filter := AttendanceRecordFilter{
		MemberID: callerID,
		From:     startOfDayVN.Format(time.RFC3339),
		To:       now.Format(time.RFC3339),
	}
	resp, err := s.repo.ListAttendanceRecords(ctx, workspaceAlias, callerID, filter, pagination.Params{Page: 1, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return AttendanceTypeCheckIn, nil
	}
	if resp.Items[0].Type == AttendanceTypeCheckIn {
		return AttendanceTypeCheckOut, nil
	}
	return AttendanceTypeCheckIn, nil
}

func (s *services) CheckStatus(ctx context.Context, workspaceAlias, callerID string, req AttendanceCheckRequest) (*AttendanceStatusResponse, error) {
	if err := normalizeAttendanceCheckRequest(&req); err != nil {
		return nil, err
	}
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	settings, err := s.repo.GetAttendanceSettings(ctx, workspaceAlias)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if req.RecordedAt != nil {
		now = *req.RecordedAt
	}
	nextAction, err := s.getNextAction(ctx, workspaceAlias, callerID, now)
	if err != nil {
		return nil, err
	}

	distance := distanceMeters(req.Latitude, req.Longitude, settings.CompanyLatitude, settings.CompanyLongitude)
	resp := &AttendanceStatusResponse{
		Valid:          true,
		NextAction:     nextAction,
		DistanceMeters: distance,
		IsConfigured:   settings.WorkspaceID != "",
	}

	if settings.ValidationEnabled {
		if !strings.EqualFold(req.WifiBSSID, settings.CompanyWifiBSSID) {
			resp.Valid = false
			resp.ValidationMsg = "BSSID Wi-Fi không khớp"
		} else if distance > float64(settings.AllowedRadiusMeters) {
			resp.Valid = false
			resp.ValidationMsg = "Ngoài bán kính cho phép"
		}
	}
	return resp, nil
}

func (s *services) check(ctx context.Context, workspaceAlias, callerID string, typ AttendanceType, req AttendanceCheckRequest) (*AttendanceRecordResponse, error) {
	if err := normalizeAttendanceCheckRequest(&req); err != nil {
		return nil, err
	}
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	settings, err := s.repo.GetAttendanceSettings(ctx, workspaceAlias)
	if err != nil {
		return nil, err
	}
	recordedAt := time.Now().UTC()
	if req.RecordedAt != nil {
		recordedAt = *req.RecordedAt
	}

	nextAction, err := s.getNextAction(ctx, workspaceAlias, callerID, recordedAt)
	if err != nil {
		return nil, err
	}
	if nextAction != typ {
		if typ == AttendanceTypeCheckIn {
			return nil, fmt.Errorf("%w: bạn đã check-in rồi", ErrAttendanceValidationFailed)
		}
		return nil, fmt.Errorf("%w: bạn đã check-out hoặc chưa check-in", ErrAttendanceValidationFailed)
	}

	distance := distanceMeters(req.Latitude, req.Longitude, settings.CompanyLatitude, settings.CompanyLongitude)
	if settings.ValidationEnabled {
		if !strings.EqualFold(req.WifiBSSID, settings.CompanyWifiBSSID) {
			return nil, fmt.Errorf("%w: BSSID Wi-Fi không khớp", ErrAttendanceValidationFailed)
		}
		if distance > float64(settings.AllowedRadiusMeters) {
			return nil, fmt.Errorf("%w: ngoài bán kính cho phép", ErrAttendanceValidationFailed)
		}
	}
	// recordedAt already determined above
	record, err := s.repo.CreateAttendanceRecord(ctx, workspaceAlias, callerID, createAttendanceRecordParams{
		Type: typ, RecordedAt: recordedAt, Latitude: req.Latitude, Longitude: req.Longitude,
		WifiBSSID: req.WifiBSSID, WifiSSID: req.WifiSSID, DistanceMeters: distance,
	})
	if err != nil {
		return nil, err
	}
	s.broadcast(ctx, record.WorkspaceID, "attendance."+string(typ), record.ID, callerID)
	return record, nil
}

func (s *services) ListRecords(ctx context.Context, workspaceAlias, callerID string, filter AttendanceRecordFilter, params pagination.Params) (pagination.Response[AttendanceRecordResponse], error) {
	filter, err := s.attendanceFilterForRole(ctx, workspaceAlias, callerID, filter)
	if err != nil {
		return pagination.Response[AttendanceRecordResponse]{}, err
	}
	return s.repo.ListAttendanceRecords(ctx, workspaceAlias, callerID, filter, params)
}

// requireHolderOrAdmin returns ErrForbidden unless callerID is the HOLDER or
// ADMIN of workspaceAlias — same bar as roleCanViewAll/CanManageMembers.
// Correcting another member's schedule or attendance records needs this
// level of trust; a plain member never does.
func (s *services) requireHolderOrAdmin(ctx context.Context, workspaceAlias, callerID string) error {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return err
	}
	if !role.CanManageMembers() {
		return ErrForbidden
	}
	return nil
}

func (s *services) DeleteRecord(ctx context.Context, workspaceAlias, callerID, recordID string) error {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return err
	}
	existing, err := s.repo.GetAttendanceRecord(ctx, recordID)
	if err != nil {
		return err
	}
	if err := requireSameWorkspace(workspaceAlias, existing.WorkspaceID); err != nil {
		return err
	}
	if err := s.repo.DeleteAttendanceRecord(ctx, recordID); err != nil {
		return err
	}
	s.broadcast(ctx, existing.WorkspaceID, "attendance.deleted", recordID, callerID)
	return nil
}

// requireSameWorkspace guards against a holder of one workspace editing/
// deleting a record that (by guessed/leaked ID) actually belongs to a
// different workspace.
func requireSameWorkspace(workspaceAlias, recordWorkspaceID string) error {
	if !strings.EqualFold(workspaceAlias, recordWorkspaceID) {
		return ErrForbidden
	}
	return nil
}

// UpsertSession applies both halves of req inside one transaction, so a
// check-in and check-out are added/corrected/deleted together instead of as
// two independent CreateRecord/UpdateRecord/DeleteRecord calls that could
// leave only one side committed if the other failed.
func (s *services) UpsertSession(ctx context.Context, workspaceAlias, callerID string, req UpsertAttendanceSessionRequest) (*AttendanceSessionResponse, error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	if err := normalizeUpsertAttendanceSessionRequest(&req); err != nil {
		return nil, err
	}

	var resp AttendanceSessionResponse
	err := s.repo.ExecTx(ctx, func(repo Repository) error {
		checkIn, err := applyAttendanceHalf(ctx, repo, workspaceAlias, req.MemberID, AttendanceTypeCheckIn, req.CheckIn)
		if err != nil {
			return err
		}
		resp.CheckIn = checkIn
		checkOut, err := applyAttendanceHalf(ctx, repo, workspaceAlias, req.MemberID, AttendanceTypeCheckOut, req.CheckOut)
		if err != nil {
			return err
		}
		resp.CheckOut = checkOut
		return nil
	})
	if err != nil {
		return nil, err
	}

	if resp.CheckIn != nil {
		s.broadcast(ctx, resp.CheckIn.WorkspaceID, "attendance.check_in", resp.CheckIn.ID, callerID)
	}
	if resp.CheckOut != nil {
		s.broadcast(ctx, resp.CheckOut.WorkspaceID, "attendance.check_out", resp.CheckOut.ID, callerID)
	}
	return &resp, nil
}

// applyAttendanceHalf adds, corrects, or deletes one half of a session
// according to half.Enabled/half.RecordID, returning the resulting record —
// nil if the half ended up deleted, or was never recorded to begin with.
func applyAttendanceHalf(ctx context.Context, repo Repository, workspaceAlias, memberID string, recordType AttendanceType, half AttendanceHalfEdit) (*AttendanceRecordResponse, error) {
	if half.Enabled {
		if half.RecordID == "" {
			return repo.CreateAttendanceRecord(ctx, workspaceAlias, memberID, createAttendanceRecordParams{
				Type:       recordType,
				RecordedAt: half.RecordedAt,
			})
		}
		existing, err := repo.GetAttendanceRecord(ctx, half.RecordID)
		if err != nil {
			return nil, err
		}
		if err := requireSameWorkspace(workspaceAlias, existing.WorkspaceID); err != nil {
			return nil, err
		}
		return repo.UpdateAttendanceRecord(ctx, half.RecordID, half.RecordedAt)
	}
	if half.RecordID == "" {
		return nil, nil
	}
	existing, err := repo.GetAttendanceRecord(ctx, half.RecordID)
	if err != nil {
		return nil, err
	}
	if err := requireSameWorkspace(workspaceAlias, existing.WorkspaceID); err != nil {
		return nil, err
	}
	if err := repo.DeleteAttendanceRecord(ctx, half.RecordID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *services) Submit(ctx context.Context, workspaceAlias, callerID string, req SubmitAttendanceCorrectionRequest) (*AttendanceCorrectionRequestResponse, error) {
	if err := normalizeSubmitAttendanceCorrectionRequest(&req); err != nil {
		return nil, err
	}
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}

	params := createCheckRequestParams{
		Reason:     req.Reason,
		CheckInAt:  req.CheckInAt,
		CheckOutAt: req.CheckOutAt,
	}

	// A target*RecordID means this half CORRECTS an existing record the
	// member already has, rather than adding a new one — validate it's
	// really theirs (and really the right type) before trusting it, and
	// snapshot its current time so a later decision can always revert to a
	// clean slate.
	if req.TargetCheckInRecordID != "" {
		target, err := s.repo.GetAttendanceRecord(ctx, req.TargetCheckInRecordID)
		if err != nil {
			return nil, err
		}
		if target.MemberID != callerID || !strings.EqualFold(target.WorkspaceID, workspaceAlias) || target.Type != AttendanceTypeCheckIn {
			return nil, ErrForbidden
		}
		params.TargetCheckInRecordID = req.TargetCheckInRecordID
		original := target.RecordedAt
		params.OriginalCheckInAt = &original
	}
	if req.TargetCheckOutRecordID != "" {
		target, err := s.repo.GetAttendanceRecord(ctx, req.TargetCheckOutRecordID)
		if err != nil {
			return nil, err
		}
		if target.MemberID != callerID || !strings.EqualFold(target.WorkspaceID, workspaceAlias) || target.Type != AttendanceTypeCheckOut {
			return nil, ErrForbidden
		}
		params.TargetCheckOutRecordID = req.TargetCheckOutRecordID
		original := target.RecordedAt
		params.OriginalCheckOutAt = &original
	}

	request, err := s.repo.CreateCheckRequest(ctx, workspaceAlias, callerID, params)
	if err != nil {
		return nil, err
	}
	s.broadcast(ctx, request.WorkspaceID, "attendance_correction_request.created", request.ID, callerID)

	if s.publisher != nil {
		recipients, err := s.repo.ListCheckRequestNotificationRecipients(ctx, workspaceAlias, callerID)
		if err != nil {
			return nil, err
		}
		memberName, _ := s.repo.GetMemberName(ctx, callerID)
		for _, recipientID := range recipients {
			if err := s.publisher.PublishCheckRequestSubmitted(ctx, CheckRequestNotificationEvent{
				Type: NotificationTypeCheckRequestSubmitted, RecipientID: recipientID, WorkspaceID: request.WorkspaceID,
				WorkspaceAlias: workspaceAlias, RequestID: request.ID, MemberID: callerID, MemberName: memberName,
				CheckInAt:  formatOptionalTime(request.CheckInAt),
				CheckOutAt: formatOptionalTime(request.CheckOutAt),
			}); err != nil {
				return nil, err
			}
		}
	}
	return request, nil
}

func (s *services) List(ctx context.Context, workspaceAlias, callerID string, filter AttendanceCorrectionRequestFilter, params pagination.Params) (pagination.Response[AttendanceCorrectionRequestResponse], error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return pagination.Response[AttendanceCorrectionRequestResponse]{}, err
	}
	return s.repo.ListCheckRequests(ctx, workspaceAlias, filter, params)
}

func (s *services) ListMine(ctx context.Context, workspaceAlias, callerID string, filter AttendanceCorrectionRequestFilter, params pagination.Params) (pagination.Response[AttendanceCorrectionRequestResponse], error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return pagination.Response[AttendanceCorrectionRequestResponse]{}, err
	}
	if role == "" {
		return pagination.Response[AttendanceCorrectionRequestResponse]{}, ErrForbidden
	}
	filter.MemberID = callerID
	return s.repo.ListCheckRequests(ctx, workspaceAlias, filter, params)
}

func (s *services) Approve(ctx context.Context, workspaceAlias, callerID, requestID string) (*AttendanceCorrectionRequestResponse, error) {
	return s.decideCheckRequest(ctx, workspaceAlias, callerID, requestID, AttendanceCorrectionStatusApproved)
}

func (s *services) Reject(ctx context.Context, workspaceAlias, callerID, requestID string) (*AttendanceCorrectionRequestResponse, error) {
	return s.decideCheckRequest(ctx, workspaceAlias, callerID, requestID, AttendanceCorrectionStatusRejected)
}

// decideCheckRequest is the shared approve/reject path for attendance
// correction requests, holder/admin-only and safely re-decidable at any
// time. The check-in half and check-out half are handled fully
// independently (a session request can mix "add" for one half and "edit"
// for the other):
// each half is first reset to a clean slate (delete the record a previous
// approval created, or revert an edited record to its pre-request value),
// then — only on approve — (re-)applied.
func (s *services) decideCheckRequest(ctx context.Context, workspaceAlias, callerID, requestID string, status AttendanceCorrectionStatus) (*AttendanceCorrectionRequestResponse, error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetCheckRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := requireSameWorkspace(workspaceAlias, existing.WorkspaceID); err != nil {
		return nil, err
	}

	if err := s.resetCheckRequestHalf(ctx, existing.TargetCheckInRecordID, existing.ResultCheckInRecordID, existing.OriginalCheckInAt); err != nil {
		return nil, err
	}
	if err := s.resetCheckRequestHalf(ctx, existing.TargetCheckOutRecordID, existing.ResultCheckOutRecordID, existing.OriginalCheckOutAt); err != nil {
		return nil, err
	}

	var resultCheckInID, resultCheckOutID *string
	var recordEvents [][2]string // [type, id] pairs to broadcast

	if status == AttendanceCorrectionStatusApproved {
		if existing.CheckInAt != nil {
			id, eventType, err := s.applyCheckRequestHalf(ctx, workspaceAlias, existing.MemberID, *existing.CheckInAt, existing.TargetCheckInRecordID, AttendanceTypeCheckIn)
			if err != nil {
				return nil, err
			}
			resultCheckInID = &id
			recordEvents = append(recordEvents, [2]string{eventType, id})
		}
		if existing.CheckOutAt != nil {
			id, eventType, err := s.applyCheckRequestHalf(ctx, workspaceAlias, existing.MemberID, *existing.CheckOutAt, existing.TargetCheckOutRecordID, AttendanceTypeCheckOut)
			if err != nil {
				return nil, err
			}
			resultCheckOutID = &id
			recordEvents = append(recordEvents, [2]string{eventType, id})
		}
	} else {
		// Rejected: each half was already reset above (deleted if it was a
		// freshly-added record, or reverted to its original value if it was
		// an edit). For edit-halves, still reference the target record so
		// callers can see what this decision concerns.
		if existing.TargetCheckInRecordID != "" {
			id := existing.TargetCheckInRecordID
			resultCheckInID = &id
			recordEvents = append(recordEvents, [2]string{"attendance.updated", id})
		}
		if existing.TargetCheckOutRecordID != "" {
			id := existing.TargetCheckOutRecordID
			resultCheckOutID = &id
			recordEvents = append(recordEvents, [2]string{"attendance.updated", id})
		}
	}

	updated, err := s.repo.DecideCheckRequest(ctx, requestID, status, callerID, resultCheckInID, resultCheckOutID)
	if err != nil {
		return nil, err
	}

	s.broadcast(ctx, updated.WorkspaceID, "attendance_correction_request.updated", updated.ID, callerID)
	for _, event := range recordEvents {
		// Piggy-back the underlying record change under the vocabulary the
		// existing calendar/timeline UI already listens for, so it updates
		// with zero new client code.
		s.broadcast(ctx, updated.WorkspaceID, event[0], event[1], callerID)
	}

	if s.publisher != nil && updated.MemberID != callerID {
		eventType := NotificationTypeCheckRequestApproved
		if status == AttendanceCorrectionStatusRejected {
			eventType = NotificationTypeCheckRequestRejected
		}
		memberName, _ := s.repo.GetMemberName(ctx, updated.MemberID)
		if err := s.publisher.PublishCheckRequestDecision(ctx, CheckRequestNotificationEvent{
			Type: eventType, RecipientID: updated.MemberID, WorkspaceID: updated.WorkspaceID,
			WorkspaceAlias: workspaceAlias, RequestID: updated.ID, MemberID: updated.MemberID, MemberName: memberName,
			CheckInAt:  formatOptionalTime(updated.CheckInAt),
			CheckOutAt: formatOptionalTime(updated.CheckOutAt),
		}); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

// resetCheckRequestHalf undoes whatever a PREVIOUS decision did to one half
// (check-in or check-out) of a request, so the next decision always starts
// clean. targetRecordID set means this half edits an existing record —
// revert it to its pre-request value (originalAt, snapshotted at
// submission). Otherwise (a plain "add" half), delete the record a
// previous approval created, if any.
func (s *services) resetCheckRequestHalf(ctx context.Context, targetRecordID, resultRecordID string, originalAt *time.Time) error {
	if targetRecordID != "" {
		if originalAt == nil {
			return nil
		}
		_, err := s.repo.UpdateAttendanceRecord(ctx, targetRecordID, *originalAt)
		return err
	}
	if resultRecordID != "" {
		return s.repo.DeleteAttendanceRecord(ctx, resultRecordID)
	}
	return nil
}

// applyCheckRequestHalf realizes one half of an approved session request —
// either correcting the existing target record's time, or creating a new
// attendance_records row — and returns its id plus the realtime event type
// to broadcast for it.
func (s *services) applyCheckRequestHalf(ctx context.Context, workspaceAlias, memberID string, at time.Time, targetRecordID string, typ AttendanceType) (id, eventType string, err error) {
	if targetRecordID != "" {
		record, err := s.repo.UpdateAttendanceRecord(ctx, targetRecordID, at)
		if err != nil {
			return "", "", err
		}
		return record.ID, "attendance.updated", nil
	}
	record, err := s.repo.CreateAttendanceRecord(ctx, workspaceAlias, memberID, createAttendanceRecordParams{Type: typ, RecordedAt: at})
	if err != nil {
		return "", "", err
	}
	return record.ID, "attendance." + string(typ), nil
}

func (s *services) UpsertWorkSchedules(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkSchedulesRequest) ([]WorkScheduleDayResponse, error) {
	if err := normalizeWorkSchedulesRequest(&req); err != nil {
		return nil, err
	}
	// Self-registered schedules always (re)start at pending, even when
	// editing a previously-approved day — the holder must approve again.
	return s.upsertSchedulesFor(ctx, workspaceAlias, callerID, callerID, req.Schedules, WorkScheduleStatusPending, req.WeekNote)
}

// UpsertMemberWorkSchedules lets a workspace HOLDER or ADMIN set another
// member's registered work schedule, e.g. to correct a mistake or fill in a
// schedule on the member's behalf. The caller is already a reviewer, so
// these are auto-approved.
func (s *services) UpsertMemberWorkSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpsertMemberWorkSchedulesRequest) ([]WorkScheduleDayResponse, error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	if err := normalizeWorkSchedulesRequest((*UpsertWorkSchedulesRequest)(&req)); err != nil {
		return nil, err
	}
	return s.upsertSchedulesFor(ctx, workspaceAlias, callerID, memberID, req.Schedules, WorkScheduleStatusApproved, req.WeekNote)
}

func (s *services) upsertSchedulesFor(ctx context.Context, workspaceAlias, callerID, memberID string, schedules []UpsertWorkScheduleRequest, status WorkScheduleStatus, weekNote string) ([]WorkScheduleDayResponse, error) {
	var responses []WorkScheduleDayResponse
	var minDate, maxDate string

	for _, sch := range schedules {
		resp, err := s.repo.ReplaceWorkSchedule(ctx, workspaceAlias, memberID, callerID, sch, status, weekNote)
		if err != nil {
			return nil, err
		}
		responses = append(responses, *resp)

		if minDate == "" || sch.Date < minDate {
			minDate = sch.Date
		}
		if maxDate == "" || sch.Date > maxDate {
			maxDate = sch.Date
		}
	}

	if len(responses) > 0 {
		s.broadcast(ctx, responses[0].WorkspaceID, "work_schedule.updated", memberID, callerID)
	}

	// Only pending (self-registered) schedules need a holder's attention;
	// holder-created ones are already approved, so skip the notification.
	if s.publisher != nil && len(responses) > 0 && status == WorkScheduleStatusPending {
		recipients, err := s.repo.ListScheduleNotificationRecipients(ctx, workspaceAlias, memberID)
		if err != nil {
			return nil, err
		}
		memberName, _ := s.repo.GetMemberName(ctx, memberID)
		for _, recipientID := range recipients {
			if err := s.publisher.PublishWorkScheduleRegistration(ctx, ScheduleNotificationEvent{
				Type: NotificationTypeWorkScheduleRegistration, RecipientID: recipientID, WorkspaceID: responses[0].WorkspaceID,
				WorkspaceAlias: workspaceAlias, MemberID: memberID, MemberName: memberName, FromDate: minDate, ToDate: maxDate,
			}); err != nil {
				return nil, err
			}
		}
	}
	return responses, nil
}

func (s *services) ListMySchedules(ctx context.Context, workspaceAlias, callerID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error) {
	filter.MemberID = callerID
	return s.repo.ListWorkSchedules(ctx, workspaceAlias, callerID, filter, params)
}

// ListMemberSchedules lets a HOLDER or ADMIN review a specific member's
// schedule days across all statuses (pending/approved/rejected), e.g. to
// find days awaiting approval.
func (s *services) ListMemberSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return pagination.Response[WorkScheduleDayResponse]{}, err
	}
	filter.MemberID = memberID
	return s.repo.ListWorkSchedules(ctx, workspaceAlias, callerID, filter, params)
}

func (s *services) ListPendingSchedules(ctx context.Context, workspaceAlias, callerID string, filter WorkScheduleFilter, params pagination.Params) (pagination.Response[WorkScheduleDayResponse], error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return pagination.Response[WorkScheduleDayResponse]{}, err
	}
	// No MemberID set — the repo query treats that as "every member in
	// the workspace" (same nullable filter ListMemberSchedules narrows
	// with a specific ID).
	if filter.Status == "" {
		filter.Status = string(WorkScheduleStatusPending)
	}
	return s.repo.ListWorkSchedules(ctx, workspaceAlias, callerID, filter, params)
}

func (s *services) ApproveSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpdateWorkScheduleStatusRequest) ([]WorkScheduleDayResponse, error) {
	return s.updateScheduleStatus(ctx, workspaceAlias, callerID, memberID, req, WorkScheduleStatusApproved)
}

func (s *services) RejectSchedules(ctx context.Context, workspaceAlias, callerID, memberID string, req UpdateWorkScheduleStatusRequest) ([]WorkScheduleDayResponse, error) {
	return s.updateScheduleStatus(ctx, workspaceAlias, callerID, memberID, req, WorkScheduleStatusRejected)
}

func (s *services) updateScheduleStatus(ctx context.Context, workspaceAlias, callerID, memberID string, req UpdateWorkScheduleStatusRequest, status WorkScheduleStatus) ([]WorkScheduleDayResponse, error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	if err := normalizeUpdateWorkScheduleStatusRequest(&req); err != nil {
		return nil, err
	}
	responses, err := s.repo.UpdateWorkScheduleDayStatus(ctx, workspaceAlias, memberID, req.DayIDs, status)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return responses, nil
	}
	s.broadcast(ctx, responses[0].WorkspaceID, "work_schedule.updated", memberID, callerID)

	if s.publisher != nil && memberID != callerID {
		eventType := NotificationTypeWorkScheduleApproved
		if status == WorkScheduleStatusRejected {
			eventType = NotificationTypeWorkScheduleRejected
		}
		memberName, _ := s.repo.GetMemberName(ctx, memberID)
		if err := s.publisher.PublishWorkScheduleDecision(ctx, ScheduleNotificationEvent{
			Type: eventType, RecipientID: memberID, WorkspaceID: responses[0].WorkspaceID,
			WorkspaceAlias: workspaceAlias, MemberID: memberID, MemberName: memberName,
			FromDate: responses[0].Date, ToDate: responses[len(responses)-1].Date,
		}); err != nil {
			return nil, err
		}
	}
	return responses, nil
}

func (s *services) ApproveAllPendingSchedules(ctx context.Context, workspaceAlias, callerID, memberID string) ([]WorkScheduleDayResponse, error) {
	if err := s.requireHolderOrAdmin(ctx, workspaceAlias, callerID); err != nil {
		return nil, err
	}
	responses, err := s.repo.ApproveAllPendingSchedules(ctx, workspaceAlias, memberID)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return responses, nil
	}
	s.broadcast(ctx, responses[0].WorkspaceID, "work_schedule.updated", memberID, callerID)

	if s.publisher != nil && memberID != callerID {
		memberName, _ := s.repo.GetMemberName(ctx, memberID)
		if err := s.publisher.PublishWorkScheduleDecision(ctx, ScheduleNotificationEvent{
			Type: NotificationTypeWorkScheduleApproved, RecipientID: memberID, WorkspaceID: responses[0].WorkspaceID,
			WorkspaceAlias: workspaceAlias, MemberID: memberID, MemberName: memberName,
			FromDate: responses[0].Date, ToDate: responses[len(responses)-1].Date,
		}); err != nil {
			return nil, err
		}
	}
	return responses, nil
}

func (s *services) GetMonthlySummary(ctx context.Context, workspaceAlias, callerID string, filter AttendanceStatisticsFilter) (*AttendanceStatisticsSummaryResponse, error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if !roleCanViewAll(role) {
		filter.MemberID = callerID
	}

	from, to, period, label, err := resolvePeriod(filter)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.ListWorkspaceMembersForStatistics(ctx, workspaceAlias, filter.MemberID)
	if err != nil {
		return nil, err
	}
	schedules, err := s.repo.ListWorkspaceScheduleStats(ctx, workspaceAlias, from, to, filter.MemberID)
	if err != nil {
		return nil, err
	}
	// Pending/rejected days don't "apply" until a holder approves them, so
	// they're excluded from registered/attendance hour calculations.
	schedules = ApprovedSchedulesOnly(schedules)
	records, err := s.repo.ListWorkspaceAttendanceStats(ctx, workspaceAlias, from, to, filter.MemberID)
	if err != nil {
		return nil, err
	}
	initiativeEstimates, err := s.repo.SumInitiativeEstimates(ctx, workspaceAlias, filter.MemberID, from, to)
	if err != nil {
		return nil, err
	}
	lateLogEstimates, err := s.repo.SumLateLogEstimates(ctx, workspaceAlias, filter.MemberID, from, to)
	if err != nil {
		return nil, err
	}
	feedbackEstimates, err := s.repo.SumFeedbackEstimates(ctx, workspaceAlias, filter.MemberID, from, to)
	if err != nil {
		return nil, err
	}
	summary := &AttendanceStatisticsSummaryResponse{
		Period:  period,
		Label:   label,
		Month:   filter.Month,
		Members: make([]AttendanceStatisticsMemberResponse, 0, len(members)),
	}
	for _, member := range members {
		memberSummary := AttendanceStatisticsMemberResponse{
			MemberID: member.MemberID, MemberAlias: member.MemberAlias, MemberName: member.MemberName, MemberAvatar: member.MemberAvatar,
		}
		memberSchedules := schedulesForMember(schedules, member.MemberID)
		memberRecords := recordsForMember(records, member.MemberID)
		memberSummary.RegisteredHours = registeredHours(memberSchedules)
		memberSummary.AttendanceHours = attendanceHours(memberRecords, memberSchedules)
		memberSummary.LateDays, memberSummary.EarlyLeaveDays = lateEarlyDays(memberSchedules, memberRecords)
		memberSummary.EstimatedTaskHours, err = s.repo.SumAssignedTaskHours(ctx, workspaceAlias, member.MemberID, from, to)
		if err != nil {
			return nil, err
		}
		memberSummary.InitiativeEstimate = initiativeEstimates[member.MemberID]
		memberSummary.LateLogEstimate = lateLogEstimates[member.MemberID]
		memberSummary.FeedbackEstimate = feedbackEstimates[member.MemberID]
		summary.Totals.LateDays += memberSummary.LateDays
		summary.Totals.EarlyLeaveDays += memberSummary.EarlyLeaveDays
		summary.Totals.RegisteredHours += memberSummary.RegisteredHours
		summary.Totals.AttendanceHours += memberSummary.AttendanceHours
		summary.Totals.EstimatedTaskHours += memberSummary.EstimatedTaskHours
		summary.Totals.InitiativeEstimate += memberSummary.InitiativeEstimate
		summary.Totals.LateLogEstimate += memberSummary.LateLogEstimate
		summary.Totals.FeedbackEstimate += memberSummary.FeedbackEstimate
		summary.Members = append(summary.Members, memberSummary)
	}
	return summary, nil
}

func resolvePeriod(filter AttendanceStatisticsFilter) (from, to time.Time, period, label string, err error) {
	now := time.Now().In(vietnamLocation())
	week := strings.TrimSpace(filter.Week)
	year := strings.TrimSpace(filter.Year)
	month := strings.TrimSpace(filter.Month)

	switch {
	case week != "":
		from, to, err = parseWeekRange(week)
		if err != nil {
			return
		}
		_, isoWeek := from.ISOWeek()
		period = "week"
		label = fmt.Sprintf("%d-W%02d", from.Year(), isoWeek)
	case year != "":
		var y time.Time
		y, err = time.ParseInLocation("2006", year, vietnamLocation())
		if err != nil {
			err = fmt.Errorf("year phải theo định dạng YYYY")
			return
		}
		from = time.Date(y.Year(), 1, 1, 0, 0, 0, 0, vietnamLocation())
		to = time.Date(y.Year(), 12, 31, 23, 59, 59, 0, vietnamLocation())
		period = "year"
		label = year
	default:
		if month == "" {
			month = now.Format("2006-01")
		}
		var m time.Time
		m, err = parseMonth(month)
		if err != nil {
			err = fmt.Errorf("month phải theo định dạng YYYY-MM")
			return
		}
		from = m
		to = m.AddDate(0, 1, 0).Add(-time.Second)
		period = "month"
		label = month
	}
	return
}

func parseWeekRange(value string) (from, to time.Time, err error) {
	loc := vietnamLocation()
	var isoYear, isoWeek int
	if n, _ := fmt.Sscanf(value, "%d-W%d", &isoYear, &isoWeek); n == 2 {
		// Monday of ISO week: Jan 4 is always in week 1
		jan4 := time.Date(isoYear, 1, 4, 0, 0, 0, 0, loc)
		_, jan4Week := jan4.ISOWeek()
		monday := jan4.AddDate(0, 0, (isoWeek-jan4Week)*7)
		// Adjust to Monday
		wd := int(monday.Weekday())
		if wd == 0 {
			wd = 7
		}
		from = monday.AddDate(0, 0, -(wd - 1))
		to = from.AddDate(0, 0, 6).Add(24*time.Hour - time.Second)
		return
	}
	// Fallback: parse as YYYY-MM-DD and find Monday of that week
	var date time.Time
	date, err = time.ParseInLocation(time.DateOnly, value, loc)
	if err != nil {
		err = fmt.Errorf("week phải theo định dạng YYYY-Www hoặc YYYY-MM-DD")
		return
	}
	wd := int(date.Weekday())
	if wd == 0 {
		wd = 7
	}
	from = date.AddDate(0, 0, -(wd - 1))
	to = from.AddDate(0, 0, 6).Add(24*time.Hour - time.Second)
	return
}

func (s *services) attendanceFilterForRole(ctx context.Context, workspaceAlias, callerID string, filter AttendanceRecordFilter) (AttendanceRecordFilter, error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return filter, err
	}
	if role == "" {
		return filter, ErrForbidden
	}
	if !roleCanViewAll(role) {
		filter.MemberID = callerID
	}
	return filter, nil
}

func roleCanViewAll(role workspace.WorkspaceRole) bool {
	return role == workspace.WorkspaceRoleHolder || role == workspace.WorkspaceRoleAdmin
}

func (s *services) GetTaskStatistics(ctx context.Context, workspaceAlias, callerID string, filter TaskStatisticsFilter) (*TaskStatisticsSummaryResponse, error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbidden
	}
	if filter.ProjectID != "" {
		// The per-project Statistics screen is admin/holder-only — checked
		// against the caller's role *within that project* (or workspace
		// role, CheckProjectManageAuth covers both), not the workspace-wide
		// roleCanViewAll below.
		canManage, err := s.repo.CheckProjectManageAuth(ctx, filter.ProjectID, callerID)
		if err != nil {
			return nil, err
		}
		if !canManage {
			return nil, ErrForbidden
		}
	} else if !roleCanViewAll(role) {
		// The workspace-wide Statistics screen (and its per-member KPI
		// drill-down, which reuses this same call unfiltered) is also
		// admin/holder-only — both of its only two Flutter call sites
		// (WorkspaceStatsScreen, KpiScreen) are already gated behind
		// canManageWorkspace, so there's no legitimate self-view caller to
		// preserve here. No self-only fallback: a regular member gets
		// nothing from this endpoint, not just their own stats — an
		// individual's own numbers are served by GetMyTaskStatistics
		// (/task/statistics/me) instead.
		return nil, ErrForbidden
	}

	var from, to time.Time
	var period, label string
	if filter.Week != "" || filter.Year != "" || filter.Month != "" {
		statsFilter := AttendanceStatisticsFilter{Month: filter.Month, Week: filter.Week, Year: filter.Year}
		from, to, period, label, err = resolvePeriod(statsFilter)
		if err != nil {
			return nil, err
		}
	}

	rows, err := s.repo.GetTaskStatsByWorkspace(ctx, workspaceAlias, filter, from, to)
	if err != nil {
		return nil, err
	}

	summary := &TaskStatisticsSummaryResponse{
		Period:    period,
		Label:     label,
		ProjectID: filter.ProjectID,
		ModuleID:  filter.ModuleID,
		Members:   rows,
	}
	for _, m := range rows {
		summary.Totals.TotalTasks += m.TotalTasks
		summary.Totals.Backlog += m.Backlog
		summary.Totals.Todo += m.Todo
		summary.Totals.InProgress += m.InProgress
		summary.Totals.Review += m.Review
		summary.Totals.Done += m.Done
		summary.Totals.Canceled += m.Canceled
		summary.Totals.TotalEstimation += m.TotalEstimation
		summary.Totals.TotalOverdue += m.TotalOverdue
		summary.Totals.TotalEstimatedTaskHours += m.TotalEstimatedTaskHours
		summary.Totals.Priority1 += m.Priority1
		summary.Totals.Priority2 += m.Priority2
		summary.Totals.Priority3 += m.Priority3
		summary.Totals.Priority4 += m.Priority4
		summary.Totals.TotalLate += m.TotalLate
		summary.Totals.TotalBug += m.TotalBug
	}
	avgCycleDays, cycleSampleSize, err := s.repo.GetTaskCycleTimeByWorkspace(ctx, workspaceAlias, filter, from, to)
	if err != nil {
		return nil, err
	}
	summary.Totals.AvgCycleDays = avgCycleDays
	summary.Totals.CycleTimeSampleSize = cycleSampleSize
	return summary, nil
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMeters = 6371000
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadiusMeters * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func registeredHours(schedules []WorkScheduleDayResponse) float64 {
	var total time.Duration
	for _, schedule := range schedules {
		for _, session := range schedule.Sessions {
			total += session.EndAt.Sub(session.StartAt)
		}
	}
	return total.Hours()
}

type timeInterval struct{ start, end time.Time }

// pairPresenceIntervals ghép record check-in/out theo FIFO thành các khoảng
// [vào, ra] hoàn chỉnh (giống getDaySessions bên event), bỏ qua nửa phiên hở vì
// không đo được thời lượng. Các khoảng có thể trùng/chồng nhau — bước union sau
// đó sẽ gộp lại nên phần giờ chồng không bị cộng hai lần.
func pairPresenceIntervals(records []AttendanceRecordResponse) []timeInterval {
	sorted := make([]AttendanceRecordResponse, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RecordedAt.Before(sorted[j].RecordedAt) })

	var intervals []timeInterval
	var openIns []time.Time
	for _, r := range sorted {
		switch r.Type {
		case AttendanceTypeCheckIn:
			openIns = append(openIns, r.RecordedAt)
		case AttendanceTypeCheckOut:
			if len(openIns) > 0 {
				in := openIns[0]
				openIns = openIns[1:]
				if r.RecordedAt.After(in) {
					intervals = append(intervals, timeInterval{in, r.RecordedAt})
				}
			}
		}
	}
	return intervals
}

// unionIntervals gộp các khoảng chồng nhau thành tập khoảng rời, đã sắp xếp.
func unionIntervals(in []timeInterval) []timeInterval {
	if len(in) == 0 {
		return nil
	}
	sorted := make([]timeInterval, len(in))
	copy(sorted, in)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start.Before(sorted[j].start) })

	out := []timeInterval{sorted[0]}
	for _, iv := range sorted[1:] {
		last := &out[len(out)-1]
		if !iv.start.After(last.end) { // chồng hoặc liền kề → gộp
			if iv.end.After(last.end) {
				last.end = iv.end
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

func intervalOverlap(a, b timeInterval) time.Duration {
	start := a.start
	if b.start.After(start) {
		start = b.start
	}
	end := a.end
	if b.end.Before(end) {
		end = b.end
	}
	if end.After(start) {
		return end.Sub(start)
	}
	return 0
}

// attendanceHours = tổng phần thời gian CÓ MẶT (hợp các phiên [vào,ra]) nằm
// TRONG các ca đã đăng ký. Dùng union để nhiều phiên trùng/chồng giờ trong một
// ngày không bị cộng trùng; các ca đã duyệt vốn không chồng nhau nên cộng theo
// từng ca là đủ.
func attendanceHours(records []AttendanceRecordResponse, schedules []WorkScheduleDayResponse) float64 {
	presence := unionIntervals(pairPresenceIntervals(records))
	var total time.Duration
	for _, schedule := range schedules {
		for _, session := range schedule.Sessions {
			window := timeInterval{session.StartAt, session.EndAt}
			for _, p := range presence {
				total += intervalOverlap(p, window)
			}
		}
	}
	return total.Hours()
}

func lateEarlyDays(schedules []WorkScheduleDayResponse, records []AttendanceRecordResponse) (int, int) {
	recordsByDate := map[string][]AttendanceRecordResponse{}
	for _, record := range records {
		date := record.RecordedAt.In(vietnamLocation()).Format(time.DateOnly)
		recordsByDate[date] = append(recordsByDate[date], record)
	}
	var lateDays, earlyDays int
	for _, schedule := range schedules {
		if len(schedule.Sessions) == 0 {
			continue
		}
		dayRecords := recordsByDate[schedule.Date]
		if len(dayRecords) == 0 {
			continue
		}
		firstSessionStart := schedule.Sessions[0].StartAt
		lastSessionEnd := schedule.Sessions[0].EndAt
		for _, session := range schedule.Sessions {
			if session.StartAt.Before(firstSessionStart) {
				firstSessionStart = session.StartAt
			}
			if session.EndAt.After(lastSessionEnd) {
				lastSessionEnd = session.EndAt
			}
		}
		var firstIn *time.Time
		var lastOut *time.Time
		for i := range dayRecords {
			record := dayRecords[i]
			if record.Type == AttendanceTypeCheckIn && (firstIn == nil || record.RecordedAt.Before(*firstIn)) {
				t := record.RecordedAt
				firstIn = &t
			}
			if record.Type == AttendanceTypeCheckOut && (lastOut == nil || record.RecordedAt.After(*lastOut)) {
				t := record.RecordedAt
				lastOut = &t
			}
		}
		// Du di 5 phút: vào trễ ≤5p hoặc về sớm ≤5p so với ca không tính muộn/sớm.
		if firstIn != nil && firstIn.After(firstSessionStart.Add(attendanceGracePeriod)) {
			lateDays++
		}
		if lastOut != nil && lastOut.Before(lastSessionEnd.Add(-attendanceGracePeriod)) {
			earlyDays++
		}
	}
	return lateDays, earlyDays
}

// ApprovedSchedulesOnly filters schedules down to those with an approved
// status. Exported so other features (e.g. performance's KPI computation)
// can apply the same "only approved schedules count" rule this package uses
// for its own monthly summary.
func ApprovedSchedulesOnly(schedules []WorkScheduleDayResponse) []WorkScheduleDayResponse {
	out := make([]WorkScheduleDayResponse, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule.Status == WorkScheduleStatusApproved {
			out = append(out, schedule)
		}
	}
	return out
}

func schedulesForMember(schedules []WorkScheduleDayResponse, memberID string) []WorkScheduleDayResponse {
	var out []WorkScheduleDayResponse
	for _, schedule := range schedules {
		if schedule.MemberID == memberID {
			out = append(out, schedule)
		}
	}
	return out
}

func recordsForMember(records []AttendanceRecordResponse, memberID string) []AttendanceRecordResponse {
	var out []AttendanceRecordResponse
	for _, record := range records {
		if record.MemberID == memberID {
			out = append(out, record)
		}
	}
	return out
}

func (s *services) GetMyAttendanceSummary(ctx context.Context, workspaceAlias, callerID string, filter AttendanceStatisticsFilter) (*MyAttendanceStatisticsResponse, error) {
	filter.MemberID = callerID
	summary, err := s.GetMonthlySummary(ctx, workspaceAlias, callerID, filter)
	if err != nil {
		return nil, err
	}
	if len(summary.Members) == 0 {
		return &MyAttendanceStatisticsResponse{
			WorkspaceID: summary.WorkspaceID,
			Period:      summary.Period,
			Label:       summary.Label,
			Month:       summary.Month,
		}, nil
	}
	m := summary.Members[0]
	return &MyAttendanceStatisticsResponse{
		WorkspaceID:        summary.WorkspaceID,
		Period:             summary.Period,
		Label:              summary.Label,
		Month:              summary.Month,
		MemberID:           m.MemberID,
		MemberAlias:        m.MemberAlias,
		MemberName:         m.MemberName,
		MemberAvatar:       m.MemberAvatar,
		LateDays:           m.LateDays,
		EarlyLeaveDays:     m.EarlyLeaveDays,
		RegisteredHours:    m.RegisteredHours,
		AttendanceHours:    m.AttendanceHours,
		EstimatedTaskHours: m.EstimatedTaskHours,
		InitiativeEstimate: m.InitiativeEstimate,
		LateLogEstimate:    m.LateLogEstimate,
		FeedbackEstimate:   m.FeedbackEstimate,
	}, nil
}

func (s *services) GetMyTaskStatistics(ctx context.Context, workspaceAlias, callerID string, filter TaskStatisticsFilter) (*MyTaskStatisticsResponse, error) {
	filter.MemberID = callerID
	summary, err := s.GetTaskStatistics(ctx, workspaceAlias, callerID, filter)
	if err != nil {
		return nil, err
	}
	if len(summary.Members) == 0 {
		return &MyTaskStatisticsResponse{
			Period:    summary.Period,
			Label:     summary.Label,
			ProjectID: summary.ProjectID,
			ModuleID:  summary.ModuleID,
		}, nil
	}
	m := summary.Members[0]
	return &MyTaskStatisticsResponse{
		Period:                  summary.Period,
		Label:                   summary.Label,
		ProjectID:               summary.ProjectID,
		ModuleID:                summary.ModuleID,
		MemberID:                m.MemberID,
		MemberAlias:             m.MemberAlias,
		MemberName:              m.MemberName,
		MemberAvatar:            m.MemberAvatar,
		TotalTasks:              m.TotalTasks,
		Backlog:                 m.Backlog,
		Todo:                    m.Todo,
		InProgress:              m.InProgress,
		Review:                  m.Review,
		Done:                    m.Done,
		Canceled:                m.Canceled,
		Priority1:               m.Priority1,
		Priority2:               m.Priority2,
		Priority3:               m.Priority3,
		Priority4:               m.Priority4,
		TotalEstimation:         m.TotalEstimation,
		TotalOverdue:            m.TotalOverdue,
		TotalEstimatedTaskHours: m.TotalEstimatedTaskHours,
		TotalLate:               m.TotalLate,
		TotalBug:                m.TotalBug,
	}, nil
}

func (s *services) GetWorkWeek(ctx context.Context, workspaceAlias, callerID string) (*WorkWeekResponse, error) {
	if _, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID); err != nil {
		return nil, ErrForbidden
	}
	return s.repo.GetWorkWeek(ctx, workspaceAlias)
}

func (s *services) UpsertMyDailyReport(ctx context.Context, workspaceAlias, callerID string, req UpsertDailyReportRequest) (*DailyReportResponse, error) {
	if _, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID); err != nil {
		return nil, ErrForbidden
	}
	req.Done = strings.TrimSpace(req.Done)
	req.InProgress = strings.TrimSpace(req.InProgress)
	req.Issues = strings.TrimSpace(req.Issues)
	if _, err := parseDate(req.Date); err != nil {
		return nil, fmt.Errorf("%w: ngày không hợp lệ", ErrInvalidDailyReport)
	}
	if req.Done == "" && req.InProgress == "" && req.Issues == "" {
		return nil, fmt.Errorf("%w: cần điền ít nhất một mục", ErrInvalidDailyReport)
	}
	return s.repo.UpsertDailyReport(ctx, workspaceAlias, callerID, req)
}

func (s *services) ListMyDailyReports(ctx context.Context, workspaceAlias, callerID string, filter DailyReportFilter, params pagination.Params) (pagination.Response[DailyReportResponse], error) {
	var empty pagination.Response[DailyReportResponse]
	if _, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID); err != nil {
		return empty, ErrForbidden
	}
	filter.MemberID = callerID
	return s.repo.ListDailyReports(ctx, workspaceAlias, filter, params)
}

func (s *services) ListDailyReports(ctx context.Context, workspaceAlias, callerID string, filter DailyReportFilter, params pagination.Params) (pagination.Response[DailyReportResponse], error) {
	var empty pagination.Response[DailyReportResponse]
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return empty, ErrForbidden
	}
	if !role.CanManageMembers() {
		return empty, ErrForbidden
	}
	return s.repo.ListDailyReports(ctx, workspaceAlias, filter, params)
}

func (s *services) GetMyWorkWeek(ctx context.Context, workspaceAlias, callerID string) (*WorkWeekResponse, error) {
	if _, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID); err != nil {
		return nil, ErrForbidden
	}
	return s.repo.GetMemberWorkWeek(ctx, workspaceAlias, callerID)
}

func (s *services) UpsertMyWorkWeek(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error) {
	if _, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID); err != nil {
		return nil, ErrForbidden
	}
	if err := normalizeWorkWeekRequest(&req); err != nil {
		return nil, err
	}
	return s.repo.ReplaceMemberWorkWeek(ctx, workspaceAlias, callerID, req)
}

func (s *services) UpsertWorkWeek(ctx context.Context, workspaceAlias, callerID string, req UpsertWorkWeekRequest) (*WorkWeekResponse, error) {
	role, err := s.repo.GetCallerRole(ctx, workspaceAlias, callerID)
	if err != nil {
		return nil, ErrForbidden
	}
	if !role.CanManageMembers() {
		return nil, ErrForbidden
	}
	if err := normalizeWorkWeekRequest(&req); err != nil {
		return nil, err
	}
	week, err := s.repo.ReplaceWorkWeek(ctx, workspaceAlias, req)
	if err != nil {
		return nil, err
	}
	if week != nil {
		s.broadcast(ctx, week.WorkspaceID, "work_week.updated", week.WorkspaceID, callerID)
	}
	return week, nil
}
