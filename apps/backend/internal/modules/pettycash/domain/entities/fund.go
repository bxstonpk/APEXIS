// Package entities คือ entity และกฎธุรกิจของเงินสดย่อย (ระบบวงเงินคงที่ / imprest)
//
//   - กองทุน (Fund): วงเงิน + ผู้ถือเงิน + บัญชีเงินสดย่อยในผังบัญชี
//   - ใบสำคัญจ่าย (Voucher): จ่ายจากเงินสดย่อย — ไม่ลงบัญชีตอนจ่าย (ลงตอนการเงินจ่ายชดเชย)
//   - ใบขอเบิกชดเชย (Replenishment): ตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชยตามใบสำคัญจ่าย — การเงินยืนยันจ่ายเงิน
package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/pettycash/domain/errors"
	vo "apexis/backend/internal/modules/pettycash/domain/value_objects"
)

// FundData คือข้อมูลกองทุนเงินสดย่อย
//
// ImprestAmount = วงเงิน: กำหนดตอนสร้าง (แก้ได้จนกว่าการเงินจ่ายใบขอตั้งวงเงิน) — หลังตั้งแล้วเพิ่มผ่านใบขอเพิ่มวงเงินเท่านั้น
type FundData struct {
	Code          string
	Name          string
	CustodianID   vo.UserID
	GLAccountID   vo.AccountID
	ImprestAmount vo.Money
	Status        vo.Status
}

type Fund struct {
	id   vo.FundID
	data FundData
}

func NewFund(d FundData, amountDecimals int) (*Fund, error) {
	n, err := normalizeFund(d, amountDecimals)
	if err != nil {
		return nil, err
	}
	return &Fund{id: vo.NewID[vo.FundID](), data: n}, nil
}

func RehydrateFund(id vo.FundID, d FundData) *Fund { return &Fund{id: id, data: d} }

func (f *Fund) ID() vo.FundID     { return f.id }
func (f *Fund) Data() FundData    { return f.data }
func (f *Fund) IsActive() bool    { return f.data.Status == vo.StatusActive }
func (f *Fund) Imprest() vo.Money { return f.data.ImprestAmount }

// Update แก้ชื่อ / ผู้ถือเงิน / บัญชี / สถานะ — รหัสคงเดิม; วงเงินแก้ได้เฉพาะกองทุนที่ยังไม่ได้ตั้งวงเงิน (funded = false)
func (f *Fund) Update(d FundData, funded bool, amountDecimals int) error {
	d.Code = f.data.Code
	if funded {
		d.ImprestAmount = f.data.ImprestAmount
	}
	n, err := normalizeFund(d, amountDecimals)
	if err != nil {
		return err
	}
	f.data = n
	return nil
}

// AddImprest: การเงินจ่ายใบขอเพิ่มวงเงิน (ยกเลิกใบจ่าย = ยอดติดลบ)
func (f *Fund) AddImprest(amount vo.Money) { f.data.ImprestAmount = f.data.ImprestAmount.Add(amount) }

func normalizeFund(d FundData, amountDecimals int) (FundData, error) {
	var err error
	if d.Code, err = vo.NewCode(d.Code); err != nil {
		return d, err
	}
	if d.Name, err = vo.NewName(d.Name, 200); err != nil {
		return d, err
	}
	if _, err = vo.ParseStatus(string(d.Status)); err != nil {
		return d, err
	}
	if !d.ImprestAmount.IsPositive() || !vo.FitsCompany(d.ImprestAmount, amountDecimals) {
		return d, fmt.Errorf("%w: imprest_amount must be positive with at most %d decimal places", domain.ErrInvalidAmount, amountDecimals)
	}
	return d, nil
}
