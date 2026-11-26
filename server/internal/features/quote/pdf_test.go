package quote

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

func sampleQuote() *QuoteResponse {
	q := &QuoteResponse{
		ID: "q1", Number: "BG-2026-0042", Title: "Triển khai phân hệ chấm công và báo cáo hiệu suất", Status: "sent",
		Currency: "VND", MinorUnits: 0, Language: "vi", IssueDate: "2026-11-26", ValidUntil: "2026-12-26",
		DiscountType: "percent", DiscountPercent: 5, VatRate: 8, ProjectName: "Kairso cho Công ty Ánh Dương", ModuleName: "Chấm công Wi-Fi",
		Customer: PartyInfo{Name: "Công ty TNHH Ánh Dương", Address: "12 Nguyễn Huệ, Quận 1, TP. Hồ Chí Minh", TaxCode: "0312345678", Phone: "028 3822 1234", Email: "ketoan@anhduong.vn", ContactName: "Chị Nguyễn Thị Hằng"},
		Issuer:   PartyInfo{Name: "Công ty Cổ phần Phần mềm Kairso", Address: "Tầng 5, 88 Láng Hạ, Đống Đa, Hà Nội", TaxCode: "0109876543", Phone: "024 3999 8888", Email: "sales@kairso.vn", BankInfo: "Ngân hàng Vietcombank – CN Thành Công\nSTK: 0011 0042 8888 · Chủ TK: Công ty CP Phần mềm Kairso"},
		Notes:    "Giá chưa bao gồm chi phí đi lại ngoài Hà Nội.\nThời gian triển khai dự kiến 6 tuần kể từ ngày ký hợp đồng.",
		Terms:    "Thanh toán 50% khi ký hợp đồng, 50% khi nghiệm thu. Bảo hành 12 tháng.",
		AmountInWords: "Một trăm năm mươi hai triệu, chín trăm bốn mươi tám nghìn đồng",
		Signatures:    []docsign.Signature{{Role: docsign.RolePreparer, UserID: "u1", Name: "Trần Quốc Việt", SignedAt: time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)}},
	}
	for i := 0; i < 14; i++ {
		q.Items = append(q.Items, QuoteItemResponse{ID: string(rune('a' + i)), Name: "Khảo sát, cấu hình máy chấm công & đồng bộ dữ liệu ca làm việc", Description: "Bao gồm cài đặt, hướng dẫn sử dụng cho quản trị viên và kiểm thử trên 3 chi nhánh.", Unit: "gói", Quantity: 1.5, UnitPrice: 12500000, DiscountPercent: float64(i % 2 * 10), LineTotal: 16875000})
	}
	q.Subtotal, q.DiscountTotal, q.TaxTotal, q.GrandTotal = 236250000, 11812500, 17955000, 242392500
	return q
}

// TestQuotePDF draws both templates. Set PDF_OUT to a folder to keep the files.
func TestQuotePDF(t *testing.T) {
	h := &Handler{pdf: docpdf.New(os.Getenv("PDF_FONT_DIR"), nil)}
	q := sampleQuote()
	cases := map[string]func() ([]byte, error){
		"quote-modern.pdf": func() ([]byte, error) { return h.modernQuotePDF(context.Background(), "", q) },
		"quote-admin.pdf": func() ([]byte, error) {
			return h.adminQuotePDF(context.Background(), "", q, docpdf.Options{Admin: true, Merge: "0"})
		},
		"quote-admin-single.pdf": func() ([]byte, error) {
			return h.adminQuotePDF(context.Background(), "", q, docpdf.Options{Admin: true, Merge: "1", Place: "Hà Nội"})
		},
		"quote-draft-en.pdf": func() ([]byte, error) {
			c := *q
			c.Status, c.Language, c.Currency, c.MinorUnits = "draft", "en", "USD", 2
			return h.modernQuotePDF(context.Background(), "", &c)
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

func TestFormatMoney(t *testing.T) {
	for _, c := range []struct {
		minor int64
		cur   string
		units int
		lang  string
		want  string
	}{
		{242392500, "VND", 0, "vi", "242.392.500 ₫"},
		{123456, "USD", 2, "en", "$1,234.56"},
		{123456, "USD", 2, "vi", "1.234,56 US$"},
		{-5000, "VND", 0, "vi", "-5.000 ₫"},
	} {
		if got := formatMoney(c.minor, c.cur, c.units, c.lang); got != c.want {
			t.Errorf("formatMoney(%d %s %s) = %q, want %q", c.minor, c.cur, c.lang, got, c.want)
		}
	}
	if got := formatNumber(1.5, 3, false, "vi"); got != "1,5" {
		t.Errorf("qty = %q", got)
	}
	if !strings.Contains(formatNumber(1234567, 0, true, "en"), ",") {
		t.Error("en grouping")
	}
}
