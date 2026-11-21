package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"apexis/backend/internal/modules/inventory/application/dto"
	"apexis/backend/internal/modules/inventory/application/ports"
	"apexis/backend/internal/modules/inventory/application/ports/repositories"
	"apexis/backend/internal/modules/inventory/domain/entities"
	domain "apexis/backend/internal/modules/inventory/domain/errors"
	"apexis/backend/internal/modules/inventory/domain/services"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// ClosingUseCases: ปรับปรุงสินค้าคงเหลือสิ้นงวด (ระบบ periodic)
//
// ปรับบัญชีสินค้าคงเหลือใน ledger ให้เท่ามูลค่า FIFO ณ วันสิ้นงวด ส่วนต่างลงบัญชีต้นทุนขาย
// งวดหนึ่งปิดได้ครั้งเดียว ต้องปิดเรียงงวด และยกเลิกย้อนจากงวดล่าสุด (ยอดของงวดถัดไปอิงงวดก่อน)
type ClosingUseCases struct {
	base
	settings repositories.SettingsRepository
	closings repositories.ClosingRepository
	items    repositories.ItemRepository
	stock    repositories.StockRepository
}

func (uc *ClosingUseCases) Get(ctx context.Context, in dto.ByID[vo.ClosingID]) (dto.Closing, error) {
	return read(ctx, uc.base, in.Actor, vo.PermClosingRead, func() (dto.Closing, error) {
		return uc.closings.Get(ctx, in.CompanyID, in.ID)
	})
}

func (uc *ClosingUseCases) List(ctx context.Context, in dto.List[dto.ClosingFilter]) (listing.Result[dto.Closing], error) {
	return read(ctx, uc.base, in.Actor, vo.PermClosingRead, func() (listing.Result[dto.Closing], error) {
		return uc.closings.List(ctx, in.CompanyID, in.Filter)
	})
}

// CreateClosingInput: Date คือวันใดก็ได้ในงวดที่จะปิด
type CreateClosingInput struct {
	dto.Actor
	Date time.Time
}

// Create: งวดต้องเปิด, ปีบัญชีต้องเป็น periodic, ตั้งบัญชีต้นทุนขาย + สมุดรายวันแล้ว,
// ยังไม่มีการปิดงวดนี้หรืองวดหลังจากนี้ -> คำนวณ -> ลงบัญชีผ่าน ledger ใน transaction เดียว
func (uc *ClosingUseCases) Create(ctx context.Context, in CreateClosingInput) (dto.Closing, error) {
	if !vo.IsDateOnly(in.Date) {
		return dto.Closing{}, domain.ErrInvalidDate
	}
	var id vo.ClosingID
	err := uc.write(ctx, in.Actor, vo.PermClosingPost, func(ctx context.Context) error {
		settings, systems, err := uc.settings.Get(ctx, in.CompanyID, repositories.ForShare)
		if err != nil {
			return err
		}
		period, err := uc.openPeriod(ctx, in.CompanyID, in.Date)
		if err != nil {
			return err
		}
		if entities.SystemFor(systems, period.FiscalYear) != vo.Periodic {
			return domain.ErrNotPeriodic
		}
		switch {
		case settings.CogsAccountID == nil:
			return fmt.Errorf("%w: cost of goods sold account is not set", domain.ErrSettingsMissing)
		case settings.JournalID == nil:
			return fmt.Errorf("%w: journal is not set", domain.ErrSettingsMissing)
		}
		if err := uc.ensureLatest(ctx, in.CompanyID, period.End); err != nil {
			return err
		}
		c := entities.NewPeriodClosing(in.CompanyID, period.ID, period.FiscalYear, period.End, in.UserID)
		if err := uc.closings.Create(ctx, c); err != nil { // จองงวดก่อนคำนวณ (unique ที่ DB กันปิดซ้อน)
			return err
		}
		id = c.ID()
		lines, err := uc.plan(ctx, in.CompanyID, settings, period.End)
		if err != nil || len(lines) == 0 {
			return err
		}
		entryID, entryNo, err := uc.ledger.Post(ctx, ports.LedgerEntry{
			CompanyID: in.CompanyID, UserID: in.UserID, JournalID: *settings.JournalID, Date: period.End,
			Description: fmt.Sprintf("Inventory period-end adjustment %d (%s)", period.FiscalYear, period.End.Format(time.DateOnly)),
			Reference:   c.ID().String(), Lines: lines,
		})
		if err != nil {
			return err
		}
		c.SetEntry(entryID, entryNo, inventoryChange(lines, *settings.CogsAccountID))
		return uc.closings.UpdateStatus(ctx, c)
	})
	if err != nil {
		return dto.Closing{}, err
	}
	return uc.closings.Get(ctx, in.CompanyID, id)
}

// Cancel: กลับรายการบัญชี ณ วันสิ้นงวดเดิม — งวดต้องยังเปิด และต้องไม่มีการปิดงวดหลังจากนี้
func (uc *ClosingUseCases) Cancel(ctx context.Context, in dto.ByID[vo.ClosingID]) (dto.Closing, error) {
	err := uc.write(ctx, in.Actor, vo.PermClosingPost, func(ctx context.Context) error {
		c, err := uc.closings.FindForUpdate(ctx, in.CompanyID, in.ID)
		if err != nil {
			return err
		}
		s := c.State()
		if err := c.Cancel(in.UserID, uc.now()); err != nil {
			return err
		}
		if _, err := uc.openPeriod(ctx, in.CompanyID, s.PeriodEnd); err != nil {
			return err
		}
		if err := uc.ensureLatest(ctx, in.CompanyID, s.PeriodEnd); err != nil {
			return err
		}
		if s.EntryID != nil {
			if err := uc.ledger.Reverse(ctx, in.CompanyID, in.UserID, *s.EntryID, s.PeriodEnd,
				"Cancel inventory period-end adjustment "+s.EntryNo); err != nil {
				return err
			}
		}
		return uc.closings.UpdateStatus(ctx, c)
	})
	if err != nil {
		return dto.Closing{}, err
	}
	return uc.closings.Get(ctx, in.CompanyID, in.ID)
}

func (uc *ClosingUseCases) openPeriod(ctx context.Context, companyID vo.CompanyID, date time.Time) (ports.Period, error) {
	p, err := uc.org.PeriodAt(ctx, companyID, date)
	if err != nil {
		return ports.Period{}, err
	}
	if !p.Open {
		return ports.Period{}, domain.ErrPeriodClosed
	}
	return p, nil
}

// ensureLatest: ไม่มีการปิดงวด (posted) ที่วันสิ้นงวดหลัง periodEnd
func (uc *ClosingUseCases) ensureLatest(ctx context.Context, companyID vo.CompanyID, periodEnd time.Time) error {
	later, err := uc.closings.PostedOnOrAfter(ctx, companyID, periodEnd.AddDate(0, 0, 1))
	if err != nil {
		return err
	}
	if later {
		return domain.ErrInventoryClosed
	}
	return nil
}

// plan: มูลค่า FIFO ต่อบัญชีสินค้าคงเหลือ (สินค้า -> หมวด -> ค่าเริ่มต้น) เทียบยอดใน ledger
// บัญชีที่เทียบรวมบัญชีสินค้าคงเหลือทุกบัญชีที่ตั้งไว้ (บัญชีที่ไม่มีของเหลือแล้วจะถูกปรับเป็นศูนย์)
func (uc *ClosingUseCases) plan(ctx context.Context, companyID vo.CompanyID, s entities.Settings, asOf time.Time) ([]services.AdjustmentLine, error) {
	values, err := uc.stock.ValueByItem(ctx, companyID, asOf)
	if err != nil {
		return nil, err
	}
	target := map[vo.AccountID]vo.Money{}
	for _, v := range values {
		acc := v.Item.Or(v.Category).Or(s.Defaults).Inventory
		if acc == nil {
			return nil, fmt.Errorf("%w: inventory account of item %s", domain.ErrSettingsMissing, v.Code)
		}
		target[*acc] = target[*acc].Add(v.Value)
	}
	configured, err := uc.items.InventoryAccountIDs(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if d := s.Defaults.Inventory; d != nil {
		configured = append(configured, *d)
	}
	accounts := make([]vo.AccountID, 0, len(target)+len(configured))
	seen := map[vo.AccountID]bool{}
	for id := range target {
		seen[id] = true
		accounts = append(accounts, id)
	}
	for _, id := range configured {
		if !seen[id] {
			seen[id] = true
			accounts = append(accounts, id)
		}
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	balances, err := uc.ledger.Balances(ctx, companyID, accounts, asOf)
	if err != nil {
		return nil, err
	}
	return services.PlanClosing(target, balances, *s.CogsAccountID), nil
}

// inventoryChange: ผลรวม (เดบิต - เครดิต) ของบรรทัดสินค้าคงเหลือ
func inventoryChange(lines []services.AdjustmentLine, cogs vo.AccountID) vo.Money {
	net := decimal.Zero
	for _, l := range lines {
		if l.AccountID != cogs {
			net = net.Add(l.Debit).Sub(l.Credit)
		}
	}
	return net
}
