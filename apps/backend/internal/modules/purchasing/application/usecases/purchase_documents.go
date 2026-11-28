package usecases

import (
	"context"
	"errors"

	"apexis/backend/internal/modules/purchasing/application/dto"
	"apexis/backend/internal/modules/purchasing/application/ports/repositories"
	domain "apexis/backend/internal/modules/purchasing/domain/errors"
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// PurchaseDocumentUseCases: รายการรวม "เอกสารซื้อ" (ใบสั่งซื้อ / ใบรับสินค้า / ซื้อสินค้า / ใบสั่งจ้าง / ตรวจรับงาน / ซื้อบริการ)
type PurchaseDocumentUseCases struct {
	base
	docs repositories.PurchaseDocumentRepository
}

// List คืนเฉพาะชนิดที่ผู้ใช้มีสิทธิ์อ่าน (สิทธิ์เดียวกับ endpoint รายการของแต่ละเอกสาร)
//
//	ไม่ระบุ type = ทุกชนิดที่มีสิทธิ์; ไม่มีสิทธิ์อ่านชนิดใดเลยที่ขอ = 403
//	ตรวจสิทธิ์ครั้งละชนิด (ไม่เกิน 6 ครั้ง, ข้อมูลสิทธิ์อยู่ใน identity) แล้วดึงรายการด้วย query เดียว
func (uc *PurchaseDocumentUseCases) List(ctx context.Context, in dto.List[dto.PurchaseDocumentFilter]) (listing.Result[dto.PurchaseDocument], error) {
	requested := in.Filter.Types
	if len(requested) == 0 {
		requested = vo.PurchaseDocTypes
	}
	allowed := make([]vo.PurchaseDocType, 0, len(requested))
	for _, t := range requested {
		err := uc.authorize(ctx, in.Actor, t.ReadPermission())
		switch {
		case err == nil:
			allowed = append(allowed, t)
		case !errors.Is(err, domain.ErrForbidden):
			return listing.Result[dto.PurchaseDocument]{}, err
		}
	}
	if len(allowed) == 0 {
		return listing.Result[dto.PurchaseDocument]{}, domain.ErrForbidden
	}
	in.Filter.Types = allowed
	return uc.docs.List(ctx, in.CompanyID, in.Filter)
}
