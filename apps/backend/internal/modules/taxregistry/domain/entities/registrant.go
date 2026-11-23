// Package entities คือข้อมูลผู้ประกอบการจดทะเบียนภาษีมูลค่าเพิ่ม (1 รายการ = 1 สาขา)
package entities

import (
	"strings"

	"apexis/backend/internal/platform/thaiaddress"
)

const HeadOffice = "00000"

// VatRegistrant คือผู้จด VAT หนึ่งสาขาตามฐานข้อมูลกรมสรรพากร
type VatRegistrant struct {
	TaxID        string
	BranchNo     string
	Title        string // คำนำหน้า เช่น บริษัท, ห้างหุ้นส่วนจำกัด, นาย
	FirstName    string // ชื่อ (นิติบุคคล = ชื่อทั้งหมดหลังคำนำหน้า)
	MiddleName   string
	LastName     string
	BranchTitle  string
	BranchName   string
	Phone        string
	RegisteredOn string // วันที่จดทะเบียน VAT ตามที่สรรพากรส่ง (dd/mm/yyyy พ.ศ.)
	Address      thaiaddress.Address
}

func join(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = thaiaddress.Clean(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// Name: ชื่อตามกฎหมาย เช่น "บริษัท เชิดชัย ไพรม์ เอ็นจิเนียริ่ง จำกัด", "นาย สมชาย ใจดี"
func (r VatRegistrant) Name() string {
	return join(r.Title, r.FirstName, r.MiddleName, r.LastName)
}

// DisplayBranch: ชื่อสาขาสำหรับเลือก (สำนักงานใหญ่ = ชื่อหลัก)
func (r VatRegistrant) DisplayBranch() string {
	if n := join(r.BranchTitle, r.BranchName); n != "" {
		return n
	}
	return r.Name()
}

// TaxType: เลขผู้เสียภาษีขึ้นต้นด้วย 0 = นิติบุคคล (ภ.ง.ด.53), อื่นๆ = บุคคลธรรมดา (ภ.ง.ด.3)
func (r VatRegistrant) TaxType() string {
	if strings.HasPrefix(r.TaxID, "0") {
		return "juristic"
	}
	return "individual"
}

func (r VatRegistrant) IsHeadOffice() bool { return r.BranchNo == HeadOffice }
