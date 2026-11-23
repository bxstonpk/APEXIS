// Package usecases คือ use case ของ taxregistry
package usecases

import (
	"context"
	"sort"

	"apexis/backend/internal/modules/taxregistry/application/dto"
	"apexis/backend/internal/modules/taxregistry/application/ports"
	"apexis/backend/internal/modules/taxregistry/domain/entities"
	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
	vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"
)

// Lookup ค้นหาผู้จด VAT ด้วยเลขผู้เสียภาษี (และเลขสาขา) — ใช้เติมข้อมูลลูกค้า / ผู้ขายอัตโนมัติ
// ไม่มี database / transaction: เป็นการอ่านจากบริการภายนอกอย่างเดียว ผลลัพธ์ไม่ถูกบันทึกจนกว่าผู้ใช้จะกดบันทึกฟอร์ม
type Lookup struct {
	auth     ports.Authorizer
	registry ports.VatRegistry
}

func NewLookup(auth ports.Authorizer, registry ports.VatRegistry) *Lookup {
	return &Lookup{auth: auth, registry: registry}
}

// Execute: สำนักงานใหญ่ขึ้นก่อน แล้วเรียงตามเลขสาขา; ไม่พบ = ErrNotFound
func (uc *Lookup) Execute(ctx context.Context, in dto.LookupInput) ([]entities.VatRegistrant, error) {
	taxID, err := vo.TaxID(in.TaxID)
	if err != nil {
		return nil, err
	}
	branch, err := vo.BranchNo(in.BranchNo)
	if err != nil {
		return nil, err
	}
	if err := uc.auth.AuthorizeAny(ctx, in.UserID, in.CompanyID); err != nil {
		return nil, err
	}
	rows, err := uc.registry.Lookup(ctx, taxID, branch)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domain.ErrNotFound
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].BranchNo < rows[j].BranchNo })
	return rows, nil
}
