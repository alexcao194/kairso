package meeting

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/alexcao194/kairso/internal/features/docsign"
)

func sampleMinutes() *MeetingMinutesResponse {
	m := &MeetingMinutesResponse{
		ID: "m1", Title: "Họp rà soát tiến độ dự án Ánh Dương – tuần 47", MeetingDate: "2026-11-24", Location: "Văn phòng khách hàng, 12 Nguyễn Huệ",
		Attendees:    []MeetingParticipant{{Name: "Trần Quốc Việt"}, {Name: "Nguyễn Thị Hằng"}, {Name: "Lê Minh Đức"}},
		Recorders:    []MeetingParticipant{{Name: "Phạm Thu Trang"}},
		Summary:      "Thống nhất ưu tiên phân hệ chấm công trước Tết, dời báo cáo KPI sang tháng 1.",
		SectionNotes: map[string]string{"planned": "Khách hàng gửi danh sách ca làm việc trước thứ Sáu."},
		Signatures:   []docsign.Signature{{Role: docsign.RolePreparer, Name: "Phạm Thu Trang", SignedAt: time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)}},
	}
	for i := 0; i < 6; i++ {
		m.Items = append(m.Items, MeetingItem{ID: "d" + string(rune('a'+i)), Section: SectionDeployed, Module: "Chấm công", Content: "Đồng bộ dữ liệu máy chấm công về hệ thống, xử lý trùng lượt quẹt thẻ", Status: MeetingItemStatus(map[bool]string{true: "done", false: "open"}[i%2 == 0])})
	}
	for i := 0; i < 8; i++ {
		m.Items = append(m.Items, MeetingItem{ID: "p" + string(rune('a'+i)), Section: SectionPlanned, Module: "Báo cáo", Content: "Thêm báo cáo đi muộn theo phòng ban, xuất Excel", Priority: i%3 == 0, Note: "Cần mẫu báo cáo từ phòng nhân sự", Tasks: []MeetingTaskRef{{ID: "t1", Code: "KAI-12" + string(rune('0'+i))}}})
	}
	m.Items = append(m.Items, MeetingItem{ID: "b1", Section: SectionBacklog, Module: "Di động", Content: "Ứng dụng di động cho quản lý duyệt lịch"})
	return m
}

func TestMinutesPDF(t *testing.T) {
	h := &Handler{pdf: docpdf.New(os.Getenv("PDF_FONT_DIR"), nil)}
	m := sampleMinutes()
	due := map[string]time.Time{"t1": time.Date(2026, 12, 15, 10, 0, 0, 0, time.UTC)}
	cases := map[string]func() ([]byte, error){
		"minutes-modern.pdf": func() ([]byte, error) { return h.modernMinutesPDF(context.Background(), m, due) },
		"minutes-admin.pdf": func() ([]byte, error) {
			return h.adminMinutesPDF(context.Background(), m, docpdf.Options{Admin: true, Place: "TP. Hồ Chí Minh"}, due)
		},
	}
	for name, run := range cases {
		data, err := run()
		if errors.Is(err, docpdf.ErrUnavailable) {
			t.Skip("no Times New Roman / Liberation Serif font on this machine")
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Fatalf("%s: not a PDF", name)
		}
		if dir := os.Getenv("PDF_OUT"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
		}
	}
}

func TestMeetingLead(t *testing.T) {
	got := meetingLead(sampleMinutes())
	want := "Cuộc họp rà soát 6 hạng mục đã triển khai, ghi nhận 8 hạng mục cần triển khai và 1 hạng mục backlog trên 3 phân hệ."
	if got != want {
		t.Fatalf("lead = %q", got)
	}
	if !strings.HasPrefix(itemDue(sampleMinutes().Items[6], map[string]time.Time{"t1": time.Date(2026, 12, 15, 20, 0, 0, 0, time.UTC)}), "16/12") {
		t.Fatal("due dates are shown in Vietnam time")
	}
}
