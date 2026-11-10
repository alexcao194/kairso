package workspace

import "testing"

// TestCanManageRole xác nhận rule: chỉ được sửa/gán vai trò cho người có cấp thấp hơn
// hoặc bằng mình (Chủ workspace > Quản trị viên > Thành viên).
func TestCanManageRole(t *testing.T) {
	cases := []struct {
		caller, target WorkspaceRole
		want           bool
	}{
		{WorkspaceRoleHolder, WorkspaceRoleHolder, true},
		{WorkspaceRoleHolder, WorkspaceRoleAdmin, true},
		{WorkspaceRoleHolder, WorkspaceRoleMember, true},
		{WorkspaceRoleAdmin, WorkspaceRoleAdmin, true},
		{WorkspaceRoleAdmin, WorkspaceRoleMember, true},
		{WorkspaceRoleAdmin, WorkspaceRoleHolder, false},
		{WorkspaceRoleMember, WorkspaceRoleMember, true},
		{WorkspaceRoleMember, WorkspaceRoleAdmin, false},
		{WorkspaceRoleMember, WorkspaceRoleHolder, false},
	}
	for _, c := range cases {
		if got := c.caller.CanManageRole(c.target); got != c.want {
			t.Errorf("%s.CanManageRole(%s) = %v, want %v", c.caller, c.target, got, c.want)
		}
	}
}
