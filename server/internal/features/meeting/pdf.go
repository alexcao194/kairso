package meeting

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docpdf"
	"github.com/alexcao194/kairso/internal/features/docsign"
	"github.com/alexcao194/kairso/pkg/httpx"
)

// SetPDF enables PDF export.
func (h *Handler) SetPDF(e *docpdf.Exporter) { h.pdf = e }

// TaskDueDates returns the due dates of tasks linked to meeting items.
func (s *service) TaskDueDates(ctx context.Context, ids []string) (map[string]time.Time, error) {
	return s.repo.TaskDueDates(ctx, ids)
}

// @Summary Export meeting minutes as a PDF
// @Description A4 PDF drawn on the server in Times New Roman, laid out like the print page.
// @Tags Meetings
// @Security BearerAuth
// @Produce application/pdf
// @Param meetingId path string true "Meeting minutes ID"
// @Param template query string false "Layout" Enums(modern, admin)
// @Param sign query string false "Signature boxes (admin layout): stamped when signed in the app, or left blank to sign by hand" Enums(digital, wet)
// @Param place query string false "Place in the date line (admin layout)"
// @Success 200 {file} binary "Minutes (.pdf)"
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 503 {object} httpx.ErrorResponse
// @Router /v1/meetings/{meetingId}/pdf [get]
func (h *Handler) ExportMeetingPDF(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	m, err := h.svc.Get(r.Context(), r.PathValue("meetingId"), userID)
	if err != nil {
		writeServiceError(w, err, "Không thể xuất biên bản họp")
		return
	}
	opts := docpdf.ParseOptions(r)
	data, err := h.minutesPDF(r.Context(), m, opts)
	if err != nil {
		slog.Error("meeting pdf export failed", "meeting", m.ID, "error", err)
		httpx.Error(w, docpdf.Status(err), "Không thể tạo PDF biên bản họp: "+err.Error())
		return
	}
	name := "Bien-ban-hop-" + strings.ReplaceAll(fmtMeetingDate(m.MeetingDate), "/", "-")
	docpdf.Write(w, docexport.SafeFilename(name)+".pdf", data)
}

type sectionRows struct {
	sec   MeetingSection
	title string
	rows  []MeetingItem
	note  string
}

func minutesSections(m *MeetingMinutesResponse) []sectionRows {
	var out []sectionRows
	for _, sec := range []MeetingSection{SectionDeployed, SectionPlanned, SectionBacklog} {
		var rows []MeetingItem
		for _, it := range m.Items {
			if it.Section == sec {
				rows = append(rows, it)
			}
		}
		note := strings.TrimSpace(m.SectionNotes[string(sec)])
		if len(rows) == 0 && note == "" {
			continue
		}
		title := sectionTitles[sec]
		if sec == SectionDeployed && len(rows) > 0 && !anyDone(rows) {
			title = "Hạng mục đã triển khai, chờ cập nhật"
		}
		out = append(out, sectionRows{sec, title, rows, note})
	}
	return out
}

func anyDone(rows []MeetingItem) bool {
	for _, r := range rows {
		if r.Status == ItemStatusDone {
			return true
		}
	}
	return false
}

// meetingLead is the one-line overview of the print page ("Cuộc họp rà soát 3 hạng mục…").
func meetingLead(m *MeetingMinutesResponse) string {
	counts := map[MeetingSection]int{}
	modules := map[string]bool{}
	for _, it := range m.Items {
		if strings.TrimSpace(it.Content) == "" {
			continue
		}
		counts[it.Section]++
		if mod := strings.ToLower(strings.TrimSpace(it.Module)); mod != "" {
			modules[mod] = true
		}
	}
	var parts []string
	if n := counts[SectionDeployed]; n > 0 {
		parts = append(parts, fmt.Sprintf("rà soát %d hạng mục đã triển khai", n))
	}
	if n := counts[SectionPlanned]; n > 0 {
		parts = append(parts, fmt.Sprintf("ghi nhận %d hạng mục cần triển khai", n))
	}
	if n := counts[SectionBacklog]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d hạng mục backlog", n))
	}
	if len(parts) == 0 {
		return ""
	}
	joined := parts[len(parts)-1]
	if len(parts) > 1 {
		joined = strings.Join(parts[:len(parts)-1], ", ") + " và " + joined
	}
	s := "Cuộc họp " + joined
	if len(modules) > 0 {
		s += fmt.Sprintf(" trên %d phân hệ", len(modules))
	}
	return s + "."
}

func contentLabel(sec MeetingSection) string {
	if sec == SectionDeployed {
		return "Nội dung"
	}
	return "Nội dung yêu cầu"
}

func statusLabel(r MeetingItem) string {
	if r.Status == ItemStatusDone {
		return "Đã cập nhật"
	}
	return "Chờ cập nhật"
}

// itemDue is the latest due date among the item's linked tasks.
func itemDue(r MeetingItem, due map[string]time.Time) string {
	var latest time.Time
	for _, t := range r.Tasks {
		if d, ok := due[t.ID]; ok && d.After(latest) {
			latest = d
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.In(vnTime).Format("02/01/2006")
}

var vnTime = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*3600)
}()

func signatureOf(m *MeetingMinutesResponse, role string) (string, time.Time) {
	for _, s := range m.Signatures {
		if s.Role == role {
			return s.Name, s.SignedAt
		}
	}
	return "", time.Time{}
}

func (h *Handler) minutesPDF(ctx context.Context, m *MeetingMinutesResponse, o docpdf.Options) ([]byte, error) {
	var ids []string
	for _, it := range m.Items {
		for _, t := range it.Tasks {
			ids = append(ids, t.ID)
		}
	}
	due := map[string]time.Time{}
	if len(ids) > 0 {
		if got, err := h.svc.TaskDueDates(ctx, ids); err == nil {
			due = got
		}
	}
	if o.Admin {
		return h.adminMinutesPDF(ctx, m, o, due)
	}
	return h.modernMinutesPDF(ctx, m, due)
}

// modernMinutesPDF mirrors MeetingPrint.
func (h *Handler) modernMinutesPDF(ctx context.Context, m *MeetingMinutesResponse, due map[string]time.Time) ([]byte, error) {
	d, ws, err := h.pdf.Begin(ctx, m.WorkspaceID, docpdf.ModernMargins())
	if err != nil {
		return nil, err
	}
	d.Start()
	px := docpdf.Px
	left, width := d.Left(), d.Width()

	y0 := d.P.GetY()
	d.Logo(left+width-15, y0, 15, ws.HasLogo)
	d.Font("B", px(10), d.Brand)
	d.P.SetXY(left, y0)
	d.P.CellFormat(width-19, docpdf.LineH(px(10), 1.4), "BIÊN BẢN HỌP", "", 2, "L", false, 0, "")
	d.Gap(1)
	d.Font("B", px(21), docpdf.Ink)
	d.P.SetX(left)
	d.P.MultiCell(width-19, docpdf.LineH(px(21), 1.25), m.Title, "", "L", false)
	y := max(d.P.GetY(), y0+15) + 3
	d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
	d.P.Rect(left, y, width, 0.5, "F")
	d.P.SetXY(left, y+4)

	info := func(x, w float64, label, value string) {
		if value == "" {
			value = "—"
		}
		d.Font("B", px(9), d.Brand)
		d.P.SetX(x)
		d.P.CellFormat(w, docpdf.LineH(px(9), 1.4), strings.ToUpper(label), "", 2, "L", false, 0, "")
		d.Font("", px(12), docpdf.Ink)
		d.P.SetX(x)
		d.P.MultiCell(w-4, docpdf.LineH(px(12), 1.35), value, "", "L", false)
	}
	y = d.P.GetY()
	unit := width / 3.8
	cols := []struct {
		w            float64
		label, value string
	}{
		{unit, "Thời gian", fmtMeetingDate(m.MeetingDate)},
		{unit * 1.6, "Địa điểm / hình thức", m.Location},
		{unit * 1.2, "Người ghi biên bản", participantNames(m.Recorders)},
	}
	x, bottom := left, y
	for _, c := range cols {
		d.P.SetY(y)
		info(x, c.w, c.label, c.value)
		bottom = max(bottom, d.P.GetY())
		x += c.w
	}
	d.P.SetY(bottom + 2)
	info(left, width, "Thành phần tham dự", participantNames(m.Attendees))
	y = d.P.GetY() + 3
	d.HLine(left, left+width, y, 0.25, docpdf.Line)
	d.P.SetXY(left, y+4)

	n := 0
	heading := func(title string) {
		n++
		d.Ensure(20)
		y := d.P.GetY()
		r := 3.0
		d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
		d.P.Circle(left+r, y+r, r, "F")
		d.Font("B", px(11), docpdf.White)
		num := strconv.Itoa(n)
		d.P.Text(left+r-d.P.GetStringWidth(num)/2, y+r+1.3, num)
		d.Font("B", px(14), docpdf.Ink)
		d.P.SetXY(left+2*r+2.5, y)
		d.P.CellFormat(width-2*r-2.5, 2*r, title, "", 2, "L", false, 0, "")
		d.P.SetXY(left, y+2*r+2)
	}
	lh := docpdf.LineH(px(12), 1.5)
	lead := meetingLead(m)
	if lead != "" || strings.TrimSpace(m.Summary) != "" {
		heading("Thông tin chung")
		if lead != "" {
			d.Font("", px(12), docpdf.Ink2)
			d.Text(left, width, lead, "L", lh)
		}
		if s := strings.TrimSpace(m.Summary); s != "" {
			d.Gap(1)
			d.Runs(lh, 0, docpdf.Run{Text: "Nội dung: ", Style: "B", Size: px(12), Color: docpdf.Ink},
				docpdf.Run{Text: s, Size: px(12), Color: docpdf.Ink2})
		}
		d.Gap(4)
	}

	for _, s := range minutesSections(m) {
		heading(s.title)
		if len(s.rows) > 0 {
			showStatus := s.sec == SectionDeployed && anyDone(s.rows)
			withTasks := s.sec != SectionDeployed
			showDue := false
			if withTasks {
				for _, r := range s.rows {
					if itemDue(r, due) != "" {
						showDue = true
					}
				}
			}
			cols := []docpdf.Col{{Head: "STT", W: 13}, {Head: "Phân hệ", W: 34}, {Head: contentLabel(s.sec)}}
			if showDue {
				cols = append(cols, docpdf.Col{Head: "Hạn", W: 24})
			}
			if showStatus {
				cols = append(cols, docpdf.Col{Head: "Trạng thái", W: 28})
			}
			if withTasks {
				cols = append(cols, docpdf.Col{Head: "Ưu tiên", W: 22})
			}
			brand := d.Brand
			var rows [][]docpdf.Cell
			for i, r := range s.rows {
				codes := make([]string, 0, len(r.Tasks))
				for _, t := range r.Tasks {
					codes = append(codes, t.Code)
				}
				sub := joinParts(" · ", r.Note, strings.Join(codes, ", "))
				row := []docpdf.Cell{{Text: strconv.Itoa(i + 1), Bold: true, Color: &brand}, {Text: r.Module, Bold: true}, {Text: r.Content, Sub: sub}}
				if showDue {
					row = append(row, docpdf.C(itemDue(r, due)))
				}
				if showStatus {
					row = append(row, docpdf.C(statusLabel(r)))
				}
				if withTasks {
					pri := docpdf.Cell{}
					if r.Priority {
						pri = docpdf.Cell{Text: "Ưu tiên", Bold: true, Color: &brand}
					}
					row = append(row, pri)
				}
				rows = append(rows, row)
			}
			d.Table(cols, rows, docpdf.ModernTable())
		}
		if s.note != "" {
			d.Gap(2.5)
			y := d.P.GetY()
			d.Runs(lh, 3, docpdf.Run{Text: "Ghi chú: ", Style: "B", Size: px(12), Color: docpdf.Ink},
				docpdf.Run{Text: s.note, Size: px(12), Color: docpdf.Ink2})
			d.P.SetFillColor(d.Brand.R, d.Brand.G, d.Brand.B)
			if d.P.GetY() > y {
				d.P.Rect(left, y, 0.6, d.P.GetY()-y, "F")
			}
		}
		d.Gap(5)
	}

	var slots []docpdf.Slot
	for _, role := range []struct{ role, label string }{{docsign.RolePreparer, "Người lập"}, {docsign.RoleController, "Người kiểm soát"}} {
		name, at := signatureOf(m, role.role)
		slots = append(slots, docpdf.Slot{Title: role.label, Hint: docpdf.HintSign, Name: name, SignedAt: at})
	}
	d.ModernSignatures(slots)
	return d.Bytes()
}

var roman = []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII"}

// adminMinutesPDF mirrors AdminMeetingPrint: a Decree 30 "biên bản".
func (h *Handler) adminMinutesPDF(ctx context.Context, m *MeetingMinutesResponse, o docpdf.Options, due map[string]time.Time) ([]byte, error) {
	d, ws, err := h.pdf.Begin(ctx, m.WorkspaceID, docpdf.AdminMargins())
	if err != nil {
		return nil, err
	}
	d.Start()
	lh := docpdf.LineH(13, 1.35)
	day, err := time.Parse("2006-01-02", m.MeetingDate[:min(len(m.MeetingDate), 10)])
	if err != nil {
		day = time.Now()
	}
	d.AdminHeader(ws.Name, "", true, day, o.Place)
	d.Gap(8)
	d.Centered("BIÊN BẢN HỌP", "B", 14, docpdf.Black, docpdf.LineH(14, 1.3))
	d.Centered(m.Title, "B", 13, docpdf.Black, lh)
	d.Gap(6)

	run := func(text, style string) docpdf.Run {
		return docpdf.Run{Text: text, Style: style, Size: 13, Color: docpdf.Black}
	}
	or := func(v string) string {
		if strings.TrimSpace(v) == "" {
			return docpdf.Blank
		}
		return v
	}
	heading := func(n int, title string) {
		d.Ensure(lh * 3)
		d.Gap(2)
		d.Runs(lh, 0, run(fmt.Sprintf("%s. %s", roman[n], strings.ToUpper(title)), "B"))
	}
	heading(0, "Thời gian, địa điểm")
	d.Runs(lh, 0, run("- Thời gian: "+fmtMeetingDate(m.MeetingDate), ""))
	d.Runs(lh, 0, run("- Địa điểm: "+or(m.Location), ""))
	heading(1, "Thành phần tham dự")
	d.Runs(lh, 0, run("- "+or(participantNames(m.Attendees)), ""))
	d.Runs(lh, 0, run("- Thư ký (người ghi biên bản): "+or(participantNames(m.Recorders)), ""))
	heading(2, "Nội dung cuộc họp")
	if lead := meetingLead(m); lead != "" {
		d.Runs(lh, 10, run(lead, ""))
	}
	if s := strings.TrimSpace(m.Summary); s != "" {
		d.Runs(lh, 10, run("Tóm tắt: ", "B"), run(s, ""))
	}
	for i, s := range minutesSections(m) {
		d.Ensure(lh * 3)
		d.Gap(1)
		d.Runs(lh, 0, run(fmt.Sprintf("%d. %s", i+1, s.title), "B"))
		if len(s.rows) > 0 {
			showStatus := s.sec == SectionDeployed && anyDone(s.rows)
			withTasks := s.sec != SectionDeployed
			showDue, showPriority := false, false
			for _, r := range s.rows {
				if withTasks && itemDue(r, due) != "" {
					showDue = true
				}
				if withTasks && r.Priority {
					showPriority = true
				}
			}
			cols := []docpdf.Col{{Head: "STT", W: 12, Align: "C"}, {Head: "Hạng mục", W: 32}, {Head: contentLabel(s.sec)}}
			if showDue {
				cols = append(cols, docpdf.Col{Head: "Hạn", W: 24, Align: "C"})
			}
			if showStatus {
				cols = append(cols, docpdf.Col{Head: "Trạng thái", W: 26, Align: "C"})
			}
			if showPriority {
				cols = append(cols, docpdf.Col{Head: "Ưu tiên", W: 18, Align: "C"})
			}
			var rows [][]docpdf.Cell
			for j, r := range s.rows {
				row := []docpdf.Cell{docpdf.C(strconv.Itoa(j + 1)), docpdf.C(r.Module), {Text: r.Content, Sub: r.Note}}
				if showDue {
					row = append(row, docpdf.C(itemDue(r, due)))
				}
				if showStatus {
					row = append(row, docpdf.C(statusLabel(r)))
				}
				if showPriority {
					v := ""
					if r.Priority {
						v = "Có"
					}
					row = append(row, docpdf.C(v))
				}
				rows = append(rows, row)
			}
			d.Gap(1)
			d.Table(cols, rows, docpdf.AdminTable())
		}
		if s.note != "" {
			d.Gap(1)
			d.Runs(lh, 0, run("Ghi chú: ", "I"), run(s.note, ""))
		}
	}
	d.Gap(2)
	d.Runs(lh, 10, run("Cuộc họp kết thúc vào hồi …… giờ …… cùng ngày. Biên bản đã được đọc lại cho các thành phần dự họp cùng nghe và thống nhất.", ""))

	pName, pAt := signatureOf(m, docsign.RolePreparer)
	cName, cAt := signatureOf(m, docsign.RoleController)
	d.AdminSignatures([]docpdf.Slot{
		{Title: "NGƯỜI LẬP BIÊN BẢN", Hint: docpdf.HintSign, Name: pName, SignedAt: pAt},
		{Title: "NGƯỜI KIỂM SOÁT", Hint: docpdf.HintSeal, Name: cName, SignedAt: cAt},
	}, o.Wet, nil)
	return d.Bytes()
}

func joinParts(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
