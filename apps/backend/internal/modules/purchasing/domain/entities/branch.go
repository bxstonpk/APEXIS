package entities

import (
	"fmt"

	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/thaiaddress"
)

// MaxBranches คือจำนวนสาขาเพิ่มเติมสูงสุดต่อผู้ขาย
const MaxBranches = 500

// HeadOffice คือเลขสาขาของสำนักงานใหญ่
const HeadOffice = "00000"

// SupplierBranch คือสาขาเพิ่มเติมของผู้ขาย — สาขาหลักคือ BranchNo / Address ของผู้ขายเอง
// กฎ: เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส; สาขาอื่นของนิติบุคคลเดียวกันเพิ่มที่นี่ (เลขสาขาไม่ซ้ำภายในผู้ขาย)
type SupplierBranch struct {
	BranchNo      string
	Name          string // ชื่อสาขา เช่น "สาขาบางนา" (ว่างได้)
	AddressDetail thaiaddress.Address
	Address       string // ข้อความที่ประกอบจาก AddressDetail
}

// BranchInfo คือสาขาที่เอกสารอ้างถึง (สาขาหลักหรือสาขาเพิ่มเติม)
type BranchInfo struct {
	BranchNo string
	Name     string
	Address  string
}

func normalizeBranches(mainNo string, in []SupplierBranch) ([]SupplierBranch, error) {
	if len(in) > MaxBranches {
		return nil, fmt.Errorf("%w: at most %d branches", domain.ErrInvalidBranchNo, MaxBranches)
	}
	if len(in) > 0 && mainNo == "" {
		return nil, fmt.Errorf("%w: branch_no of the customer is required when it has other branches", domain.ErrInvalidBranchNo)
	}
	seen := map[string]bool{mainNo: true}
	out := make([]SupplierBranch, 0, len(in))
	for i, b := range in {
		no, err := vo.BranchNo(b.BranchNo)
		if err != nil {
			return nil, err
		}
		if no == "" {
			return nil, fmt.Errorf("%w: branches[%d].branch_no is required", domain.ErrInvalidBranchNo, i)
		}
		if seen[no] {
			return nil, fmt.Errorf("%w: %s", domain.ErrDuplicateBranch, no)
		}
		seen[no] = true
		name, err := vo.OptionalText(b.Name, 200)
		if err != nil {
			return nil, err
		}
		addr, err := thaiaddress.New(b.AddressDetail)
		if err != nil {
			return nil, err
		}
		out = append(out, SupplierBranch{BranchNo: no, Name: name, AddressDetail: addr, Address: addr.Format()})
	}
	return out, nil
}

// Branch: สาขาตามเลข — ว่าง = สาขาหลัก; ไม่มีในผู้ขาย = ErrBranchNotFound
func (s *Supplier) Branch(no string) (BranchInfo, error) {
	d := s.data
	if no == "" || no == d.BranchNo {
		return BranchInfo{BranchNo: d.BranchNo, Address: d.Address}, nil
	}
	for _, b := range d.Branches {
		if b.BranchNo == no {
			return BranchInfo{BranchNo: b.BranchNo, Name: b.Name, Address: b.Address}, nil
		}
	}
	return BranchInfo{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, no)
}
