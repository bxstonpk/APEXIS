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

// inputVatRows: แถวที่ใช้สิทธิภาษีซื้อได้ (ได้รับใบกำกับภาษีแล้ว) ที่ยังไม่ยกเลิก, วันที่ลงบัญชีอยู่ในช่วง
//
//	invoice                 = ซื้อสินค้า (ใบแจ้งหนี้ของผู้ขาย = ใบกำกับภาษี)
//	service_invoice_with_tx = ซื้อบริการที่ได้รับใบกำกับพร้อมใบแจ้งหนี้
//	tax_invoice             = ใบกำกับที่ได้รับภายหลังของซื้อบริการ (เดือนภาษี = วันที่ลงบัญชีของการบันทึกใบกำกับ; อ้างอิง = เลขที่ซื้อบริการ)
//	credit_note             = ใบลดหนี้ที่ลดภาษีซื้อ (ติดลบ) — ใบลดหนี้ของภาษีที่ยังไม่ถึงกำหนดไม่นับ
//
// ซื้อบริการที่ยังรอใบกำกับไม่อยู่ในรายงาน (อยู่ในรายงานภาษีซื้อยังไม่ถึงกำหนด)
// ชื่อ / เลขผู้เสียภาษี / สาขาของผู้ขายตามใบกำกับภาษี ณ วันบันทึก (bill_from_*) — ไม่ใช่ทะเบียนผู้ขายปัจจุบัน
// คืน SQL ของ CTE x + args (บริษัท / ช่วงวันที่ / ผู้ขาย ของทั้งสามฝั่ง)
func inputVatRows(companyID uuid.UUID, start, end time.Time, supplierID *uuid.UUID) (string, []any) {
	cond := "company_id = ? AND status = 'posted' AND posting_date BETWEEN ? AND ?"
	args := []any{companyID, start, end}
	if supplierID != nil {
		cond += " AND supplier_id = ?"
		args = append(args, *supplierID)
	}
	q := `WITH x AS (
		SELECT d.posting_date AS doc_date, d.id AS document_id, d.document_no,
		       CASE WHEN d.kind = 'service' THEN 'service_invoice_with_tx' ELSE 'invoice' END AS kind,
		       CASE WHEN d.kind = 'service' THEN d.tax_invoice_no ELSE d.supplier_invoice_no END AS tax_invoice_no,
		       CASE WHEN d.kind = 'service' THEN COALESCE(d.tax_invoice_date, d.invoice_date) ELSE d.invoice_date END AS tax_invoice_date,
		       '' AS reference_no,
		       d.supplier_id, s.code AS supplier_code, d.bill_from_name AS supplier_name, d.bill_from_tax_id AS tax_id,
		       d.bill_from_branch_no AS branch_no, d.total_net AS base, d.total_vat AS tax
		  FROM (SELECT * FROM purchasing_invoices WHERE ` + cond + ` AND total_vat <> 0 AND tax_invoice_received) AS d
		  JOIN purchasing_suppliers AS s ON s.id = d.supplier_id
		UNION ALL
		SELECT t.posting_date, t.invoice_id, i.document_no, 'tax_invoice',
		       t.tax_invoice_no, t.tax_invoice_date, i.document_no,
		       t.supplier_id, s.code, i.bill_from_name, i.bill_from_tax_id,
		       i.bill_from_branch_no, t.base, t.vat
		  FROM (SELECT * FROM purchasing_tax_invoices WHERE ` + cond + `) AS t
		  JOIN purchasing_invoices AS i ON i.id = t.invoice_id
		  JOIN purchasing_suppliers AS s ON s.id = t.supplier_id
		UNION ALL
		SELECT c.posting_date, c.id, c.document_no, 'credit_note',
		       c.supplier_credit_note_no, c.credit_note_date, i.supplier_invoice_no,
		       c.supplier_id, s.code, c.bill_from_name, c.bill_from_tax_id,
		       c.bill_from_branch_no, -c.total_net, -c.total_vat
		  FROM (SELECT * FROM purchasing_credit_notes WHERE ` + cond + ` AND total_vat <> 0 AND vat_kind <> 'undue') AS c
		  JOIN purchasing_invoices AS i ON i.id = c.invoice_id
		  JOIN purchasing_suppliers AS s ON s.id = c.supplier_id)`
	return q, append(append(append([]any{}, args...), args...), args...)
}

// UndueInputVat — 1 query: ซื้อบริการที่ภาษีซื้อยังรอใบกำกับ ณ วันที่ (ยอดคงเหลือของบัญชีภาษีซื้อยังไม่ถึงกำหนด)
// ภาษีทั้งใบ - ได้รับใบกำกับแล้ว (บันทึกภายในวันที่) - ลดหนี้ส่วนที่ยังไม่ถึงกำหนด (ภายในวันที่) > 0
func (r *ReportRepository) UndueInputVat(ctx context.Context, companyID vo.CompanyID, f dto.UndueInputVatFilter) (listing.Result[dto.UndueInputVatRow], dto.UndueInputVatRow, error) {
	args := []any{f.End, f.End, uuidOf(companyID), f.End}
	cond := ""
	if f.SupplierID != nil {
		cond = " AND d.supplier_id = ?"
		args = append(args, uuidOf(*f.SupplierID))
	}
	cte := `WITH x AS (
		SELECT d.id, d.document_no, d.supplier_invoice_no, d.posting_date, d.supplier_id, s.code AS supplier_code, d.bill_from_name AS supplier_name,
		       d.total_gross AS gross, d.total_vat AS vat_total,
		       COALESCE((SELECT SUM(t.vat) FROM purchasing_tax_invoices AS t WHERE t.invoice_id = d.id AND t.status = 'posted' AND t.posting_date <= ?), 0) AS vat_recognized,
		       COALESCE((SELECT SUM(c.total_vat) FROM purchasing_credit_notes AS c WHERE c.invoice_id = d.id AND c.status = 'posted' AND c.vat_kind = 'undue' AND c.posting_date <= ?), 0) AS vat_credited
		  FROM purchasing_invoices AS d
		  JOIN purchasing_suppliers AS s ON s.id = d.supplier_id
		 WHERE d.company_id = ? AND d.status = 'posted' AND d.kind = 'service' AND NOT d.tax_invoice_received
		   AND d.total_vat <> 0 AND d.posting_date <= ?` + cond + `)
		, y AS (SELECT x.*, x.vat_total - x.vat_recognized - x.vat_credited AS vat_undue FROM x)`
	var rows []struct {
		ID                uuid.UUID
		DocumentNo        string
		SupplierInvoiceNo string
		PostingDate       time.Time
		SupplierID        uuid.UUID
		SupplierCode      string
		SupplierName      string
		Gross             decimal.Decimal
		VatTotal          decimal.Decimal
		VatRecognized     decimal.Decimal
		VatCredited       decimal.Decimal
		VatUndue          decimal.Decimal
		Total             int64
		TGross            decimal.Decimal
		TVat              decimal.Decimal
		TRecognized       decimal.Decimal
		TCredited         decimal.Decimal
		TUndue            decimal.Decimal
	}
	q := cte + ` SELECT y.*, COUNT(*) OVER () AS total, SUM(y.gross) OVER () AS t_gross, SUM(y.vat_total) OVER () AS t_vat,
		SUM(y.vat_recognized) OVER () AS t_recognized, SUM(y.vat_credited) OVER () AS t_credited, SUM(y.vat_undue) OVER () AS t_undue
		FROM y WHERE y.vat_undue > 0 ORDER BY y.posting_date, y.document_no LIMIT ? OFFSET ?`
	if err := database.Conn(ctx, r.db).Raw(q, append(args, f.Page.Limit(), f.Page.Offset())...).Scan(&rows).Error; err != nil {
		return listing.Result[dto.UndueInputVatRow]{}, dto.UndueInputVatRow{}, err
	}
	res := listing.Result[dto.UndueInputVatRow]{Items: make([]dto.UndueInputVatRow, len(rows)), Page: f.Page}
	totals := dto.UndueInputVatRow{Gross: decimal.Zero, VatTotal: decimal.Zero, VatRecognized: decimal.Zero, VatCredited: decimal.Zero, VatUndue: decimal.Zero}
	for i, x := range rows {
		res.Items[i] = dto.UndueInputVatRow{
			Invoice: dto.Ref{ID: x.ID.String(), Code: x.DocumentNo, Name: x.SupplierInvoiceNo}, Date: gp.DateUTC(x.PostingDate),
			Supplier: dto.Ref{ID: x.SupplierID.String(), Code: x.SupplierCode, Name: x.SupplierName},
			Gross: x.Gross, VatTotal: x.VatTotal, VatRecognized: x.VatRecognized, VatCredited: x.VatCredited, VatUndue: x.VatUndue,
		}
	}
	if len(rows) > 0 {
		x := rows[0]
		res.Total = x.Total
		totals = dto.UndueInputVatRow{Gross: x.TGross, VatTotal: x.TVat, VatRecognized: x.TRecognized, VatCredited: x.TCredited, VatUndue: x.TUndue}
	}
	return res, totals, nil
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
