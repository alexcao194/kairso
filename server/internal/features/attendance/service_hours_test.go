package attendance

import (
	"testing"
	"time"
)

func at(h, m int) time.Time { return time.Date(2026, 11, 4, h, m, 0, 0, time.UTC) }

func rec(typ AttendanceType, h, m int) AttendanceRecordResponse {
	return AttendanceRecordResponse{Type: typ, RecordedAt: at(h, m)}
}

func day(sessions ...[2]time.Time) []WorkScheduleDayResponse {
	ws := make([]WorkScheduleSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		ws = append(ws, WorkScheduleSessionResponse{StartAt: s[0], EndAt: s[1]})
	}
	return []WorkScheduleDayResponse{{Date: "2026-11-04", Status: WorkScheduleStatusApproved, Sessions: ws}}
}

func approxEqual(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// Kịch bản bug: yêu cầu bổ sung công (vào 08:00, ra 18:00) đè lên phiên đã có
// (vào 09:19, ra 17:54) → 4 record vào,vào,ra,ra chồng giờ nhau. Ghép FIFO cho
// hai phiên hoàn chỉnh; union không cộng trùng: có mặt 10h, giờ công 8h.
func TestAttendanceHours_OverlappingSupplementRequest(t *testing.T) {
	records := []AttendanceRecordResponse{
		rec(AttendanceTypeCheckIn, 8, 0),
		rec(AttendanceTypeCheckIn, 9, 19),
		rec(AttendanceTypeCheckOut, 17, 54),
		rec(AttendanceTypeCheckOut, 18, 0),
	}
	schedules := day([2]time.Time{at(8, 0), at(12, 0)}, [2]time.Time{at(14, 0), at(18, 0)})

	if got := attendanceHours(records, schedules); !approxEqual(got, 8) {
		t.Fatalf("giờ công = %v, muốn 8", got)
	}

	presence := unionIntervals(pairPresenceIntervals(records))
	var total time.Duration
	for _, p := range presence {
		total += p.end.Sub(p.start)
	}
	if h := total.Hours(); !approxEqual(h, 10) {
		t.Fatalf("có mặt (union) = %v, muốn 10", h)
	}
}

// Ngày bình thường có nghỉ trưa: vào/ra xen kẽ, không chồng giờ → giờ công 8h.
func TestAttendanceHours_NormalLunchBreak(t *testing.T) {
	records := []AttendanceRecordResponse{
		rec(AttendanceTypeCheckIn, 8, 0),
		rec(AttendanceTypeCheckOut, 12, 0),
		rec(AttendanceTypeCheckIn, 14, 0),
		rec(AttendanceTypeCheckOut, 18, 0),
	}
	schedules := day([2]time.Time{at(8, 0), at(12, 0)}, [2]time.Time{at(14, 0), at(18, 0)})
	if got := attendanceHours(records, schedules); !approxEqual(got, 8) {
		t.Fatalf("giờ công = %v, muốn 8", got)
	}
}

// Vào trễ/ra sớm: chỉ phần nằm trong ca được tính (09:19→17:54 so với 08-12,14-18).
func TestAttendanceHours_LateInEarlyOutClampedToShift(t *testing.T) {
	records := []AttendanceRecordResponse{
		rec(AttendanceTypeCheckIn, 9, 19),
		rec(AttendanceTypeCheckOut, 17, 54),
	}
	schedules := day([2]time.Time{at(8, 0), at(12, 0)}, [2]time.Time{at(14, 0), at(18, 0)})
	// (12:00-09:19) + (17:54-14:00) = 2h41 + 3h54 = 6h35 = 6.583...h
	want := (2.0 + 41.0/60) + (3.0 + 54.0/60)
	if got := attendanceHours(records, schedules); got < want-1e-6 || got > want+1e-6 {
		t.Fatalf("giờ công = %v, muốn %v", got, want)
	}
}

func TestUnionIntervals_MergesOverlap(t *testing.T) {
	u := unionIntervals([]timeInterval{
		{at(8, 0), at(17, 54)},
		{at(9, 19), at(18, 0)},
	})
	if len(u) != 1 || !u[0].start.Equal(at(8, 0)) || !u[0].end.Equal(at(18, 0)) {
		t.Fatalf("union = %+v, muốn 1 khoảng 08:00–18:00", u)
	}
}
