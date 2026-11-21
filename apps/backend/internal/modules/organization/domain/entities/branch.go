package entities

import (
	"slices"

	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

type BranchData struct {
	Code        string
	Name        string
	TaxBranchNo string // 5 หลัก, "00000" = สำนักงานใหญ่
	Address     vo.Address
	Phone       string
	Status      vo.Status
}

// Branch คือสาขา (ตามทะเบียนภาษี) ของบริษัท
//
// กฎ:
//   - เลขที่สาขาไม่ซ้ำในบริษัท จึงมีสำนักงานใหญ่ได้สาขาเดียว (บังคับด้วย unique constraint)
//   - สำนักงานใหญ่ปิดใช้งาน / ลบ / เปลี่ยนเลขที่สาขาไม่ได้
type Branch struct {
	id        vo.BranchID
	companyID vo.CompanyID
	data      BranchData
}

func NewBranch(companyID vo.CompanyID, d BranchData) (*Branch, error) {
	n, err := normalizeBranch(d)
	if err != nil {
		return nil, err
	}
	return &Branch{id: vo.NewID[vo.BranchID](), companyID: companyID, data: n}, nil
}

func RehydrateBranch(id vo.BranchID, companyID vo.CompanyID, d BranchData) *Branch {
	return &Branch{id: id, companyID: companyID, data: d}
}

func (b *Branch) ID() vo.BranchID         { return b.id }
func (b *Branch) CompanyID() vo.CompanyID { return b.companyID }
func (b *Branch) Data() BranchData        { return b.data }
func (b *Branch) IsHeadOffice() bool      { return vo.TaxBranchNo(b.data.TaxBranchNo).IsHeadOffice() }

func (b *Branch) Update(d BranchData) error {
	n, err := normalizeBranch(d)
	if err != nil {
		return err
	}
	if b.IsHeadOffice() && (n.Status != vo.StatusActive || !vo.TaxBranchNo(n.TaxBranchNo).IsHeadOffice()) {
		return domain.ErrHeadOfficeImmutable
	}
	b.data = n
	return nil
}

func (b *Branch) EnsureDeletable() error {
	if b.IsHeadOffice() {
		return domain.ErrHeadOfficeImmutable
	}
	return nil
}

func normalizeBranch(d BranchData) (BranchData, error) {
	code, err := vo.NewCode(d.Code)
	if err != nil {
		return BranchData{}, err
	}
	name, err := vo.NewName(d.Name, 200)
	if err != nil {
		return BranchData{}, err
	}
	taxNo, err := vo.NewTaxBranchNo(d.TaxBranchNo)
	if err != nil {
		return BranchData{}, err
	}
	addr, err := vo.NewAddress(d.Address)
	if err != nil {
		return BranchData{}, err
	}
	phone, err := vo.NewPhone(d.Phone)
	if err != nil {
		return BranchData{}, err
	}
	if err := ensureStatus(d.Status); err != nil {
		return BranchData{}, err
	}
	return BranchData{Code: code.String(), Name: name, TaxBranchNo: taxNo.String(), Address: addr, Phone: phone, Status: d.Status}, nil
}

func ensureStatus(s vo.Status) error {
	if !slices.Contains(vo.Statuses, s) {
		return domain.ErrInvalidStatus
	}
	return nil
}
