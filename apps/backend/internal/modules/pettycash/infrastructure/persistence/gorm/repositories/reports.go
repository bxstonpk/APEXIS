package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/pettycash/application/dto"
	"apexis/backend/internal/modules/pettycash/domain/entities"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
	"apexis/backend/internal/platform/database"
)

type ReportRepository struct {
	db       *gorm.DB
	vouchers *VoucherRepository
}

func NewReportRepository(db *gorm.DB, vouchers *VoucherRepository) *ReportRepository {
	return &ReportRepository{db: db, vouchers: vouchers}
}

// Register — หน้า 1 (+COUNT) + ยอดรวม 1 (ทั้งช่วง ไม่ใช่เฉพาะหน้า)
func (r *ReportRepository) Register(ctx context.Context, companyID vo.CompanyID, f dto.RegisterFilter) (dto.RegisterReport, error) {
	var out dto.RegisterReport
	base := func() *gorm.DB {
		q := r.vouchers.baseQuery(ctx, companyID).Where("v.doc_date BETWEEN ? AND ?", f.Start, f.End)
		if f.FundID != nil {
			q = q.Where("v.fund_id = ?", uuidOf(*f.FundID))
		}
		if len(f.Status) > 0 {
			q = q.Where("v.status IN ?", strs(f.Status))
		} else {
			q = q.Where("v.status <> 'cancelled'")
		}
		return q
	}
	rows, total, err := database.FetchPage(base, voucherColumns, "v.doc_date ASC, v.document_no ASC", f.Page, func(x voucherRow) int64 { return x.Total })
	if err != nil {
		return out, err
	}
	out.Rows = toResult(rows, total, f.Page, voucherRow.dto)
	var t struct{ Net, Vat, Gross decimal.Decimal }
	err = base().Select("COALESCE(SUM(v.total_net), 0) AS net, COALESCE(SUM(v.total_vat), 0) AS vat, COALESCE(SUM(v.total_gross), 0) AS gross").Scan(&t).Error
	out.Totals = entities.Totals{Net: t.Net, Vat: t.Vat, Gross: t.Gross}
	return out, err
}

func (r *ReportRepository) FundRows(ctx context.Context, companyID vo.CompanyID) ([]dto.FundBalanceRow, error) {
	var rows []struct {
		ID            uuid.UUID
		Code          string
		Name          string
		CustodianID   uuid.UUID
		ImprestAmount decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("petty_cash_funds").Select("id, code, name, custodian_id, imprest_amount").
		Where("company_id = ?", uuidOf(companyID)).Order("code").Scan(&rows).Error
	out := make([]dto.FundBalanceRow, len(rows))
	for i, x := range rows {
		out[i] = dto.FundBalanceRow{Fund: dto.Ref{ID: x.ID.String(), Code: x.Code, Name: x.Name}, Custodian: dto.Ref{ID: x.CustodianID.String()},
			ImprestAmount: x.ImprestAmount}
	}
	return out, err
}

// vatBase: ใบสำคัญจ่ายที่ชดเชยแล้ว (การเงินจ่ายแล้ว) มี VAT และวันที่ใบกำกับภาษีอยู่ในช่วง
func (r *ReportRepository) vatBase(ctx context.Context, companyID vo.CompanyID, start, end time.Time) *gorm.DB {
	return database.Conn(ctx, r.db).Table("petty_cash_vouchers AS v").
		Where("v.company_id = ? AND v.status = 'replenished' AND v.total_vat > 0 AND v.tax_invoice_date BETWEEN ? AND ?", uuidOf(companyID), start, end)
}

func (r *ReportRepository) InputVat(ctx context.Context, companyID vo.CompanyID, f dto.InputVatFilter) (dto.InputVatReport, error) {
	var out dto.InputVatReport
	type row struct {
		ID             uuid.UUID
		DocumentNo     string
		TaxInvoiceDate time.Time
		TaxInvoiceNo   string
		PayeeName      string
		PayeeTaxID     string
		PayeeBranchNo  string
		TotalNet       decimal.Decimal
		TotalVat       decimal.Decimal
		Total          int64
	}
	base := func() *gorm.DB { return r.vatBase(ctx, companyID, f.Start, f.End) }
	rows, total, err := database.FetchPage(base, "v.id, v.document_no, v.tax_invoice_date, v.tax_invoice_no, v.payee_name, v.payee_tax_id, v.payee_branch_no, v.total_net, v.total_vat",
		"v.tax_invoice_date ASC, v.document_no ASC", f.Page, func(x row) int64 { return x.Total })
	if err != nil {
		return out, err
	}
	out.Rows = toResult(rows, total, f.Page, func(x row) dto.InputVatRow {
		return dto.InputVatRow{Date: dateOnly(x.TaxInvoiceDate), TaxInvoiceNo: x.TaxInvoiceNo, Voucher: dto.Ref{ID: x.ID.String(), Code: x.DocumentNo},
			PayeeName: x.PayeeName, TaxID: x.PayeeTaxID, BranchNo: x.PayeeBranchNo, Base: x.TotalNet, Vat: x.TotalVat}
	})
	t, err := r.VatTotals(ctx, companyID, f.Start, f.End)
	out.Totals.Base, out.Totals.Vat = t.Base, t.Tax
	return out, err
}

func (r *ReportRepository) VatTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.VatTotals, error) {
	var t struct {
		Base decimal.Decimal
		Tax  decimal.Decimal
		N    int
	}
	err := r.vatBase(ctx, companyID, start, end).
		Select("COALESCE(SUM(v.total_net), 0) AS base, COALESCE(SUM(v.total_vat), 0) AS tax, COUNT(*) AS n").Scan(&t).Error
	return dto.VatTotals{Base: t.Base, Tax: t.Tax, Documents: t.N}, err
}
