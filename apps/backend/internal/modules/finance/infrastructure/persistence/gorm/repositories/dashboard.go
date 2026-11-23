package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/finance/application/dto"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	gp "apexis/backend/internal/modules/finance/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
)

// ActiveCashAccounts — 1 query (index: company_id + code)
func (r *ReportRepository) ActiveCashAccounts(ctx context.Context, companyID vo.CompanyID) ([]dto.DashboardCashAccount, error) {
	var rows []struct {
		ID          uuid.UUID
		Code        string
		Name        string
		Type        string
		GLAccountID uuid.UUID `gorm:"column:gl_account_id"`
	}
	err := database.Conn(ctx, r.db).Table("finance_cash_accounts").Select("id, code, name, type, gl_account_id").
		Where("company_id = ? AND status = 'active' AND deleted_at IS NULL", uuidOf(companyID)).Order("code").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.DashboardCashAccount, len(rows))
	for i, x := range rows {
		out[i] = dto.DashboardCashAccount{Account: dto.Ref{ID: x.ID.String(), Code: x.Code, Name: x.Name}, Type: x.Type,
			GLAccountID: vo.GLAccountID(x.GLAccountID), Balance: decimal.Zero}
	}
	return out, nil
}

// flowDocs: เอกสารรับ / จ่าย / โอนที่ยังไม่ยกเลิกในช่วง [?, ?] — 3 ชุด x (company_id, start, end)
const flowDocs = `
	SELECT 'r' AS k, doc_date, total_received AS amount, total_wht AS wht FROM finance_receipts
		WHERE company_id = ? AND status = 'posted' AND doc_date BETWEEN ? AND ?
	UNION ALL
	SELECT 'p', doc_date, total_paid, total_wht FROM finance_payments
		WHERE company_id = ? AND status = 'posted' AND doc_date BETWEEN ? AND ?
	UNION ALL
	SELECT 't', doc_date, amount, 0 FROM finance_transfers
		WHERE company_id = ? AND status = 'posted' AND doc_date BETWEEN ? AND ?`

func flowArgs(companyID vo.CompanyID, from, to time.Time) []any {
	c := uuidOf(companyID)
	return []any{c, from, to, c, from, to, c, from, to}
}

// FlowTotals — 1 query: UNION ALL ทั้ง 3 ตาราง แล้วรวมด้วย FILTER (index: company_id + doc_date ของแต่ละตาราง)
func (r *ReportRepository) FlowTotals(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (dto.FlowTotals, error) {
	var row struct {
		Received      decimal.Decimal
		Paid          decimal.Decimal
		TransferCount int64
		WhtDeducted   decimal.Decimal
		WhtSuffered   decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Raw(`SELECT
			COALESCE(SUM(amount) FILTER (WHERE k = 'r'), 0) AS received,
			COALESCE(SUM(amount) FILTER (WHERE k = 'p'), 0) AS paid,
			COUNT(*) FILTER (WHERE k = 't') AS transfer_count,
			COALESCE(SUM(wht) FILTER (WHERE k = 'p'), 0) AS wht_deducted,
			COALESCE(SUM(wht) FILTER (WHERE k = 'r'), 0) AS wht_suffered
		FROM (`+flowDocs+`) AS d`, flowArgs(companyID, start, end)...).Scan(&row).Error
	return dto.FlowTotals{Received: row.Received, Paid: row.Paid, TransferCount: row.TransferCount,
		WhtDeducted: row.WhtDeducted, WhtSuffered: row.WhtSuffered}, err
}

// MonthlyFlows — 1 query: GROUP BY เดือน (โอนเงินไม่นับเป็นเงินเข้า-ออกของบริษัท)
func (r *ReportRepository) MonthlyFlows(ctx context.Context, companyID vo.CompanyID, from, to time.Time) ([]dto.MonthlyFlow, error) {
	var rows []struct {
		Month    time.Time
		Received decimal.Decimal
		Paid     decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Raw(`SELECT DATE_TRUNC('month', doc_date)::date AS month,
			COALESCE(SUM(amount) FILTER (WHERE k = 'r'), 0) AS received,
			COALESCE(SUM(amount) FILTER (WHERE k = 'p'), 0) AS paid
		FROM (`+flowDocs+`) AS d WHERE k <> 't' GROUP BY 1 ORDER BY 1`, flowArgs(companyID, from, to)...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.MonthlyFlow, len(rows))
	for i, x := range rows {
		out[i] = dto.MonthlyFlow{Month: gp.DateUTC(x.Month), Received: x.Received, Paid: x.Paid}
	}
	return out, nil
}

// RecentDocuments — 1 query: UNION ALL 3 ตาราง เรียงวันที่ / เวลาสร้างล่าสุด (โอนเงิน: party = บัญชีต้นทาง → ปลายทาง)
func (r *ReportRepository) RecentDocuments(ctx context.Context, companyID vo.CompanyID, limit int) ([]dto.RecentDocument, error) {
	c := uuidOf(companyID)
	var rows []struct {
		Kind       string
		ID         uuid.UUID
		DocumentNo string
		DocDate    time.Time
		Party      string
		Amount     decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Raw(`SELECT kind, id, document_no, doc_date, party, amount FROM (
			SELECT 'receipt' AS kind, id, document_no, doc_date, party_name AS party, total_received AS amount, created_at
				FROM finance_receipts WHERE company_id = ? AND status = 'posted'
			UNION ALL
			SELECT 'payment', id, document_no, doc_date, party_name, total_paid, created_at
				FROM finance_payments WHERE company_id = ? AND status = 'posted'
			UNION ALL
			SELECT 'transfer', t.id, t.document_no, t.doc_date, fa.name || ' → ' || ta.name, t.amount, t.created_at
				FROM finance_transfers AS t
				JOIN finance_cash_accounts AS fa ON fa.id = t.from_account_id
				JOIN finance_cash_accounts AS ta ON ta.id = t.to_account_id
				WHERE t.company_id = ? AND t.status = 'posted'
		) AS d ORDER BY doc_date DESC, created_at DESC LIMIT ?`, c, c, c, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RecentDocument, len(rows))
	for i, x := range rows {
		out[i] = dto.RecentDocument{Kind: x.Kind, ID: x.ID.String(), DocumentNo: x.DocumentNo, Date: gp.DateUTC(x.DocDate), Party: x.Party, Amount: x.Amount}
	}
	return out, nil
}
