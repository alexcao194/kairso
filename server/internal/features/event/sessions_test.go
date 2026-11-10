package event

import (
	"testing"
	"time"
)

// Giờ dựng theo +07 (Việt Nam) để khớp cách getDaySessions lọc record theo
// ngày giờ Việt Nam — giờ UTC sẽ làm 17:54/18:00 rơi sang ngày hôm sau ở VN.
var ict = time.FixedZone("ICT", 7*3600)

func erec(id, typ string, h, m int) attendanceRecord {
	return attendanceRecord{ID: id, MemberID: "m1", Type: typ, RecordedAt: time.Date(2026, 11, 4, h, m, 0, 0, ict)}
}

// vào,vào,ra,ra chồng giờ (do yêu cầu bổ sung đè phiên cũ) → GỘP KHI HIỂN THỊ
// thành MỘT phiên theo biên ngoài (vào sớm nhất → ra muộn nhất). Record ở giữa
// (in2, out1) KHÔNG bị xoá, chỉ không hiện thành dòng riêng.
func TestGetDaySessions_OverlappingPairsMergeForDisplay(t *testing.T) {
	records := []attendanceRecord{
		erec("in1", attendanceTypeCheckIn, 8, 0),
		erec("in2", attendanceTypeCheckIn, 9, 19),
		erec("out1", attendanceTypeCheckOut, 17, 54),
		erec("out2", attendanceTypeCheckOut, 18, 0),
	}
	sessions := getDaySessions("2026-11-04", "m1", records)
	if len(sessions) != 1 {
		t.Fatalf("muốn 1 phiên gộp, có %d: %+v", len(sessions), sessions)
	}
	if sessions[0].CheckIn == nil || sessions[0].CheckOut == nil {
		t.Fatalf("phiên gộp phải hoàn chỉnh: %+v", sessions[0])
	}
	if sessions[0].CheckIn.ID != "in1" || sessions[0].CheckOut.ID != "out2" {
		t.Errorf("phiên gộp = %s→%s, muốn in1→out2 (08:00→18:00)", sessions[0].CheckIn.ID, sessions[0].CheckOut.ID)
	}
}

// Hai ca tách biệt (nghỉ trưa, không chồng giờ) KHÔNG bị gộp — vẫn là 2 phiên.
func TestGetDaySessions_NonOverlappingStaySeparate(t *testing.T) {
	records := []attendanceRecord{
		erec("in1", attendanceTypeCheckIn, 8, 0),
		erec("out1", attendanceTypeCheckOut, 12, 0),
		erec("in2", attendanceTypeCheckIn, 14, 0),
		erec("out2", attendanceTypeCheckOut, 18, 0),
	}
	sessions := getDaySessions("2026-11-04", "m1", records)
	if len(sessions) != 2 {
		t.Fatalf("muốn 2 phiên tách biệt, có %d: %+v", len(sessions), sessions)
	}
	if sessions[0].CheckIn.ID != "in1" || sessions[0].CheckOut.ID != "out1" ||
		sessions[1].CheckIn.ID != "in2" || sessions[1].CheckOut.ID != "out2" {
		t.Errorf("gộp nhầm 2 ca tách biệt: %+v", sessions)
	}
}

// Thừa vào (quên check-out) vẫn giữ phiên hở; thừa ra vẫn giữ phiên chỉ có ra.
func TestGetDaySessions_UnbalancedKeepsHalfSessions(t *testing.T) {
	records := []attendanceRecord{
		erec("in1", attendanceTypeCheckIn, 8, 0),
		erec("out1", attendanceTypeCheckOut, 12, 0),
		erec("in2", attendanceTypeCheckIn, 14, 0), // quên check-out
	}
	sessions := getDaySessions("2026-11-04", "m1", records)
	if len(sessions) != 2 {
		t.Fatalf("muốn 2 phiên, có %d", len(sessions))
	}
	if sessions[0].CheckIn == nil || sessions[0].CheckOut == nil {
		t.Errorf("phiên 1 phải hoàn chỉnh")
	}
	if sessions[1].CheckIn == nil || sessions[1].CheckOut != nil {
		t.Errorf("phiên 2 phải là phiên hở (chỉ có vào)")
	}
}
