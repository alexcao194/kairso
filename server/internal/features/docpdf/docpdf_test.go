package docpdf

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	o := ParseOptions(httptest.NewRequest("GET", "/x?template=admin&sign=wet&place=+H%C3%A0+N%E1%BB%99i+&signer=gi%C3%A1m+%C4%91%E1%BB%91c&merge=1", nil))
	if !o.Admin || !o.Wet || o.Place != "Hà Nội" || o.Signer != "GIÁM ĐỐC" || o.Merge != "1" {
		t.Fatalf("%+v", o)
	}
	if o := ParseOptions(httptest.NewRequest("GET", "/x?merge=2", nil)); o.Admin || o.Merge != "" {
		t.Fatalf("%+v", o)
	}
}

func TestDateLine(t *testing.T) {
	if got := DateLine(PlaceFromAddress("88 Láng Hạ, Đống Đa, Hà Nội"), time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)); got != "Hà Nội, ngày 05 tháng 03 năm 2026" {
		t.Fatal(got)
	}
	if got := DateLine("", time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)); got != "……………, ngày 05 tháng 03 năm 2026" {
		t.Fatal(got)
	}
}

// A font folder wins over the system fonts, and missing styles fall back to the regular face.
func TestLoadFontsFromDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Times.TTF"), []byte("regular"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := LoadFonts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Family != "Times New Roman" || f.Source != dir || string(f.faces["B"]) != "regular" {
		t.Fatalf("family %s from %s", f.Family, f.Source)
	}
}

func TestParseHex(t *testing.T) {
	if c := ParseHex("#4F46E5", DefaultBrand); c != (RGB{0x4F, 0x46, 0xE5}) {
		t.Fatal(c)
	}
	if c := ParseHex("", DefaultBrand); c != DefaultBrand {
		t.Fatal(c)
	}
}
