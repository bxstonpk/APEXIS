package usecases

import (
	"context"
	"errors"

	"apexis/backend/internal/platform/numbering"
	"fmt"
	"strings"
	"time"

	"apexis/backend/internal/modules/ledger/application/dto"
	"apexis/backend/internal/modules/ledger/application/ports"
	"apexis/backend/internal/modules/ledger/application/ports/repositories"
	"apexis/backend/internal/modules/ledger/domain/entities"
	domain "apexis/backend/internal/modules/ledger/domain/errors"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// EntryUseCases จัดการรายการบัญชี: draft -> posted -> reversed
type EntryUseCases struct {
	base
	entries  repositories.EntryRepository
	accounts repositories.AccountRepository
	journals repositories.JournalRepository
	yearEnds repositories.YearEndRepository
}

func (uc *EntryUseCases) Get(ctx context.Context, in dto.ByID[vo.EntryID]) (dto.Entry, error) {
	return read(ctx, uc.base, in.Actor, vo.PermEntryRead, func() (dto.Entry, error) {
		return uc.entries.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *EntryUseCases) List(ctx context.Context, in dto.List[dto.EntryFilter]) (listing.Result[dto.EntrySummary], error) {
	return read(ctx, uc.base, in.Actor, vo.PermEntryRead, func() (listing.Result[dto.EntrySummary], error) {
		return uc.entries.List(ctx, in.CompanyID, in.Filter)
	})
}

// Create สร้าง draft — ถ้า in.Post จะผ่านรายการต่อทันทีใน transaction เดียว (ต้องมีสิทธิ์ post ด้วย)
func (uc *EntryUseCases) Create(ctx context.Context, in dto.CreateEntry[entities.EntryData]) (dto.Entry, error) {
	var e *entities.Entry
	err := uc.writeWith(ctx, in.Actor, vo.PermEntryManage, func(ctx context.Context, s ports.CompanySettings) error {
		var err error
		if e, err = entities.NewEntry(in.CompanyID, in.UserID, in.Data, s.AmountDecimals); err != nil {
			return err
		}
		journal, err := uc.checkRefs(ctx, in.CompanyID, e, in.Post)
		if err != nil {
			return err
		}
		if in.Post {
			if err := uc.authorize(ctx, in.Actor, vo.PermEntryPost); err != nil {
				return err
			}
			if err := uc.post(ctx, in.Actor, e, journal); err != nil {
				return err
			}
		}
		if err := uc.entries.Create(ctx, e); err != nil {
			return err
		}
		return uc.afterPost(ctx, in.Actor, e)
	})
	if err != nil {
		return dto.Entry{}, err
	}
	return uc.entries.Get(ctx, in.CompanyID, e.ID())
}

// Update แทนที่ draft ทั้งก้อน (หัวรายการ + บรรทัด)
func (uc *EntryUseCases) Update(ctx context.Context, in dto.Update[vo.EntryID, entities.EntryData]) (dto.Entry, error) {
	err := uc.writeWith(ctx, in.Actor, vo.PermEntryManage, func(ctx context.Context, s ports.CompanySettings) error {
		e, err := uc.entries.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := e.Update(in.Data, s.AmountDecimals); err != nil {
			return err
		}
		if _, err := uc.checkRefs(ctx, in.CompanyID, e, false); err != nil {
			return err
		}
		return uc.entries.Update(ctx, e)
	})
	if err != nil {
		return dto.Entry{}, err
	}
	return uc.entries.Get(ctx, in.CompanyID, in.ID)
}

// Delete ลบ draft (ลบจริง — draft ยังไม่ใช่หลักฐานทางบัญชี; รายการที่ผ่านแล้วต้องกลับรายการแทน)
func (uc *EntryUseCases) Delete(ctx context.Context, in dto.ByID[vo.EntryID]) error {
	return uc.write(ctx, in.Actor, vo.PermEntryManage, func(ctx context.Context) error {
		e, err := uc.entries.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := e.EnsureDeletable(); err != nil {
			return err
		}
		return uc.entries.Delete(ctx, in.CompanyID, in.ID)
	})
}

// Post ผ่านรายการ draft: lock รายการ -> ตรวจบัญชี / มิติ / สมุด -> งวดต้องเปิด -> ออกเลขที่ -> บันทึก
func (uc *EntryUseCases) Post(ctx context.Context, in dto.ByID[vo.EntryID]) (dto.Entry, error) {
	err := uc.write(ctx, in.Actor, vo.PermEntryPost, func(ctx context.Context) error {
		e, err := uc.entries.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if err := e.EnsurePostable(); err != nil {
			return err
		}
		journal, err := uc.checkRefs(ctx, in.CompanyID, e, true)
		if err != nil {
			return err
		}
		if err := uc.post(ctx, in.Actor, e, journal); err != nil {
			return err
		}
		if err := uc.entries.UpdateStatus(ctx, e); err != nil {
			return err
		}
		return uc.afterPost(ctx, in.Actor, e)
	})
	if err != nil {
		return dto.Entry{}, err
	}
	return uc.entries.Get(ctx, in.CompanyID, in.ID)
}

// Reverse กลับรายการที่ผ่านแล้ว: สร้างรายการใหม่ที่สลับเดบิต / เครดิต แล้วผ่านทันที
// และเปลี่ยนต้นฉบับเป็น reversed — ทั้งหมดใน transaction เดียว
//
// ไม่ตรวจว่าบัญชี / สมุด / มิติยัง active (ต้องกลับรายการได้เสมอ) แต่งวดของวันที่กลับรายการต้องเปิด
func (uc *EntryUseCases) Reverse(ctx context.Context, in dto.ReverseEntry) (dto.Entry, error) {
	var reversal *entities.Entry
	err := uc.write(ctx, in.Actor, vo.PermEntryPost, func(ctx context.Context) error {
		e, err := uc.entries.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		if e.IsClosing() {
			return domain.ErrYearClosed // รายการปิดบัญชีกลับได้ด้วยการเปิดปี (year-end reopen) เท่านั้น
		}
		reversal, err = uc.reverse(ctx, in.Actor, in.ID, in.Date, in.Description)
		return err
	})
	if err != nil {
		return dto.Entry{}, err
	}
	return uc.entries.Get(ctx, in.CompanyID, reversal.ID())
}

// reverse คือขั้นตอนกลับรายการ (ต้องอยู่ใน transaction ที่ตรวจสิทธิ์ / สถานะบริษัทแล้ว)
func (uc *EntryUseCases) reverse(ctx context.Context, a dto.Actor, id vo.EntryID, date time.Time, description string) (*entities.Entry, error) {
	original, err := uc.entries.FindForUpdate(ctx, a.CompanyID, id)
	if err != nil {
		return nil, err
	}
	reversal, err := original.Reversal(date, description, a.UserID)
	if err != nil {
		return nil, err
	}
	if err := uc.post(ctx, a, reversal, nil); err != nil {
		return nil, err
	}
	if err := uc.entries.Create(ctx, reversal); err != nil {
		return nil, err
	}
	if err := original.MarkReversed(reversal.ID()); err != nil {
		return nil, err
	}
	if err := uc.entries.UpdateStatus(ctx, original); err != nil {
		return nil, err
	}
	// รายการปรับปรุงที่มีรายการกลับอัตโนมัติ: ยกเลิกไปด้วยใน transaction เดียว
	// รอผ่าน (draft) = ลบ; ผ่านแล้ว = กลับรายการ ณ วันที่ของมัน (งวดต้องเปิด มิฉะนั้น 422 ทั้งหมด)
	if revID := original.AutoReversalOf(); revID != nil {
		rev, err := uc.entries.FindForUpdate(ctx, a.CompanyID, *revID)
		if err != nil {
			return nil, err
		}
		switch rev.Status() {
		case vo.EntryDraft:
			if err := uc.entries.Delete(ctx, a.CompanyID, *revID); err != nil {
				return nil, err
			}
		case vo.EntryPosted:
			if _, err := uc.reverse(ctx, a, *revID, rev.Data().Date, ""); err != nil {
				return nil, err
			}
		}
	}
	return reversal, nil
}

// afterPost: รายการปรับปรุงที่กลับรายการอัตโนมัติ -> สร้างรายการกลับ ณ วันแรกของงวดถัดไป
// ผ่านทันทีถ้างวดนั้นมีและเปิดอยู่ มิฉะนั้นเก็บเป็น draft (รอผ่าน) — ผ่านภายหลังด้วย PostPendingReversals
// (เรียกอัตโนมัติตอนสร้างปีบัญชีใหม่ผ่าน hook ของ organization)
func (uc *EntryUseCases) afterPost(ctx context.Context, a dto.Actor, e *entities.Entry) error {
	d := e.Data()
	if e.Status() != vo.EntryPosted || !d.AutoReverse || e.AutoReversalOf() != nil {
		return nil
	}
	r, err := e.NewAutoReversal(d.Date.AddDate(0, 0, 1), a.UserID)
	if err != nil {
		return err
	}
	if _, err := uc.tryPost(ctx, a, r); err != nil {
		return err
	}
	if err := uc.entries.Create(ctx, r); err != nil {
		return err
	}
	return uc.entries.UpdateStatus(ctx, e)
}

// tryPost ผ่านรายการถ้างวดของวันที่มีอยู่และเปิด และปีบัญชียังไม่ปิด — ไม่ได้ = คงเป็น draft (ไม่ใช่ error)
func (uc *EntryUseCases) tryPost(ctx context.Context, a dto.Actor, e *entities.Entry) (bool, error) {
	period, err := uc.org.PeriodAt(ctx, a.CompanyID, e.Data().Date)
	if errors.Is(err, domain.ErrNoFiscalPeriod) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !period.Open {
		return false, nil
	}
	closed, err := uc.yearEnds.IsClosed(ctx, a.CompanyID, period.FiscalYear)
	if err != nil || closed {
		return false, err
	}
	return true, uc.post(ctx, a, e, nil)
}

// PostPendingReversals ผ่านรายการกลับอัตโนมัติที่รออยู่ของบริษัท (งวดของวันที่ต้องมีและเปิดแล้ว)
// ไม่ตรวจสิทธิ์ (ระบบเรียกจาก hook สร้างปีบัญชี ภายใน transaction ของผู้เรียก) — คืนจำนวนที่ผ่าน
func (uc *EntryUseCases) PostPendingReversals(ctx context.Context, companyID vo.CompanyID) (int, error) {
	pending, err := uc.entries.PendingAutoReversals(ctx, companyID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range pending {
		ok, err := uc.tryPost(ctx, dto.Actor{UserID: r.State().CreatedBy, CompanyID: companyID}, r)
		if err != nil {
			return n, err
		}
		if ok {
			if err := uc.entries.UpdateStatus(ctx, r); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// PostPending: POST /adjusting-entries/pending-reversals — สั่งผ่านรายการกลับที่รออยู่เอง (ต้องมีสิทธิ์ผ่านรายการ)
func (uc *EntryUseCases) PostPending(ctx context.Context, a dto.Actor) (int, error) {
	var n int
	err := uc.write(ctx, a, vo.PermEntryPost, func(ctx context.Context) error {
		var err error
		n, err = uc.PostPendingReversals(ctx, a.CompanyID)
		return err
	})
	return n, err
}

// post: กฎที่ต้องใช้ข้อมูลภายนอก entity — งวดบัญชีต้องมีและเปิด, ออกเลขที่เรียงไม่ขาดตอน
// journal = สมุดที่โหลดไว้แล้ว (nil = โหลดใหม่)
func (uc *EntryUseCases) post(ctx context.Context, a dto.Actor, e *entities.Entry, journal *entities.Journal) error {
	if err := e.EnsurePostable(); err != nil {
		return err
	}
	d := e.Data()
	if journal == nil {
		var err error
		if journal, err = uc.journals.FindByID(ctx, a.CompanyID, d.JournalID); err != nil {
			return err
		}
	}
	period, err := uc.org.PeriodAt(ctx, a.CompanyID, d.Date)
	if err != nil {
		return err
	}
	if err := uc.checkPeriod(ctx, a.CompanyID, d, period); err != nil {
		return err
	}
	format := journal.NumberFormat()
	nc := numbering.Context{Date: d.Date, FiscalYear: period.FiscalYear}
	n, err := uc.journals.NextNumber(ctx, a.CompanyID, d.JournalID, format.PeriodKey(nc))
	if err != nil {
		return err
	}
	return e.Post(entities.Posting{
		EntryNo:  format.Render(nc, n),
		PeriodID: period.ID, By: a.UserID, At: uc.now(),
	})
}

// checkPeriod: ลงรายการได้เมื่อ
//
//	ปีบัญชีของวันที่ยังไม่ปิดบัญชีสิ้นปี (ปิดแล้ว = ล็อกทั้งปีรวมงวดปรับปรุง)
//	รายการงวดปรับปรุง (งวดที่ 13): วันที่ = วันสุดท้ายของปีบัญชี — ลงได้แม้งวด 12 ปิดแล้ว
//	รายการปกติ: งวด (1–12) ต้องเปิด; รายการปรับปรุงสิ้นงวด (template) ต้องลงวันสุดท้ายของงวด
//
// ไม่มีเงื่อนไขเรื่องปีก่อน: ปีใหม่ลงรายการได้ตามปกติแม้ปีก่อนยังไม่ปิดบัญชี
func (uc *EntryUseCases) checkPeriod(ctx context.Context, companyID vo.CompanyID, d entities.EntryData, p ports.Period) error {
	closed, err := uc.yearEnds.IsClosed(ctx, companyID, p.FiscalYear)
	if err != nil {
		return err
	}
	if closed {
		return domain.ErrYearClosed
	}
	if d.AdjustingPeriod {
		years, err := uc.org.FiscalYears(ctx, companyID)
		if err != nil {
			return err
		}
		if y := ports.FindYear(years, p.FiscalYear); y == nil || !d.Date.Equal(y.End) {
			return domain.ErrAdjustingDate
		}
		return nil
	}
	if !p.Open {
		return domain.ErrPeriodClosed
	}
	if d.IsAdjustment() {
		r, err := uc.org.PeriodRange(ctx, companyID, d.Date)
		if err != nil {
			return err
		}
		if r == nil || !d.Date.Equal(r.End) {
			return domain.ErrAdjustmentDate
		}
	}
	return nil
}

// checkRefs ตรวจ record ที่รายการอ้างถึง (query ละ 1 ครั้ง ไม่ว่าจะมีกี่บรรทัด):
//   - สมุดต้องมีอยู่ (forPosting: ต้อง active)
//   - บัญชีต้องมีอยู่และเป็นบัญชีย่อย (forPosting: ต้อง active ด้วย)
//   - มิติวิเคราะห์ต้องมีอยู่ในบริษัท (organization)
func (uc *EntryUseCases) checkRefs(ctx context.Context, companyID vo.CompanyID, e *entities.Entry, forPosting bool) (*entities.Journal, error) {
	d := e.Data()
	journal, err := uc.journals.FindByID(ctx, companyID, d.JournalID)
	if err != nil {
		return nil, err
	}
	if forPosting && !journal.IsActive() {
		return nil, domain.ErrJournalInactive
	}

	ids := e.AccountIDs()
	accounts, err := uc.accounts.FindByIDs(ctx, companyID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[vo.AccountID]*entities.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID()] = a
	}
	for i, l := range d.Lines {
		a, ok := byID[l.AccountID]
		switch {
		case !ok:
			return nil, fmt.Errorf("%w: line %d", domain.ErrAccountNotFound, i+1)
		case !a.Data().Postable || (forPosting && !a.CanPost()):
			return nil, fmt.Errorf("%w: line %d account %s", domain.ErrAccountNotPostable, i+1, a.Data().Code)
		}
	}

	dims := dimensionsOf(d.Lines)
	missing, err := uc.org.MissingDimensions(ctx, companyID, dims)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrDimensionNotFound, strings.Join(missing, ", "))
	}
	return journal, nil
}

func dimensionsOf(lines []entities.LineData) ports.Dimensions {
	var d ports.Dimensions
	for _, l := range lines {
		if l.BranchID != nil {
			d.Branches = append(d.Branches, *l.BranchID)
		}
		if l.DepartmentID != nil {
			d.Departments = append(d.Departments, *l.DepartmentID)
		}
		if l.CostCenterID != nil {
			d.CostCenters = append(d.CostCenters, *l.CostCenterID)
		}
		if l.ProjectSiteID != nil {
			d.ProjectSites = append(d.ProjectSites, *l.ProjectSiteID)
		}
	}
	return d
}
