package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

// Settings คือค่าตั้งเงินสดย่อยของบริษัท
//   - InputVatAccountID: ภาษีซื้อของใบสำคัญจ่ายที่มีใบกำกับภาษี (ตั้งให้อัตโนมัติจากผังบัญชีมาตรฐาน 1107090000)
type Settings struct {
	InputVatAccountID *vo.AccountID
}

// RequireInputVat: ต้องตั้งบัญชีภาษีซื้อก่อนเบิกชดเชยใบที่มี VAT
func (s Settings) RequireInputVat() (vo.AccountID, error) {
	if s.InputVatAccountID == nil {
		return vo.AccountID{}, fmt.Errorf("%w: input_vat_account_id", domain.ErrSettingsMissing)
	}
	return *s.InputVatAccountID, nil
}
