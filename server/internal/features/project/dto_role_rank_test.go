package project

import "testing"

// TestCanManageRole xác nhận rule: chỉ được sửa/gán vai trò cho người có cấp thấp hơn
// hoặc bằng mình (Chủ workspace > Quản trị viên > Thành viên).
func TestCanManageRole(t *testing.T) {
	cases := []struct {
		caller, target ProjectRole
		want           bool
	}{
		{ProjectRoleHolder, ProjectRoleHolder, true},
		{ProjectRoleHolder, ProjectRoleAdmin, true},
		{ProjectRoleHolder, ProjectRoleMember, true},
		{ProjectRoleAdmin, ProjectRoleAdmin, true},
		{ProjectRoleAdmin, ProjectRoleMember, true},
		{ProjectRoleAdmin, ProjectRoleHolder, false},
		{ProjectRoleMember, ProjectRoleMember, true},
		{ProjectRoleMember, ProjectRoleAdmin, false},
		{ProjectRoleMember, ProjectRoleHolder, false},
	}
	for _, c := range cases {
		if got := c.caller.CanManageRole(c.target); got != c.want {
			t.Errorf("%s.CanManageRole(%s) = %v, want %v", c.caller, c.target, got, c.want)
		}
	}
}
