package usecases

import (
	"context"

	"apexis/backend/internal/modules/sales/domain/entities"
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

// รหัสในผังบัญชีมาตรฐาน (ledger) ที่ใช้เป็นค่าตั้งขายเริ่มต้นของบริษัทใหม่
const (
	codeGeneralJournal = "GEN"
	codeTradeAR        = "1103010100" // ลูกหนี้การค้า
	codeOutputVat      = "2102021100" // ภาษีขาย
	codeUndueVat       = "2102020900" // ภาษีขายยังไม่ถึงกำหนดชำระ (ขายบริการ ม.78/1)
	codeDeposit        = "2102021000" // เงินมัดจำรับ
)

// salesDefaultCodes: บัญชีรายได้ / ต้นทุนขาย / รับคืน ตามประเภทธุรกิจ ("" = ไม่ตั้ง)
type salesDefaultCodes struct{ Revenue, Cogs, Returns string }

var salesDefaults = map[integration.BusinessType]salesDefaultCodes{
	// บริการ: รายได้ค่าบริการ, ไม่มีต้นทุนขายสินค้า, ใบลดหนี้ลงส่วนลดจ่าย
	integration.BusinessService: {Revenue: "4101000000", Returns: "4104000000"},
	integration.BusinessTrading: {Revenue: "4051000000", Cogs: "5051000000", Returns: "4052000000"},
	// ผลิต: ขายสินค้าสำเร็จรูปเหมือนซื้อขาย
	integration.BusinessManufacturing: {Revenue: "4051000000", Cogs: "5051000000", Returns: "4052000000"},
	// ก่อสร้าง: รายได้ค่าก่อสร้าง, วัสดุที่ตัดสต็อกลงค่าวัสดุก่อสร้าง; ใบลดหนี้ลดบัญชีรายได้ของบรรทัด (ไม่ตั้งบัญชีรับคืน)
	integration.BusinessConstruction: {Revenue: "4061000000", Cogs: "5071000000"},
}

// Provision ตั้งค่าขายเริ่มต้นของบริษัทใหม่จากผังบัญชีมาตรฐานตามประเภทธุรกิจ (VAT 7%, GEN, ลูกหนี้ / ภาษีขาย / รายได้ / ต้นทุนขาย / รับคืน)
// เรียกจาก hook ตอนสร้างบริษัท — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ของผู้เรียก; บัญชีที่ไม่มีในผังจะไม่ตั้ง
// query: บัญชีตามรหัส 1 + สมุดตามรหัส 1 + UPSERT ค่าตั้ง 1
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID, bt integration.BusinessType) error {
	codes := salesDefaults[bt]
	return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		accounts, err := uc.ledger.AccountsByCode(ctx, companyID, nonEmpty(codeTradeAR, codeOutputVat, codeUndueVat, codeDeposit, codes.Revenue, codes.Cogs, codes.Returns))
		if err != nil {
			return err
		}
		journals, err := uc.ledger.JournalsByCode(ctx, companyID, []string{codeGeneralJournal})
		if err != nil {
			return err
		}
		s := entities.DefaultSettings()
		s.JournalID = pick(journals, codeGeneralJournal)
		s.ARAccountID = pick(accounts, codeTradeAR)
		s.OutputVatAccountID = pick(accounts, codeOutputVat)
		s.RevenueAccountID = pick(accounts, codes.Revenue)
		s.CogsAccountID = pick(accounts, codes.Cogs)
		s.SalesReturnsAccountID = pick(accounts, codes.Returns)
		s.UndueOutputVatAccountID = pick(accounts, codeUndueVat)
		s.DepositAccountID = pick(accounts, codeDeposit)
		if s, err = entities.NewSettings(s); err != nil {
			return err
		}
		return uc.settings.Save(ctx, companyID, s)
	})
}

// pick คืน id ของรหัส (nil เมื่อไม่ระบุรหัสหรือไม่พบในผัง)
func pick[T any](found map[string]T, code string) *T {
	if id, ok := found[code]; ok && code != "" {
		return &id
	}
	return nil
}

func nonEmpty(codes ...string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}
