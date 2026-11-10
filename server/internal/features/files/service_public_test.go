package files

import (
	"testing"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
)

func uid(n string) pgtype.UUID { return pgutil.MustUUID("00000000-0000-0000-0000-00000000000" + n) }

func folder(id, parent string, public bool) database.FileFolder {
	f := database.FileFolder{ID: uid(id), IsPublic: public, Kind: FolderKindCustom}
	if parent != "" {
		f.ParentID = uid(parent)
	}
	return f
}

// "Folder private che hết": một folder/node chỉ effective-public khi CHÍNH NÓ và
// toàn bộ chuỗi cha đều public.
func TestEffectivePublicMap(t *testing.T) {
	folders := []database.FileFolder{
		folder("1", "", true),  // P: root public
		folder("2", "1", true), // A: public con của P  -> public
		folder("3", "1", false),// B: private con của P -> private
		folder("4", "3", true), // C: public con của B  -> BỊ CHE (cha private)
		folder("5", "", false), // Q: root private
		folder("6", "5", true), // D: public con của Q  -> BỊ CHE
	}
	eff := effectivePublicMap(folders)

	want := map[string]bool{
		pgutil.UUIDToString(uid("1")): true,
		pgutil.UUIDToString(uid("2")): true,
		pgutil.UUIDToString(uid("3")): false,
		pgutil.UUIDToString(uid("4")): false,
		pgutil.UUIDToString(uid("5")): false,
		pgutil.UUIDToString(uid("6")): false,
	}
	for id, w := range want {
		if eff[id] != w {
			t.Errorf("folder %s: effective-public = %v, muốn %v", id, eff[id], w)
		}
	}
}
