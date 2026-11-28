package usecases

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	"apexis/backend/internal/modules/ledger/domain/services"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
)

// YearEndUseCases คือการปิดบัญชีสิ้นปี / เปิดปีใหม่ และค่าตั้งของ ledger
//
// แนวทาง (ตัดสินใจ 2026-11-28):
//   - ปิดบัญชีได้ภายใน 120 วันหลังสิ้นปี — ระหว่างนั้นปีใหม่ลงรายการได้ตามปกติ ไม่ขึ้นต่อกัน
//   - งวด 1–12 ของปีปิดได้ตามปกติ; รายการปรับปรุงสิ้นปีลงใน "งวดปรับปรุง" (งวดที่ 13 ของ ledger —
//     รายการที่ adjusting_period = true ลงวันสุดท้ายของปี) ซึ่งเปิดอยู่จนปิดบัญชี
//   - รายการปิดบัญชีเป็นรายการสุดท้ายในงวดปรับปรุง: ปิดรายได้ / ค่าใช้จ่ายของทั้งปี (รวมงวดปรับปรุง)
//     เข้ากำไรสะสม — ปิดแล้วทั้งปีล็อก (รวมงวดปรับปรุง)
//   - ปิด / เปิดปี N ไม่แตะเอกสารหรือรายการของปี N+1; เปิดปี N ไม่ได้เฉพาะเมื่อปี N+1 ปิดแล้ว
type YearEndUseCases struct {
	base
	entries  *EntryUseCases
	accounts repositories.AccountRepository
	journals repositories.JournalRepository
	reports  repositories.ReportRepository
	yearEnds repositories.YearEndRepository
	settings repositories.SettingsRepository
}

func closeDeadline(end time.Time) time.Time { return end.AddDate(0, 0, dto.CloseDeadlineDays) }

// List: ทุกปีบัญชี (ปีล่าสุดก่อน) พร้อมสถานะการปิดบัญชีและกำหนดเวลา — 2 query (ปีบัญชี + แถวการปิด)
func (uc *YearEndUseCases) List(ctx context.Context, a dto.Actor) ([]dto.YearEnd, error) {
	return read(ctx, uc.base, a, vo.PermReportRead, func() ([]dto.YearEnd, error) {
		years, recs, err := uc.load(ctx, a.CompanyID)
		if err != nil {
			return nil, err
		}
		out := make([]dto.YearEnd, 0, len(years))
		for i := len(years) - 1; i >= 0; i-- {
			out = append(out, uc.status(years[i], recs, nil, true))
		}
		return out, nil
	})
}

// Get: สถานะของปีพร้อมเงื่อนไขก่อนปิด (งวดที่ยังเปิด, บัญชีพักที่มียอด, ค่าตั้ง)
func (uc *YearEndUseCases) Get(ctx context.Context, in dto.YearEndInput) (dto.YearEnd, error) {
	return read(ctx, uc.base, in.Actor, vo.PermReportRead, func() (dto.YearEnd, error) {
		years, recs, err := uc.load(ctx, in.CompanyID)
		if err != nil {
			return dto.YearEnd{}, err
		}
		y := ports.FindYear(years, in.FiscalYear)
		if y == nil {
			return dto.YearEnd{}, domain.ErrFiscalYearNotFound
		}
		suspense, err := uc.suspense(ctx, in.CompanyID, y.End)
		if err != nil {
			return dto.YearEnd{}, err
		}
		s, err := uc.loadSettings(ctx, in.CompanyID)
		if err != nil {
			return dto.YearEnd{}, err
		}
		out := uc.status(*y, recs, suspense, s.Ready())
		return out, nil
	})
}

func (uc *YearEndUseCases) load(ctx context.Context, companyID vo.CompanyID) ([]ports.FiscalYear, map[int]dto.YearEndRecord, error) {
	years, err := uc.org.FiscalYears(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}
	list, err := uc.yearEnds.List(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}
	recs := make(map[int]dto.YearEndRecord, len(list))
	for _, r := range list {
		recs[r.FiscalYear] = r
	}
	return years, recs, nil
}

func (uc *YearEndUseCases) status(y ports.FiscalYear, recs map[int]dto.YearEndRecord, suspense []dto.SuspenseBalance, settingsReady bool) dto.YearEnd {
	out := dto.YearEnd{
		FiscalYear: y.Year, Start: y.Start, End: y.End, Deadline: closeDeadline(y.End),
		Status: "open", Periods: y.Periods, OpenPeriods: y.OpenPeriods, Suspense: suspense, SettingsReady: settingsReady,
	}
	if suspense == nil {
		out.Suspense = []dto.SuspenseBalance{}
	}
	if r, ok := recs[y.Year]; ok {
		rec := r
		out.Record = &rec
		if r.Status == string(entities.YearClosed) {
			out.Status = "closed"
		}
	}
	today := uc.now().Truncate(24 * time.Hour)
	switch {
	case out.Status == "closed":
		out.DeadlineStatus = "closed"
		next, ok := recs[y.Year+1]
		out.CanReopen = !ok || next.Status != string(entities.YearClosed)
	case today.After(out.Deadline):
		out.DeadlineStatus = "overdue"
	default:
		out.DeadlineStatus = "on_time"
	}
	out.CanClose = out.Status == "open" && y.OpenPeriods == 0 && len(out.Suspense) == 0 && settingsReady && suspense != nil
	return out
}

// suspense: บัญชีพักที่ยังมียอด ณ วันสิ้นปี — 1 query รวมยอด + ผังบัญชี
func (uc *YearEndUseCases) suspense(ctx context.Context, companyID vo.CompanyID, end time.Time) ([]dto.SuspenseBalance, error) {
	sums, err := uc.reports.SumByAccount(ctx, companyID, nil, []repositories.Bucket{{To: end}})
	if err != nil {
		return nil, err
	}
	chart, err := uc.accounts.Chart(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := []dto.SuspenseBalance{}
	for _, a := range chart {
		if a.Type != vo.AccountSuspense {
			continue
		}
		if s, ok := sums[a.ID]; ok && !s[0].Net().IsZero() {
			out = append(out, dto.SuspenseBalance{Account: info(a), Balance: s[0].Net()})
		}
	}
	slices.SortFunc(out, func(x, y dto.SuspenseBalance) int { return compare(x.Account.Code, y.Account.Code) })
	return out, nil
}

// Close ปิดบัญชีสิ้นปี — 1 transaction: lock แถวการปิดของปี -> ตรวจเงื่อนไข -> รายการปิดบัญชี (งวดปรับปรุง) -> บันทึกสถานะ
func (uc *YearEndUseCases) Close(ctx context.Context, in dto.YearEndInput) (dto.YearEnd, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermYearEndClose, func(ctx context.Context, cs ports.CompanySettings) error {
		years, err := uc.org.FiscalYears(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		y := ports.FindYear(years, in.FiscalYear)
		if y == nil {
			return domain.ErrFiscalYearNotFound
		}
		rec, err := uc.yearEnds.FindForUpdate(ctx, in.CompanyID, y.Year)
		if err != nil {
			return err
		}
		if rec != nil && rec.IsClosed() {
			return domain.ErrYearAlreadyClosed
		}
		if y.OpenPeriods > 0 {
			return fmt.Errorf("%w: %d open period(s)", domain.ErrYearHasOpenPeriods, y.OpenPeriods)
		}
		suspense, err := uc.suspense(ctx, in.CompanyID, y.End)
		if err != nil {
			return err
		}
		if len(suspense) > 0 {
			codes := make([]string, len(suspense))
			for i, s := range suspense {
				codes[i] = s.Account.Code
			}
			return fmt.Errorf("%w: %s", domain.ErrSuspenseNotZero, strings.Join(codes, ", "))
		}
		s, err := uc.loadSettings(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		if !s.Ready() {
			return domain.ErrLedgerSettingsMissing
		}
		// รายได้ / ค่าใช้จ่ายทั้งปี (รวมงวดปรับปรุง และรายการปิด / กลับของการปิดครั้งก่อนซึ่งหักล้างกันแล้ว)
		sums, err := uc.reports.SumByAccount(ctx, in.CompanyID, nil, []repositories.Bucket{{From: &y.Start, To: y.End}})
		if err != nil {
			return err
		}
		chart, err := uc.accounts.Chart(ctx, in.CompanyID)
		if err != nil {
			return err
		}
		slices.SortFunc(chart, func(a, b services.AccountNode) int { return compare(a.Code, b.Code) })
		var balances []entities.ClosingBalance
		for _, a := range chart {
			if a.Type != vo.AccountRevenue && a.Type != vo.AccountExpense {
				continue
			}
			if sum, ok := sums[a.ID]; ok {
				balances = append(balances, entities.ClosingBalance{AccountID: a.ID, Net: sum[0].Net()})
			}
		}
		desc := "ปิดบัญชีสิ้นปีบัญชี " + strconv.Itoa(y.Year+543)
		lines, net := entities.ClosingLines(balances, *s.RetainedEarningsAccountID, s.IncomeSummaryAccountID, desc)
		var entryID *vo.EntryID
		if len(lines) > 0 {
			e, err := entities.NewEntry(in.CompanyID, in.UserID, entities.EntryData{
				JournalID: *s.ClosingJournalID, Date: y.End, Description: desc, Reference: "FY" + strconv.Itoa(y.Year),
				Lines: lines, AdjustingPeriod: true,
			}, cs.AmountDecimals)
			if err != nil {
				return err
			}
			e.MarkClosing()
			if err := uc.entries.post(ctx, in.Actor, e, nil); err != nil {
				return err
			}
			if err := uc.entries.entries.Create(ctx, e); err != nil {
				return err
			}
			id := e.ID()
			entryID = &id
		}
		if rec == nil {
			rec = entities.NewYearEnd(in.CompanyID, y.Year, y.Start, y.End)
		}
		if err := rec.Close(net, entryID, in.UserID, uc.now()); err != nil {
			return err
		}
		return uc.yearEnds.Save(ctx, rec)
	})
	if err != nil {
		return dto.YearEnd{}, err
	}
	return uc.Get(ctx, in)
}

// Reopen เปิดปีที่ปิดแล้ว: กลับรายการปิดบัญชี ณ วันสิ้นปี (งวดปรับปรุงเปิดอีกครั้ง) — ปีถัดไปต้องยังไม่ปิด
func (uc *YearEndUseCases) Reopen(ctx context.Context, in dto.YearEndInput) (dto.YearEnd, error) {
	err := uc.write(ctx, in.Actor, vo.PermYearEndClose, func(ctx context.Context) error {
		rec, err := uc.yearEnds.FindForUpdate(ctx, in.CompanyID, in.FiscalYear)
		if err != nil {
			return err
		}
		if rec == nil || !rec.IsClosed() {
			return domain.ErrYearNotClosed
		}
		next, err := uc.yearEnds.IsClosed(ctx, in.CompanyID, in.FiscalYear+1)
		if err != nil {
			return err
		}
		if next {
			return domain.ErrNextYearClosed
		}
		// เปลี่ยนสถานะก่อน เพื่อให้รายการกลับลงในงวดปรับปรุงของปีนี้ได้
		if err := rec.Reopen(nil, in.UserID, uc.now()); err != nil {
			return err
		}
		if err := uc.yearEnds.Save(ctx, rec); err != nil {
			return err
		}
		st := rec.State()
		if st.EntryID == nil {
			return nil
		}
		r, err := uc.entries.reverse(ctx, in.Actor, *st.EntryID, st.EndDate, "เปิดปีบัญชี "+strconv.Itoa(st.FiscalYear))
		if err != nil {
			return err
		}
		rec.SetReversal(r.ID())
		return uc.yearEnds.Save(ctx, rec)
	})
	if err != nil {
		return dto.YearEnd{}, err
	}
	return uc.Get(ctx, in)
}

// ---------- ค่าตั้ง ----------

// loadSettings: ค่าที่บันทึกไว้ — ยังไม่เคยบันทึก (บริษัทเดิม) = หาจากรหัสในผังมาตรฐาน
func (uc *YearEndUseCases) loadSettings(ctx context.Context, companyID vo.CompanyID) (entities.Settings, error) {
	s, found, err := uc.settings.Get(ctx, companyID)
	if err != nil || found {
		return s, err
	}
	return defaultSettings(ctx, uc.accounts, uc.journals, companyID)
}

// Settings: GET /ledger-settings
func (uc *YearEndUseCases) Settings(ctx context.Context, a dto.Actor) (dto.LedgerSettings, error) {
	return read(ctx, uc.base, a, vo.PermAccountRead, func() (dto.LedgerSettings, error) {
		s, err := uc.loadSettings(ctx, a.CompanyID)
		if err != nil {
			return dto.LedgerSettings{}, err
		}
		return uc.settingsDTO(ctx, a.CompanyID, s)
	})
}

// UpdateSettings: PUT /ledger-settings — บัญชีกำไรสะสม / สรุปผลต้องเป็นบัญชีย่อยหมวดส่วนของเจ้าของที่ active
func (uc *YearEndUseCases) UpdateSettings(ctx context.Context, a dto.Actor, s entities.Settings) (dto.LedgerSettings, error) {
	err := uc.write(ctx, a, vo.PermAccountManage, func(ctx context.Context) error {
		var ids []vo.AccountID
		for _, id := range []*vo.AccountID{s.RetainedEarningsAccountID, s.IncomeSummaryAccountID} {
			if id != nil {
				ids = append(ids, *id)
			}
		}
		accounts, err := uc.accounts.FindByIDs(ctx, a.CompanyID, ids)
		if err != nil {
			return err
		}
		ok := map[vo.AccountID]bool{}
		for _, acc := range accounts {
			ok[acc.ID()] = acc.CanPost() && acc.Data().Type == vo.AccountEquity
		}
		for _, id := range ids {
			if !ok[id] {
				return fmt.Errorf("%w: retained earnings / income summary must be an active equity sub-account", domain.ErrAccountNotPostable)
			}
		}
		if s.ClosingJournalID != nil {
			j, err := uc.journals.FindByID(ctx, a.CompanyID, *s.ClosingJournalID)
			if err != nil {
				return err
			}
			if !j.IsActive() {
				return domain.ErrJournalInactive
			}
		}
		return uc.settings.Save(ctx, a.CompanyID, s)
	})
	if err != nil {
		return dto.LedgerSettings{}, err
	}
	return uc.Settings(ctx, a)
}

func (uc *YearEndUseCases) settingsDTO(ctx context.Context, companyID vo.CompanyID, s entities.Settings) (dto.LedgerSettings, error) {
	var ids []vo.AccountID
	for _, id := range []*vo.AccountID{s.RetainedEarningsAccountID, s.IncomeSummaryAccountID} {
		if id != nil {
			ids = append(ids, *id)
		}
	}
	accs, err := uc.accounts.RefsByIDs(ctx, companyID, ids)
	if err != nil {
		return dto.LedgerSettings{}, err
	}
	out := dto.LedgerSettings{}
	ref := func(id *vo.AccountID) *dto.Ref {
		if id == nil {
			return nil
		}
		r := accs[*id]
		return &dto.Ref{ID: id.String(), Code: r.Code, Name: r.Name}
	}
	out.RetainedEarningsAccount, out.IncomeSummaryAccount = ref(s.RetainedEarningsAccountID), ref(s.IncomeSummaryAccountID)
	if s.ClosingJournalID != nil {
		js, err := uc.journals.RefsByIDs(ctx, companyID, []vo.JournalID{*s.ClosingJournalID})
		if err != nil {
			return dto.LedgerSettings{}, err
		}
		j := js[*s.ClosingJournalID]
		out.ClosingJournal = &dto.Ref{ID: s.ClosingJournalID.String(), Code: j.Code, Name: j.Name}
	}
	return out, nil
}
