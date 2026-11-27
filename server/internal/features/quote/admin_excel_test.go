package quote

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/xuri/excelize/v2"
)

// TestAdminQuoteExcel builds the administrative workbook. Set PDF_OUT to a folder to keep it.
func TestAdminQuoteExcel(t *testing.T) {
	h := &Handler{pdf: docpdf.New(os.Getenv("PDF_FONT_DIR"), nil)}
	q := sampleQuote()
	for name, o := range map[string]docpdf.Options{
		"quote-admin.xlsx":        {Admin: true, Merge: "0"},
		"quote-admin-single.xlsx": {Admin: true, Merge: "1", Place: "Hà Nội"},
		"quote-admin-wet.xlsx":    {Admin: true, Merge: "1", Wet: true},
	} {
		file, err := h.adminQuoteExcel(context.Background(), "", q, o)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		f, err := excelize.OpenReader(bytes.NewReader(file.Data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rows, err := f.GetRows(f.GetSheetName(0))
		if err != nil {
			t.Fatal(err)
		}
		var all strings.Builder
		for _, r := range rows {
			all.WriteString(strings.Join(r, "|") + "\n")
		}
		for _, want := range []string{"CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", "Số: BG-2026-0042", "BẢNG BÁO GIÁ", "TỔNG CỘNG", "Nơi nhận:"} {
			if !strings.Contains(all.String(), want) {
				t.Errorf("%s: missing %q", name, want)
			}
		}
		if dir := os.Getenv("PDF_OUT"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, name), file.Data, 0o644)
		}
	}
}

func TestMoneyNumFmt(t *testing.T) {
	if got := moneyNumFmt("VND", 0, "vi"); got != `#,##0" ₫";"− "#,##0" ₫"` {
		t.Errorf("VND: %s", got)
	}
	if got := moneyNumFmt("USD", 2, "en"); got != `"$"#,##0.00;"− ""$"#,##0.00` {
		t.Errorf("USD: %s", got)
	}
}
