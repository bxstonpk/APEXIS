package repositories

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/purchasing/application/dto"
	ports "apexis/backend/internal/modules/purchasing/application/ports/repositories"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.PurchaseDocumentRepository = (*PurchaseDocumentRepository)(nil)

// PurchaseDocumentRepository: รายการรวม "เอกสารซื้อ" — UNION ALL ของใบสั่ง / ใบรับ / ใบแจ้งหนี้ (แต่ละตารางมีทั้งสายสินค้าและบริการแยกด้วย kind)
//
// query เดียว: แต่ละส่วนของ UNION กรองบริษัท / สาย / ผู้ขาย / สถานะ / ช่วงวันที่ในตารางของตัวเอง
// (ใช้ index (company_id, วันที่) ของแต่ละตาราง) แล้ว JOIN ผู้ขาย ค้นหา เรียง และแบ่งหน้าในฐานข้อมูล
type PurchaseDocumentRepository struct{ db *gorm.DB }

func NewPurchaseDocumentRepository(db *gorm.DB) *PurchaseDocumentRepository {
	return &PurchaseDocumentRepository{db: db}
}

// purchaseDocPart คือส่วนหนึ่งของ UNION: ตาราง + ชนิดเอกสารของสินค้า / บริการ + คอลัมน์ที่ map เข้าแถวร่วม
type purchaseDocPart struct {
	table, date, number, reference, total, outstanding, vatStatus string
	goods, service                                                vo.PurchaseDocType
}

var purchaseDocParts = []purchaseDocPart{
	{
		table: "purchasing_orders", date: "order_date", number: "COALESCE(t.order_no, '')", reference: "t.reference",
		total: "t.total_gross", outstanding: "NULL::numeric", vatStatus: "''",
		goods: vo.DocTypeOrder, service: vo.DocTypeWorkOrder,
	},
	{
		table: "purchasing_receipts", date: "receipt_date", number: "t.document_no", reference: "t.reference",
		total: "NULL::numeric", outstanding: "NULL::numeric", vatStatus: "''",
		goods: vo.DocTypeReceipt, service: vo.DocTypeServiceAcceptance,
	},
	{
		table: "purchasing_invoices", date: "posting_date", number: "t.document_no", reference: "t.supplier_invoice_no",
		total: "t.total_gross", outstanding: "t.total_gross - t.paid_amount",
		// ตรงกับ entities.Invoice.VatStatus: ภาษีรอใบกำกับ = vat - ที่ได้รับใบกำกับแล้ว - ที่ลดหนี้
		vatStatus: `CASE WHEN t.total_vat = 0 THEN 'none'
			WHEN t.tax_invoice_received OR t.total_vat - t.vat_recognized - t.vat_credited <= 0 THEN 'claimed'
			WHEN t.vat_recognized > 0 THEN 'partial' ELSE 'undue' END`,
		goods: vo.DocTypeInvoice, service: vo.DocTypeServiceInvoice,
	},
}

type purchaseDocRow struct {
	DocType      string
	ID           uuid.UUID
	DocumentNo   string
	DocDate      time.Time
	SupplierID   uuid.UUID
	SupplierCode string
	SupplierName string
	Status       string
	Reference    string
	Gross        *decimal.Decimal
	Outstanding  *decimal.Decimal
	VatStatus    string
	Total        int64 // COUNT(*) OVER() ของ FetchPage
}

func (r purchaseDocRow) item() dto.PurchaseDocument {
	return dto.PurchaseDocument{
		Type: vo.PurchaseDocType(r.DocType), ID: r.ID.String(), DocumentNo: r.DocumentNo, Date: gp.DateUTC(r.DocDate),
		Supplier: dto.Ref{ID: r.SupplierID.String(), Code: r.SupplierCode, Name: r.SupplierName},
		Status:   r.Status, Reference: r.Reference, Total: r.Gross, Outstanding: r.Outstanding, VatStatus: r.VatStatus,
	}
}


func (r *PurchaseDocumentRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.PurchaseDocumentFilter) (listing.Result[dto.PurchaseDocument], error) {
	wanted := make(map[vo.PurchaseDocType]bool, len(f.Types))
	for _, t := range f.Types {
		wanted[t] = true
	}
	var parts []string
	var args []any
	for _, p := range purchaseDocParts {
		var kinds []string
		if wanted[p.goods] {
			kinds = append(kinds, string(vo.KindGoods))
		}
		if wanted[p.service] {
			kinds = append(kinds, string(vo.KindService))
		}
		if len(kinds) == 0 {
			continue
		}
		sql, a := p.selectSQL(companyID, kinds, f)
		parts = append(parts, sql)
		args = append(args, a...)
	}
	if len(parts) == 0 {
		return listing.Result[dto.PurchaseDocument]{Items: []dto.PurchaseDocument{}, Page: f.Page}, nil
	}
	union := strings.Join(parts, "\nUNION ALL\n")
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "d.doc_date", "document_no": "d.document_no"}, "d.id")
	if err != nil {
		return listing.Result[dto.PurchaseDocument]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).Table("("+union+") AS d", args...).
			Joins("JOIN purchasing_suppliers AS s ON s.id = d.supplier_id")
		return search(q, f.Search, "d.document_no", "d.reference", "s.code", "s.name")
	}
	columns := `d.doc_type, d.id, d.document_no, d.doc_date, d.supplier_id, s.code AS supplier_code, s.name AS supplier_name,
		d.status, d.reference, d.gross, d.outstanding, d.vat_status`
	rows, total, err := database.FetchPage(base, columns, order, f.Page, func(x purchaseDocRow) int64 { return x.Total })
	return toResult(rows, total, f.Page, purchaseDocRow.item), err
}

// selectSQL: SELECT ของตารางนี้ (alias t) พร้อมเงื่อนไข — ชื่อตาราง / คอลัมน์มาจากค่าคงที่ในโค้ด ค่าจากผู้ใช้ผ่าน placeholder
func (p purchaseDocPart) selectSQL(companyID vo.CompanyID, kinds []string, f dto.PurchaseDocumentFilter) (string, []any) {
	var b strings.Builder
	b.WriteString(`SELECT CASE WHEN t.kind = 'service' THEN '` + string(p.service) + `' ELSE '` + string(p.goods) + `' END AS doc_type,
		t.id, ` + p.number + ` AS document_no, t.` + p.date + ` AS doc_date, t.supplier_id, t.status,
		` + p.reference + ` AS reference, ` + p.total + ` AS gross, ` + p.outstanding + ` AS outstanding, ` + p.vatStatus + ` AS vat_status
		FROM ` + p.table + ` AS t WHERE t.company_id = ? AND t.kind IN ?`)
	args := []any{uuidOf(companyID), kinds}
	if f.SupplierID != nil {
		b.WriteString(" AND t.supplier_id = ?")
		args = append(args, uuidOf(*f.SupplierID))
	}
	if len(f.Statuses) > 0 {
		b.WriteString(" AND t.status IN ?")
		args = append(args, f.Statuses)
	}
	if f.StartDate != nil {
		b.WriteString(" AND t." + p.date + " >= ?")
		args = append(args, *f.StartDate)
	}
	if f.EndDate != nil {
		b.WriteString(" AND t." + p.date + " <= ?")
		args = append(args, *f.EndDate)
	}
	return b.String(), args
}
