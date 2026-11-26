package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/purchasing/application/dto"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	gp "apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

// ---------- ภาษีซื้อ ----------

// inputVatRows: ใบแจ้งหนี้ผู้ขาย (บวก) + ใบลดหนี้ผู้ขาย (ลบ) ที่มี VAT, ยังไม่ยกเลิก, วันที่ลงบัญชีอยู่ในช่วง
// ชื่อ / เลขผู้เสียภาษี / สาขาของผู้ขายตามใบกำกับภาษี ณ วันบันทึก (bill_from_*) — ไม่ใช่ทะเบียนผู้ขายปัจจุบัน
// คืน SQL ของ CTE x + args (บริษัท / ช่วงวันที่ / ผู้ขาย ของทั้งสองฝั่ง)
func inputVatRows(companyID uuid.UUID, start, end time.Time, supplierID *uuid.UUID) (string, []any) {
	cond := "company_id = ? AND status = 'posted' AND total_vat <> 0 AND posting_date BETWEEN ? AND ?"
	args := []any{companyID, start, end}
	if supplierID != nil {
		cond += " AND supplier_id = ?"
		args = append(args, *supplierID)
	}
	q := `WITH x AS (
		SELECT d.posting_date AS doc_date, d.id AS document_id, d.document_no, 'invoice' AS kind,
		       d.supplier_invoice_no AS tax_invoice_no, d.invoice_date AS tax_invoice_date, '' AS reference_no,
		       d.supplier_id, s.code AS supplier_code, d.bill_from_name AS supplier_name, d.bill_from_tax_id AS tax_id,
		       d.bill_from_branch_no AS branch_no, d.total_net AS base, d.total_vat AS tax
		  FROM (SELECT * FROM purchasing_invoices WHERE ` + cond + `) AS d
		  JOIN purchasing_suppliers AS s ON s.id = d.supplier_id
		UNION ALL
		SELECT c.posting_date, c.id, c.document_no, 'credit_note',
		       c.supplier_credit_note_no, c.credit_note_date, i.supplier_invoice_no,
		       c.supplier_id, s.code, c.bill_from_name, c.bill_from_tax_id,
		       c.bill_from_branch_no, -c.total_net, -c.total_vat
		  FROM (SELECT * FROM purchasing_credit_notes WHERE ` + cond + `) AS c
		  JOIN purchasing_invoices AS i ON i.id = c.invoice_id
		  JOIN purchasing_suppliers AS s ON s.id = c.supplier_id)`
	return q, append(append([]any{}, args...), args...)
}

type inputVatRow struct {
	DocDate        time.Time
	DocumentID     uuid.UUID
	DocumentNo     string
	Kind           string
	TaxInvoiceNo   string
	TaxInvoiceDate time.Time
	ReferenceNo    string
	SupplierID     uuid.UUID
	SupplierCode   string
	SupplierName   string
	TaxID          string
	BranchNo       string
	Base           decimal.Decimal
	Tax            decimal.Decimal
	Total          int64
	TotalBase      decimal.Decimal
	TotalTax       decimal.Decimal
}

// InputVat — 1 query: แถวตามเอกสาร + ยอดรวมทุกแถวด้วย window function (+1 COUNT เมื่อขอหน้าที่เกินข้อมูล)
func (r *ReportRepository) InputVat(ctx context.Context, companyID vo.CompanyID, f dto.InputVatFilter) (listing.Result[dto.InputVatRow], dto.InputVatRow, error) {
	order, err := database.OrderBy(f.Sort, map[string]string{"date": "x.doc_date", "document_no": "x.document_no", "supplier_name": "x.supplier_name"}, "x.document_id")
	if err != nil {
		return listing.Result[dto.InputVatRow]{}, dto.InputVatRow{}, err
	}
	var sup *uuid.UUID
	if f.SupplierID != nil {
		u := uuidOf(*f.SupplierID)
		sup = &u
	}
	cte, args := inputVatRows(uuidOf(companyID), f.Start, f.End, sup)
	var rows []inputVatRow
	query := cte + ` SELECT x.*, COUNT(*) OVER () AS total, SUM(x.base) OVER () AS total_base, SUM(x.tax) OVER () AS total_tax
		FROM x ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	if err := database.Conn(ctx, r.db).Raw(query, append(args, f.Page.Limit(), f.Page.Offset())...).Scan(&rows).Error; err != nil {
		return listing.Result[dto.InputVatRow]{}, dto.InputVatRow{}, err
	}
	res := listing.Result[dto.InputVatRow]{Items: make([]dto.InputVatRow, len(rows)), Page: f.Page}
	totals := dto.InputVatRow{Base: decimal.Zero, Tax: decimal.Zero}
	for i, x := range rows {
		res.Items[i] = dto.InputVatRow{
			Date: gp.DateUTC(x.DocDate), DocumentID: x.DocumentID.String(), DocumentNo: x.DocumentNo, Kind: x.Kind,
			TaxInvoiceNo: x.TaxInvoiceNo, TaxInvoiceDate: gp.DateUTC(x.TaxInvoiceDate), ReferenceNo: x.ReferenceNo,
			Supplier: dto.Ref{ID: x.SupplierID.String(), Code: x.SupplierCode, Name: x.SupplierName},
			TaxID:    x.TaxID, BranchNo: x.BranchNo, Base: x.Base, Tax: x.Tax,
		}
	}
	if len(rows) > 0 {
		res.Total, totals.Base, totals.Tax = rows[0].Total, rows[0].TotalBase, rows[0].TotalTax
	} else if f.Page.Offset() > 0 {
		var t struct {
			Total     int64
			TotalBase decimal.Decimal
			TotalTax  decimal.Decimal
		}
		if err := database.Conn(ctx, r.db).Raw(cte+` SELECT COUNT(*) AS total, COALESCE(SUM(base), 0) AS total_base, COALESCE(SUM(tax), 0) AS total_tax FROM x`, args...).
			Scan(&t).Error; err != nil {
			return listing.Result[dto.InputVatRow]{}, dto.InputVatRow{}, err
		}
		res.Total, totals.Base, totals.Tax = t.Total, t.TotalBase, t.TotalTax
	}
	return res, totals, nil
}

// VatTotals — 1 query: ยอดรวมภาษีซื้อ (ใบแจ้งหนี้ - ใบลดหนี้) ของช่วงวันที่ สำหรับสรุป ภ.พ.30
func (r *ReportRepository) VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error) {
	cte, args := inputVatRows(uuidOf(companyID), start, end, nil)
	var t struct {
		Base      decimal.Decimal
		Tax       decimal.Decimal
		Documents int
	}
	err := database.Conn(ctx, r.db).Raw(cte+` SELECT COALESCE(SUM(base), 0) AS base, COALESCE(SUM(tax), 0) AS tax, COUNT(*) AS documents FROM x`, args...).
		Scan(&t).Error
	return dto.VatTotals{Base: t.Base, Tax: t.Tax, Documents: t.Documents}, err
}
