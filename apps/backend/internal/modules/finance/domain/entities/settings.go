package entities

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	domain "apexis/backend/internal/modules/finance/domain/errors"
	vo "apexis/backend/internal/modules/finance/domain/value_objects"
	"apexis/backend/internal/platform/numeric"
)

// ---------- ค่าตั้งการเงิน ----------

// Settings: สมุดรายวันของเอกสารแต่ละชนิด + บัญชีภาษีหัก ณ ที่จ่าย (nil = ไม่ได้ตั้ง)
// บัญชีลูกหนี้ / เจ้าหนี้มาจากค่าตั้งขาย / จัดซื้อ (เจ้าของบัญชีคุมนั้น)
type Settings struct {
	ReceiptJournalID     *vo.JournalID   // สมุดรายวันรับ (เช่น CRJ)
	PaymentJournalID     *vo.JournalID   // สมุดรายวันจ่าย (เช่น CDJ)
	TransferJournalID    *vo.JournalID   // โอนเงินระหว่างบัญชี (เช่น GEN)
	WhtReceivedAccountID *vo.GLAccountID // ภาษีถูกหัก ณ ที่จ่าย (สินทรัพย์)
	WhtPnd3AccountID     *vo.GLAccountID // ภาษีหัก ณ ที่จ่ายค้างจ่าย ภ.ง.ด.3
	WhtPnd53AccountID    *vo.GLAccountID // ภาษีหัก ณ ที่จ่ายค้างจ่าย ภ.ง.ด.53
}

func (s Settings) AccountIDs() []vo.GLAccountID {
	var out []vo.GLAccountID
	for _, p := range []*vo.GLAccountID{s.WhtReceivedAccountID, s.WhtPnd3AccountID, s.WhtPnd53AccountID} {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

func (s Settings) JournalIDs() []vo.JournalID {
	var out []vo.JournalID
	for _, p := range []*vo.JournalID{s.ReceiptJournalID, s.PaymentJournalID, s.TransferJournalID} {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// Require: ค่าตั้งที่ต้องมีก่อนลงบัญชี — คืน ErrSettingsMissing พร้อมชื่อที่ขาด
func (s Settings) Require(fields ...string) error {
	set := map[string]bool{
		"receipt_journal_id": s.ReceiptJournalID != nil, "payment_journal_id": s.PaymentJournalID != nil,
		"transfer_journal_id": s.TransferJournalID != nil, "wht_received_account_id": s.WhtReceivedAccountID != nil,
		"wht_pnd3_account_id": s.WhtPnd3AccountID != nil, "wht_pnd53_account_id": s.WhtPnd53AccountID != nil,
	}
	var missing []string
	for _, f := range fields {
		if !set[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrSettingsMissing, strings.Join(missing, ", "))
	}
	return nil
}

// WhtPayable: บัญชีภาษีหัก ณ ที่จ่ายค้างจ่ายตามแบบยื่น + ชื่อ field ของค่าตั้ง
func (s Settings) WhtPayable(t vo.TaxType) (*vo.GLAccountID, string) {
	if t == vo.TaxIndividual {
		return s.WhtPnd3AccountID, "wht_pnd3_account_id"
	}
	return s.WhtPnd53AccountID, "wht_pnd53_account_id"
}

// ---------- ประเภทภาษีหัก ณ ที่จ่าย ----------

type WhtTypeData struct {
	Code   string
	Name   string // เช่น ค่าบริการ, ค่าเช่า, ค่าขนส่ง
	Rate   vo.Money
	Status vo.Status
}

// WhtType คือประเภทเงินได้ที่หักภาษี ณ ที่จ่าย พร้อมอัตรา — ใบจ่ายเก็บอัตรา ณ วันจ่ายไว้ในบรรทัด
type WhtType struct {
	id        vo.WhtTypeID
	companyID vo.CompanyID
	data      WhtTypeData
}

func NewWhtType(companyID vo.CompanyID, d WhtTypeData) (*WhtType, error) {
	n, err := normalizeWhtType(d)
	if err != nil {
		return nil, err
	}
	return &WhtType{id: vo.NewID[vo.WhtTypeID](), companyID: companyID, data: n}, nil
}

func RehydrateWhtType(id vo.WhtTypeID, companyID vo.CompanyID, d WhtTypeData) *WhtType {
	return &WhtType{id: id, companyID: companyID, data: d}
}

func (w *WhtType) ID() vo.WhtTypeID        { return w.id }
func (w *WhtType) CompanyID() vo.CompanyID { return w.companyID }
func (w *WhtType) Data() WhtTypeData       { return w.data }
func (w *WhtType) IsActive() bool          { return w.data.Status == vo.StatusActive }

func (w *WhtType) Update(d WhtTypeData) error {
	n, err := normalizeWhtType(d)
	if err != nil {
		return err
	}
	w.data = n
	return nil
}

func normalizeWhtType(d WhtTypeData) (WhtTypeData, error) {
	var err error
	out := WhtTypeData{}
	if out.Code, err = vo.NewCode(d.Code); err != nil {
		return WhtTypeData{}, err
	}
	if out.Name, err = vo.NewName(d.Name, 200); err != nil {
		return WhtTypeData{}, err
	}
	if !d.Rate.IsPositive() || d.Rate.GreaterThan(decimal.NewFromInt(100)) || !numeric.FitsScale(d.Rate, 2) {
		return WhtTypeData{}, domain.ErrInvalidWhtRate
	}
	out.Rate = d.Rate
	if out.Status, err = vo.ParseStatus(string(d.Status)); err != nil {
		return WhtTypeData{}, err
	}
	return out, nil
}
