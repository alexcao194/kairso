package meeting

import (
	"os"
	"testing"
)

func TestMeetingWorkbook(t *testing.T) {
	m := &MeetingMinutesResponse{
		Title: "Họp tiến độ sprint 12", MeetingDate: "2026-11-26", Location: "Google Meet",
		Attendees: []MeetingParticipant{{Name: "An"}, {Name: "Bình"}}, Recorders: []MeetingParticipant{{Name: "Chi"}},
		Summary: "Rà soát hạng mục đã triển khai; tiếp nhận yêu cầu mới.",
		Items: []MeetingItem{
			{Section: SectionDeployed, Module: "Chấm công", Content: "Sửa lỗi Wi-Fi", Status: ItemStatusDone},
			{Section: SectionPlanned, Module: "Báo giá", Content: "Xuất Excel", Priority: true, Tasks: []MeetingTaskRef{{Code: "KAI-12"}}},
		},
		SectionNotes: map[string]string{"planned": "Triển khai trong tuần"},
	}
	f, err := meetingWorkbook(m)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("XLSX_OUT"); out != "" {
		_ = os.WriteFile(out+"/"+f.Filename, f.Data, 0o644)
	}
}
