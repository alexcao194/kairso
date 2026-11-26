package quote

import (
	"os"
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/features/docsign"
)

func TestQuoteWorkbook(t *testing.T) {
	q := &QuoteResponse{
		Number: "BG-2026-0001", Title: "Phát triển module chấm công", Language: "vi", Currency: "VND",
		IssueDate: "2026-11-26", ValidUntil: "2026-12-26", VatRate: 8, DiscountType: "percent", DiscountPercent: 5,
		Subtotal: 30000000, DiscountTotal: 1500000, TaxTotal: 2280000, GrandTotal: 30780000,
		AmountInWords: "Ba mươi triệu bảy trăm tám mươi nghìn đồng",
		Issuer:        PartyInfo{Name: "Công ty TNHH Kairso", Address: "1 Lê Lợi, Q.1, TP. Hồ Chí Minh", TaxCode: "0312345678", Phone: "0909000000", Email: "hi@kairso.com", BankInfo: "VCB 0123456789"},
		Customer:      PartyInfo{Name: "Công ty ABC", Address: "Hà Nội"},
		Items: []QuoteItemResponse{
			{Name: "Phân tích yêu cầu", Description: "Khảo sát, viết tài liệu đặc tả chi tiết cho toàn bộ luồng chấm công và phê duyệt", Unit: "gói", Quantity: 1, UnitPrice: 10000000, LineTotal: 10000000},
			{Name: "Lập trình", Unit: "ngày công", Quantity: 10, UnitPrice: 2000000, DiscountPercent: 0, LineTotal: 20000000},
		},
		Notes:      "Giá chưa bao gồm chi phí máy chủ.",
		Signatures: []docsign.Signature{{Role: docsign.RolePreparer, Name: "Duc Cao", SignedAt: time.Now()}},
	}
	f, err := quoteWorkbook(q)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("XLSX_OUT"); out != "" {
		_ = os.WriteFile(out+"/"+f.Filename, f.Data, 0o644)
	}
}
