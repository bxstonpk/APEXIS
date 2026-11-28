package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/application/dto"
	ports "apexis/backend/internal/modules/ledger/application/ports/repositories"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	gp "apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.ReportRepository = (*ReportRepository)(nil)

type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) *ReportRepository { return &ReportRepository{db: db} }

// รายการที่มีผลต่อยอดบัญชี (การกลับรายการเป็นรายการใหม่อีกใบ ต้นฉบับจึงยังนับอยู่)
var affecting = []vo.EntryStatus{vo.EntryPosted, vo.EntryReversed}

// SumByAccount — 1 query: GROUP BY บัญชี พร้อม SUM(CASE ...) 2 column (เดบิต / เครดิต) ต่อ bucket
//
//	SELECT l.account_id,
//	       SUM(CASE WHEN e.entry_date BETWEEN ? AND ? THEN l.debit ELSE 0 END) AS d0, ... AS c0, ...
//	FROM ledger_journal_lines l JOIN ledger_journal_entries e ON e.id = l.entry_id
//	WHERE e.company_id = ? AND e.status IN (...) AND e.entry_date <= <วันสุดท้ายของทุก bucket>
//	GROUP BY l.account_id
func (r *ReportRepository) SumByAccount(ctx context.Context, companyID vo.CompanyID, accountID *vo.AccountID, buckets []ports.Bucket) (map[vo.AccountID][]ports.Sum, error) {
	if len(buckets) == 0 {
		return map[vo.AccountID][]ports.Sum{}, nil
	}
	cols := []string{"l.account_id"}
	var args []any
	maxTo := buckets[0].To
	for i, b := range buckets {
		cond := "e.entry_date <= ?"
		bargs := []any{b.To}
		if b.From != nil {
			cond = "e.entry_date BETWEEN ? AND ?"
			bargs = []any{*b.From, b.To}
		}
		if b.ExcludeClosing {
			cond += " AND NOT e.is_closing"
		}
		cols = append(cols,
			fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN l.debit END), 0) AS d%d", cond, i),
			fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN l.credit END), 0) AS c%d", cond, i))
		args = append(args, bargs...)
		args = append(args, bargs...)
		if b.To.After(maxTo) {
			maxTo = b.To
		}
	}
	where := "e.company_id = ? AND e.status IN ? AND e.entry_date <= ?"
	args = append(args, uuidOf(companyID), affecting, maxTo)
	if accountID != nil {
		where += " AND l.account_id = ?"
		args = append(args, uuidOf(*accountID))
	}
	sql := "SELECT " + strings.Join(cols, ", ") +
		" FROM ledger_journal_lines AS l JOIN ledger_journal_entries AS e ON e.id = l.entry_id" +
		" WHERE " + where + " GROUP BY l.account_id"

	rows, err := database.Conn(ctx, r.db).Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[vo.AccountID][]ports.Sum{}
	for rows.Next() {
		var id uuid.UUID
		amounts := make([]decimal.Decimal, 2*len(buckets))
		dest := []any{&id}
		for i := range amounts {
			dest = append(dest, &amounts[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		sums := make([]ports.Sum, len(buckets))
		for i := range sums {
			sums[i] = ports.Sum{Debit: amounts[2*i], Credit: amounts[2*i+1]}
		}
		out[vo.AccountID(id)] = sums
	}
	return out, rows.Err()
}

// AccountLines — 1 query: ยอดสะสมคำนวณด้วย window function ก่อนตัดหน้า จึงถูกต้องทุกหน้า
// (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล)
func (r *ReportRepository) AccountLines(ctx context.Context, companyID vo.CompanyID, accountID vo.AccountID, start, end time.Time, page listing.Page) ([]dto.LedgerLine, int64, error) {
	inner := database.Conn(ctx, r.db).Table("ledger_journal_lines AS l").
		Joins("JOIN ledger_journal_entries AS e ON e.id = l.entry_id").
		Where("e.company_id = ? AND e.status IN ? AND l.account_id = ? AND e.entry_date BETWEEN ? AND ?",
			uuidOf(companyID), affecting, uuidOf(accountID), start, end)

	type row struct {
		EntryID     uuid.UUID
		EntryNo     string
		EntryDate   time.Time
		LineNo      int
		Description string
		Reference   string
		Debit       decimal.Decimal
		Credit      decimal.Decimal
		Running     decimal.Decimal
		Total       int64
	}
	var rows []row
	err := database.Conn(ctx, r.db).Table("(?) AS t", inner.Select(`e.id AS entry_id, e.entry_no, e.entry_date, l.line_no,
			COALESCE(NULLIF(l.description, ''), e.description) AS description, e.reference, l.debit, l.credit,
			SUM(l.debit - l.credit) OVER (ORDER BY e.entry_date, e.entry_no, l.line_no ROWS UNBOUNDED PRECEDING) AS running,
			COUNT(*) OVER () AS total`)).
		Order("entry_date, entry_no, line_no").
		Limit(page.Limit()).Offset(page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var total int64
	switch {
	case len(rows) > 0:
		total = rows[0].Total
	case page.Offset() > 0:
		if err := inner.Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}
	out := make([]dto.LedgerLine, len(rows))
	for i, x := range rows {
		out[i] = dto.LedgerLine{
			EntryID: x.EntryID.String(), EntryNo: x.EntryNo, Date: gp.DateUTC(x.EntryDate),
			Description: x.Description, Reference: x.Reference,
			Debit: x.Debit, Credit: x.Credit, Balance: x.Running,
		}
	}
	return out, total, nil
}

// EntryActivity — 1 query: COUNT ... FILTER ตามสถานะ (index: company_id + entry_date)
func (r *ReportRepository) EntryActivity(ctx context.Context, companyID vo.CompanyID, start, end time.Time) (int64, int64, error) {
	var row struct {
		Drafts int64
		Posted int64
	}
	err := database.Conn(ctx, r.db).Raw(`SELECT
			COUNT(*) FILTER (WHERE status = ?) AS drafts,
			COUNT(*) FILTER (WHERE status IN ?) AS posted
		FROM ledger_journal_entries WHERE company_id = ? AND entry_date BETWEEN ? AND ?`,
		vo.EntryDraft, affecting, uuidOf(companyID), start, end).Scan(&row).Error
	return row.Drafts, row.Posted, err
}

// RecentEntries — 1 query: ยอดเดบิตรวมเก็บในหัวรายการแล้ว (total_debit) จึงไม่ต้อง JOIN บรรทัด
func (r *ReportRepository) RecentEntries(ctx context.Context, companyID vo.CompanyID, limit int) ([]dto.RecentEntry, error) {
	var rows []struct {
		ID          uuid.UUID
		EntryNo     *string
		EntryDate   time.Time
		Description string
		TotalDebit  decimal.Decimal
	}
	err := database.Conn(ctx, r.db).Table("ledger_journal_entries").
		Select("id, entry_no, entry_date, description, total_debit").
		Where("company_id = ? AND status IN ?", uuidOf(companyID), affecting).
		Order("entry_date DESC, entry_no DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RecentEntry, len(rows))
	for i, x := range rows {
		no := ""
		if x.EntryNo != nil {
			no = *x.EntryNo
		}
		out[i] = dto.RecentEntry{ID: x.ID.String(), EntryNo: no, Date: gp.DateUTC(x.EntryDate), Description: x.Description, Amount: x.TotalDebit}
	}
	return out, nil
}
