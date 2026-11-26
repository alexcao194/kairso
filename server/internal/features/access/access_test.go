package access

import (
	"slices"
	"testing"
)

func TestDefaultsKeepTheOldBehaviour(t *testing.T) {
	m := ParseMatrix(nil)
	holder, admin, member := NewGrant(RoleHolder, m), NewGrant(RoleAdmin, m), NewGrant(RoleMember, m)
	for _, p := range All {
		if !holder.Can(p) {
			t.Errorf("holder lacks %s", p)
		}
		if member.Can(p) {
			t.Errorf("member has %s by default", p)
		}
		if want := p != KPISettings; admin.Can(p) != want {
			t.Errorf("admin %s = %v, want %v", p, admin.Can(p), want)
		}
	}
	if (Grant{}).Can(QuotesManage) || (Grant{}).IsMember() {
		t.Error("a non-member must have nothing")
	}
}

func TestMatrixOverridesOneRoleOnly(t *testing.T) {
	m := ParseMatrix([]byte(`{"member":["quotes.manage","bogus"],"holder":[]}`))
	member := NewGrant(RoleMember, m)
	if !member.Can(QuotesManage) || member.Can(FilesManage) {
		t.Fatalf("member perms = %v", member.Permissions())
	}
	if got := m.For(RoleAdmin); !slices.Equal(got, Defaults(RoleAdmin)) {
		t.Fatalf("admin must keep defaults, got %v", got)
	}
	if !NewGrant(RoleHolder, m).Can(KPISettings) {
		t.Fatal("the matrix must not limit the holder")
	}
	if _, ok := m["holder"]; ok {
		t.Fatal("holder entries must be ignored")
	}
}

func TestEmptyListRemovesEverything(t *testing.T) {
	m := ParseMatrix([]byte(`{"admin":[]}`))
	if len(NewGrant(RoleAdmin, m).Permissions()) != 0 {
		t.Fatal("an empty list means no permissions, not defaults")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(Matrix{RoleAdmin: {FilesManage}}); err != nil {
		t.Fatal(err)
	}
	if Validate(Matrix{RoleHolder: {}}) == nil {
		t.Error("holder is not editable")
	}
	if Validate(Matrix{RoleMember: {"nope"}}) == nil {
		t.Error("unknown permission accepted")
	}
}

func TestNormalizeOrdersAndFillsRoles(t *testing.T) {
	m := Normalize(Matrix{RoleMember: {FilesManage, QuotesManage, FilesManage}})
	if !slices.Equal(m[RoleMember], []Permission{QuotesManage, FilesManage}) {
		t.Fatalf("member = %v", m[RoleMember])
	}
	if !slices.Equal(m[RoleAdmin], Defaults(RoleAdmin)) {
		t.Fatalf("admin = %v", m[RoleAdmin])
	}
}
