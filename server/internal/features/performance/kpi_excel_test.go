package performance

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// Dựng workbook một thành viên, mở lại và kiểm tra: mã mẫu, công thức tổng nhóm
// và TỔNG KPI, ghi chú điểm thái độ (comment) có vào cột Ghi chú.
func TestBuildKPIWorkbook(t *testing.T) {
	settings := KPISettingsResponse{CustomerWeight: 10, ProgressWeight: 35, QualityWeight: 30, InitiativeWeight: 5, PunctualityWeight: 15, AttitudeWeight: 5}
	k := buildKPIMember("m1", "long", "Vũ Bảo Long", "", "month", "2026-10",
		&kpiTaskStats{totalWorkerEstimationReviewedDone: 10, lateWorkerEstimation: 0, totalWorkerEstimation: 10},
		0, 0, 0,
		kpiAttitude{score: 5, comment: "Chủ động, nhiệt tình"}, 1, 0, settings)

	recs := map[string]*kpiMemberRecords{
		"m1": {
			initiatives: []InitiativeResponse{{Content: "Tối ưu truy vấn", Estimate: 2, ApprovedAt: timePtr(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))}},
		},
	}

	data, err := buildKPIWorkbook([]KPIMemberResponse{k}, settings, recs, "10/2026", "Cao Việt Đức")
	if err != nil {
		t.Fatalf("buildKPIWorkbook: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty workbook")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetList()[0]
	if sheet != "long" {
		t.Fatalf("sheet name = %q, want long", sheet)
	}

	if got, _ := f.GetCellValue(sheet, "F1"); got != kpiFormCode {
		t.Fatalf("form code F1 = %q", got)
	}

	// Hàng dữ liệu bắt đầu ở 10: A=section, 11..13 con; B section ở 14; C ở 18.
	// TỔNG KPI ở hàng 20 (sau C.1=19, C.2=... chờ: A(10)+3con(11-13), B(14)+3(15-17), C(18)+2(19-20) => total 21).
	// Kiểm tra công thức thay vì số cụ thể cho chắc.
	fA, _ := f.GetCellFormula(sheet, "G10")
	if fA == "" {
		t.Fatalf("section A result G10 should be a formula, got empty")
	}
	totalF, _ := f.GetCellFormula(sheet, "G21")
	if totalF == "" {
		t.Fatalf("TỔNG KPI G21 should be a formula, got empty (formula=%q)", totalF)
	}

	// Ghi chú điểm thái độ ở C.2 (hàng 20), cột H.
	noteH20, _ := f.GetCellValue(sheet, "H20")
	if noteH20 != "Chủ động, nhiệt tình" {
		t.Fatalf("attitude comment note H20 = %q", noteH20)
	}
}

func timePtr(t time.Time) *time.Time { return &t }
