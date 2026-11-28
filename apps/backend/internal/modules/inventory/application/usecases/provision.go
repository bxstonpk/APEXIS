package usecases

import (
	"context"

	"apexis/backend/internal/modules/inventory/domain/entities"
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

const codeGeneralJournal = "GEN"

// inventoryDefaultCodes: บัญชีเริ่มต้นของสินค้า (เมื่อสินค้า / หมวดไม่ได้ตั้ง) และบัญชีปรับปรุงสินค้าสิ้นงวด ("" = ไม่ตั้ง)
type inventoryDefaultCodes struct{ Purchase, Inventory, Cogs string }

var inventoryDefaults = map[integration.BusinessType]inventoryDefaultCodes{
	// บริการ: วัสดุที่ใช้ในการให้บริการ
	integration.BusinessService: {Purchase: "5102000000", Inventory: "1105010000", Cogs: "5102000000"},
	integration.BusinessTrading: {Purchase: "5052000000", Inventory: "1105040000", Cogs: "5051000000"},
	// ผลิต: ของที่ซื้อเข้าส่วนใหญ่คือวัตถุดิบ — ปรับปรุงสิ้นงวดลงวัตถุดิบใช้ไป (หมวดสินค้าสำเร็จรูปตั้งบัญชี 1105040000 / 5051000000 เอง)
	integration.BusinessManufacturing: {Purchase: "5052000000", Inventory: "1105060000", Cogs: "5061000000"},
	// ก่อสร้าง: วัสดุก่อสร้าง — ซื้อ / ผลต่างคงเหลือสิ้นงวดลงค่าวัสดุก่อสร้าง
	integration.BusinessConstruction: {Purchase: "5071000000", Inventory: "1105100000", Cogs: "5071000000"},
}

// Provision ตั้งค่าคลังสินค้าเริ่มต้นของบริษัทใหม่จากผังบัญชีมาตรฐานตามประเภทธุรกิจ
// ระบบบัญชีสินค้าไม่ตั้ง = สิ้นงวด (periodic) ทุกปีบัญชีทุกประเภทธุรกิจ (รวมธุรกิจผลิต) — เปลี่ยนเป็นต่อเนื่องได้ที่หน้าค่าตั้ง
// เรียกจาก hook ตอนสร้างบริษัท — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ของผู้เรียก; บัญชีที่ไม่มีในผังจะไม่ตั้ง
// query: บัญชีตามรหัส 1 + สมุดตามรหัส 1 + UPSERT ค่าตั้ง 1
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID, bt integration.BusinessType) error {
	codes := inventoryDefaults[bt]
	return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		accounts, err := uc.ledger.AccountsByCode(ctx, companyID, nonEmpty(codes.Purchase, codes.Inventory, codes.Cogs))
		if err != nil {
			return err
		}
		journals, err := uc.ledger.JournalsByCode(ctx, companyID, []string{codeGeneralJournal})
		if err != nil {
			return err
		}
		s := entities.Settings{
			Defaults:      vo.GLAccounts{Purchase: pick(accounts, codes.Purchase), Inventory: pick(accounts, codes.Inventory)},
			CogsAccountID: pick(accounts, codes.Cogs),
			JournalID:     pick(journals, codeGeneralJournal),
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
