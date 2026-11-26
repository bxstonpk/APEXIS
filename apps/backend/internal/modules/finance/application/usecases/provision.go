package usecases

import (
	"context"

	"apexis/backend/internal/modules/finance/domain/entities"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/integration"
)

// รหัสในผังบัญชีมาตรฐาน (ledger) ที่ใช้เป็นค่าตั้งการเงินเริ่มต้นของบริษัทใหม่ — เหมือนกันทุกประเภทธุรกิจ
const (
	codeReceiptJournal  = "CRJ"
	codePaymentJournal  = "CDJ"
	codeTransferJournal = "GEN"
	codeWhtReceived     = "1106030000" // ภาษีถูกหัก ณ ที่จ่ายรอนำเครดิต
	codeWhtPnd3         = "2102020700" // ภาษีเงินได้หัก ณ ที่จ่าย ภงด.3
	codeWhtPnd53        = "2102020800" // ภาษีเงินได้หัก ณ ที่จ่าย ภงด.53
)

// Provision ตั้งค่าการเงินเริ่มต้นของบริษัทใหม่ (สมุดรับ / จ่าย / โอน, บัญชีภาษีหัก ณ ที่จ่าย)
// เรียกจาก hook ตอนสร้างบริษัท — ไม่ตรวจสิทธิ์, เข้าร่วม transaction ของผู้เรียก; รหัสที่ไม่มีในผังจะไม่ตั้ง
// query: บัญชีตามรหัส 1 + สมุดตามรหัส 1 + UPSERT ค่าตั้ง 1
func (uc *SettingsUseCases) Provision(ctx context.Context, companyID vo.CompanyID, _ integration.BusinessType) error {
	return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		accounts, err := uc.ledger.AccountsByCode(ctx, companyID, []string{codeWhtReceived, codeWhtPnd3, codeWhtPnd53})
		if err != nil {
			return err
		}
		journals, err := uc.ledger.JournalsByCode(ctx, companyID, []string{codeReceiptJournal, codePaymentJournal, codeTransferJournal})
		if err != nil {
			return err
		}
		return uc.settings.Save(ctx, companyID, entities.Settings{
			ReceiptJournalID:     pick(journals, codeReceiptJournal),
			PaymentJournalID:     pick(journals, codePaymentJournal),
			TransferJournalID:    pick(journals, codeTransferJournal),
			WhtReceivedAccountID: pick(accounts, codeWhtReceived),
			WhtPnd3AccountID:     pick(accounts, codeWhtPnd3),
			WhtPnd53AccountID:    pick(accounts, codeWhtPnd53),
		})
	})
}

// pick คืน id ของรหัส (nil เมื่อไม่พบในผัง)
func pick[T any](found map[string]T, code string) *T {
	if id, ok := found[code]; ok {
		return &id
	}
	return nil
}
