// Package access decides what a workspace member may do. Each workspace keeps a permission
// matrix for its built-in roles: the holder always has every permission, while the workspace's
// holder chooses which permissions admins and members get. A role the matrix does not mention
// keeps the default permissions (the behaviour from before the matrix existed).
package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Permission is one thing a workspace member may be allowed to do.
type Permission string

const (
	// WorkspaceSettings: rename the workspace, change its logo and brand colour.
	WorkspaceSettings Permission = "workspace.settings"
	// MembersManage: invite, remove and change the role of workspace members (never above their
	// own role), and manage project members.
	MembersManage Permission = "members.manage"
	// AttendanceManage: attendance settings and sessions, delete records, decide correction
	// requests, edit and approve members' work schedules, the workspace work week.
	AttendanceManage Permission = "attendance.manage"
	// AttendanceViewAll: see every member's attendance, calendar, statistics and daily reports.
	AttendanceViewAll Permission = "attendance.view_all"
	// QuotesManage: create, edit, send and change quotes, customers and quote settings; remove
	// other people's signatures on quotes.
	QuotesManage Permission = "quotes.manage"
	// MeetingsManage: delete other people's minutes, sign minutes as the controller, remove other
	// people's signatures on minutes.
	MeetingsManage Permission = "meetings.manage"
	// FilesManage: manage the workspace's shared files and folders.
	FilesManage Permission = "files.manage"
	// WebhooksManage: manage outgoing webhooks.
	WebhooksManage Permission = "webhooks.manage"
	// PerformanceManage: accept initiatives, rate attitude, see and export members' KPI.
	PerformanceManage Permission = "performance.manage"
	// KPISettings: see and change how KPI is computed.
	KPISettings Permission = "kpi.settings"
)

// Roles of a workspace. They mirror workspace.WorkspaceRole, which imports this package.
const (
	RoleHolder = "holder"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// All lists every permission in display order.
var All = []Permission{
	WorkspaceSettings, MembersManage, AttendanceManage, AttendanceViewAll, QuotesManage,
	MeetingsManage, FilesManage, WebhooksManage, PerformanceManage, KPISettings,
}

// EditableRoles are the roles whose permissions the matrix sets.
var EditableRoles = []string{RoleAdmin, RoleMember}

// Defaults are the permissions of a role the matrix does not mention.
func Defaults(role string) []Permission {
	switch role {
	case RoleHolder:
		return slices.Clone(All)
	case RoleAdmin:
		return slices.DeleteFunc(slices.Clone(All), func(p Permission) bool { return p == KPISettings })
	default:
		return []Permission{}
	}
}

// Valid reports whether p is a known permission.
func (p Permission) Valid() bool { return slices.Contains(All, p) }

// Matrix maps an editable role to its permissions.
type Matrix map[string][]Permission

// ParseMatrix reads the stored JSON matrix; unknown roles and permissions are dropped.
func ParseMatrix(raw []byte) Matrix {
	m := Matrix{}
	if len(raw) == 0 {
		return m
	}
	var stored map[string][]Permission
	if err := json.Unmarshal(raw, &stored); err != nil {
		return m
	}
	for _, role := range EditableRoles {
		if perms, ok := stored[role]; ok {
			m[role] = normalize(perms)
		}
	}
	return m
}

// For returns the permissions of role under this matrix.
func (m Matrix) For(role string) []Permission {
	if role == RoleHolder {
		return Defaults(RoleHolder)
	}
	if perms, ok := m[role]; ok {
		return slices.Clone(perms)
	}
	return Defaults(role)
}

// Resolved returns the full matrix, defaults filled in, for every editable role.
func (m Matrix) Resolved() Matrix {
	out := Matrix{}
	for _, role := range EditableRoles {
		out[role] = m.For(role)
	}
	return out
}

// Validate checks a matrix sent by a client: only editable roles and known permissions.
func Validate(m Matrix) error {
	for role, perms := range m {
		if !slices.Contains(EditableRoles, role) {
			return fmt.Errorf("vai trò %q không chỉnh được quyền", role)
		}
		for _, p := range perms {
			if !p.Valid() {
				return fmt.Errorf("quyền %q không tồn tại", p)
			}
		}
	}
	return nil
}

// normalize removes unknown and duplicate permissions and sorts them in display order.
func normalize(perms []Permission) []Permission {
	out := []Permission{}
	for _, p := range All {
		if slices.Contains(perms, p) {
			out = append(out, p)
		}
	}
	return out
}

// Normalize returns m with unknown permissions dropped and every editable role present.
func Normalize(m Matrix) Matrix {
	out := Matrix{}
	for _, role := range EditableRoles {
		if perms, ok := m[role]; ok {
			out[role] = normalize(perms)
		} else {
			out[role] = Defaults(role)
		}
	}
	return out
}

// Grant is what one user may do in one workspace. The zero Grant is a non-member.
type Grant struct {
	Role  string
	perms []Permission
}

// IsMember reports whether the user belongs to the workspace.
func (g Grant) IsMember() bool { return g.Role != "" }

// IsHolder reports whether the user owns the workspace.
func (g Grant) IsHolder() bool { return g.Role == RoleHolder }

// Can reports whether the user has permission p.
func (g Grant) Can(p Permission) bool { return g.IsMember() && slices.Contains(g.perms, p) }

// Permissions lists the user's permissions.
func (g Grant) Permissions() []Permission { return slices.Clone(g.perms) }

// NewGrant builds a grant for role under matrix m (tests, callers that already hold both).
func NewGrant(role string, m Matrix) Grant {
	if role == "" {
		return Grant{}
	}
	return Grant{Role: role, perms: m.For(role)}
}

// Load reads userID's role and the workspace's matrix. A non-member gets the zero Grant and no
// error; malformed IDs are errors.
func Load(ctx context.Context, q *database.Queries, workspaceID, userID string) (Grant, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return Grant{}, err
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return Grant{}, err
	}
	return LoadUUID(ctx, q, wid, uid)
}

// LoadUUID is Load for callers that already parsed the IDs.
func LoadUUID(ctx context.Context, q *database.Queries, workspaceID, userID pgtype.UUID) (Grant, error) {
	row, err := q.GetWorkspaceAccess(ctx, database.GetWorkspaceAccessParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, nil
	}
	if err != nil {
		return Grant{}, err
	}
	return NewGrant(string(row.Role), ParseMatrix(row.RolePermissions)), nil
}

// UsersWith lists the members of the workspace who have permission p, except exceptUserID
// (pass "" to keep everyone). Used to pick who gets a notification.
func UsersWith(ctx context.Context, q *database.Queries, workspaceID string, p Permission, exceptUserID string) ([]string, error) {
	wid, err := pgutil.ParseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	raw, err := q.GetWorkspaceRolePermissions(ctx, wid)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := ParseMatrix(raw)
	rows, err := q.ListWorkspaceMemberRoles(ctx, wid)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, row := range rows {
		id := pgutil.UUIDToString(row.UserID)
		if id != exceptUserID && NewGrant(string(row.Role), m).Can(p) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
