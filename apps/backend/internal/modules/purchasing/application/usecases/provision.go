package usecases

import (
	"context"

	"apexis/backend/internal/modules/purchasing/domain/entities"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

// รหัสในผังบัญชีมาตรฐาน (ledger) ที่ใช้เป็นค่าตั้งจัดซื้อเริ่มต้นของบริษัทใหม่
const (
	codeGeneralJournal = "GEN"
	codeTradeAP        = "2102010000" // เจ้าหนี้การค้า
	codeInputVat       = "1107090000" // ภาษีซื้อ
	codeGRIR           = "2102030000" // สินค้ารับแล้วรอใบแจ้งหนี้ (ไม่มีในผังธุรกิจบริการ)
)

// purchasingDefaultCodes: ผลต่างราคา / ส่งคืน ตามประเภทธุรกิจ ("" = ไม่ตั้ง)
type purchasingDefaultCodes struct{ PriceVariance, Returns string }

var purchasingDefaults = map[integration.BusinessType]purchasingDefaultCodes{
	// บริการ: ไม่มีบัญชีสินค้า — ใบลดหนี้ผู้ขายลดบัญชีซื้อของบรรทัด
	integration.BusinessService:       {},
	integration.BusinessTrading:       {PriceVariance: "5051000000", Returns: "5054000000"},
	integration.BusinessManufacturing: {PriceVariance: "5051000000", Returns: "5054000000"},
	// ก่อสร้าง: ผลต่างราคาวัสดุลงค่าวัสดุก่อสร้าง
	integration.BusinessConstruction: {PriceVariance: "5071000000"},
}

// Provision ตั้งค่าจัดซื้อเริ่มต้นของบริษัทใหม่จากผังบัญชีมาตรฐานตามประเภทธุรกิจ (VAT 7%, GEN, เจ้าหนี้ / ภาษีซื้อ / รับสินค้ารอใบแจ้งหนี้ / ผลต่างราคา / ส่งคืน)
// เรียกจาก hook ตอนสร้างบริษัท — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ของผู้เรียก; บัญชีที่ไม่มีในผังจะไม่ตั้ง
// query: บัญชีตามรหัส 1 + สมุดตามรหัส 1 + UPSERT ค่าตั้ง 1
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID, bt integration.BusinessType) error {
	codes := purchasingDefaults[bt]
	return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		accounts, err := uc.ledger.AccountsByCode(ctx, companyID, nonEmpty(codeTradeAP, codeInputVat, codeGRIR, codes.PriceVariance, codes.Returns))
		if err != nil {
			return err
		}
		journals, err := uc.ledger.JournalsByCode(ctx, companyID, []string{codeGeneralJournal})
		if err != nil {
			return err
		}
		s := entities.DefaultSettings()
		s.JournalID = pick(journals, codeGeneralJournal)
		s.APAccountID = pick(accounts, codeTradeAP)
		s.InputVatAccountID = pick(accounts, codeInputVat)
		s.GRIRAccountID = pick(accounts, codeGRIR)
		s.PriceVarianceAccountID = pick(accounts, codes.PriceVariance)
		s.PurchaseReturnsAccountID = pick(accounts, codes.Returns)
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
