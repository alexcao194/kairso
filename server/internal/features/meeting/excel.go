package meeting

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/docexport"
	"github.com/alexcao194/kairso/internal/features/docsign"
	"github.com/alexcao194/kairso/pkg/httpx"
)

var sectionTitles = map[MeetingSection]string{
	SectionDeployed: "Hạng mục đã triển khai",
	SectionPlanned:  "Hạng mục cần triển khai",
	SectionBacklog:  "Backlog",
}

func participantNames(list []MeetingParticipant) string {
	names := make([]string, 0, len(list))
	for _, p := range list {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return strings.Join(names, ", ")
}

func fmtMeetingDate(d string) string {
	if t, err := time.Parse("2006-01-02", d[:min(len(d), 10)]); err == nil {
		return t.Format("02/01/2006")
	}
	return d
}

// Export renders meeting minutes as an .xlsx sheet (Vietnamese, like the printed minutes).
func (s *service) Export(ctx context.Context, id, callerID string) (*ExportFile, error) {
	m, err := s.Get(ctx, id, callerID)
	if err != nil {
		return nil, err
	}
	return meetingWorkbook(m)
}

func meetingWorkbook(m *MeetingMinutesResponse) (*ExportFile, error) {
	// STT | Module | Content | Note | Tasks | Priority / status
	b, err := docexport.New("Biên bản họp", []float64{6, 18, 42, 28, 16, 16}, 0)
	if err != nil {
		return nil, err
	}
	date := fmtMeetingDate(m.MeetingDate)
	b.Title("BIÊN BẢN HỌP")
	if m.Title != "" {
		b.Subtitle(m.Title)
	}
	b.Blank()
	b.Field("Thời gian", date)
	b.Field("Địa điểm / hình thức", m.Location)
	b.Field("Thành phần tham dự", participantNames(m.Attendees))
	b.Field("Người ghi biên bản", participantNames(m.Recorders))
	if strings.TrimSpace(m.Summary) != "" {
		b.Blank()
		b.Bold("Nội dung cuộc họp")
		b.Line(m.Summary)
	}

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
		if sec == SectionDeployed && len(rows) > 0 {
			pending := true
			for _, r := range rows {
				if r.Status == ItemStatusDone {
					pending = false
				}
			}
			if pending {
				title = "Hạng mục đã triển khai, chờ cập nhật"
			}
		}
		b.Blank()
		b.Bold(title)
		if len(rows) > 0 {
			content, last := "Nội dung yêu cầu", "Ưu tiên"
			if sec == SectionDeployed {
				content, last = "Nội dung", "Trạng thái"
			}
			b.Header("STT", "Module", content, "Ghi chú", "Task", last)
			for i, r := range rows {
				codes := make([]string, 0, len(r.Tasks))
				for _, t := range r.Tasks {
					codes = append(codes, t.Code)
				}
				flag := ""
				switch {
				case sec == SectionDeployed && r.Status == ItemStatusDone:
					flag = "Đã cập nhật"
				case sec == SectionDeployed:
					flag = "Chờ cập nhật"
				case r.Priority:
					flag = "Ưu tiên"
				}
				b.Row(i+1, r.Module, r.Content, r.Note, strings.Join(codes, ", "), docexport.Center(flag))
			}
		}
		if note != "" {
			b.Italic("Ghi chú: " + note)
		}
	}

	roles := []struct{ role, label string }{{docsign.RolePreparer, "Người lập biên bản"}, {docsign.RoleController, "Người kiểm soát"}}
	first := true
	for _, r := range roles {
		for _, sig := range m.Signatures {
			if sig.Role != r.role {
				continue
			}
			if first {
				b.Blank()
				first = false
			}
			b.Line(fmt.Sprintf("%s: %s (%s)", r.label, sig.Name, sig.SignedAt.Format("02/01/2006")))
		}
	}

	data, err := b.Bytes()
	if err != nil {
		return nil, err
	}
	name := "Bien-ban-hop-" + strings.ReplaceAll(date, "/", "-")
	return &ExportFile{Filename: docexport.SafeFilename(name) + ".xlsx", Data: data}, nil
}

// @Summary Export meeting minutes as an Excel workbook
// @Tags Meetings
// @Security BearerAuth
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param meetingId path string true "Meeting minutes ID"
// @Success 200 {file} binary "Minutes workbook (.xlsx)"
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /v1/meetings/{meetingId}/export [get]
func (h *Handler) ExportMeeting(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "Chưa xác thực hoặc phiên đăng nhập đã hết hạn")
		return
	}
	file, err := h.svc.Export(r.Context(), r.PathValue("meetingId"), userID)
	if err != nil {
		writeServiceError(w, err, "Không thể xuất biên bản họp")
		return
	}
	w.Header().Set("Content-Type", docexport.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
