package valueobjects

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	domain "apexis/backend/internal/modules/organization/domain/errors"
)

// แม่แบบเอกสารพิมพ์ (HTML template) — organization เก็บแม่แบบ / รูปของบริษัท ไม่รู้จักข้อมูลของเอกสาร
// (ข้อมูลพิมพ์ประกอบที่หน้าเว็บจาก API ของ module เจ้าของเอกสาร)
type (
	DocumentTemplateID uuid.UUID
	DocumentAssetID    uuid.UUID
)

// Paper คือขนาด / แนวกระดาษของแม่แบบ
type Paper string

const (
	PaperA4Portrait  Paper = "a4_portrait"
	PaperA4Landscape Paper = "a4_landscape"
	PaperA5Portrait  Paper = "a5_portrait"
)

func ParsePaper(s string) (Paper, error) {
	switch p := Paper(strings.TrimSpace(s)); p {
	case PaperA4Portrait, PaperA4Landscape, PaperA5Portrait:
		return p, nil
	case "":
		return PaperA4Portrait, nil
	}
	return "", domain.ErrInvalidPaper
}

// TemplateDocTypes คือชนิดเอกสารที่พิมพ์ผ่านแม่แบบได้ (key คงที่ — หน้าเว็บใช้ key เดียวกันเลือกแม่แบบมาตรฐาน)
var TemplateDocTypes = []string{
	"sales.quotation",
	"sales.invoice",
	"sales.service_invoice",
	"sales.tax_invoice",
	"sales.deposit",
	"sales.billing_note",
	"sales.credit_note",
	"purchasing.order",
	"purchasing.work_order",
	"finance.receipt",
	"finance.wht_certificate",
	"finance.pnd3",
	"finance.pnd53",
	"finance.pp30",
	"finance.vat_report_output",
	"finance.vat_report_input",
	"manufacturing.production_order",
	"petty_cash.voucher",
	"petty_cash.replenishment",
}

func ParseTemplateDocType(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !slices.Contains(TemplateDocTypes, s) {
		return "", domain.ErrInvalidTemplateDocType
	}
	return s, nil
}
