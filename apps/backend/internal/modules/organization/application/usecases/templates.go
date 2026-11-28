package usecases

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	"apexis/backend/internal/modules/organization/domain/entities"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/listing"
)

// TemplateUseCases: แม่แบบเอกสารพิมพ์ + รูปของแม่แบบ
//
// สิทธิ์:
//   - อ่าน (ใช้พิมพ์ / แสดงรูป) = สมาชิกของบริษัททุกคน — ใครเปิดเอกสารได้ก็ต้องพิมพ์ได้ด้วยแม่แบบของบริษัท
//     (ตรวจสิทธิ์อ่านเอกสารที่ module เจ้าของเอกสารแล้ว; แม่แบบไม่มีข้อมูลธุรกิจ) — แบบเดียวกับหัวกระดาษ (Letterhead)
//   - เขียน = organization.document_template:manage (ใน transaction เดียวกับการแก้ข้อมูล)
//   - organization.document_template:read ใช้เปิดหน้าจัดการแม่แบบในหน้าเว็บ
type TemplateUseCases struct {
	base
	members   ports.Membership
	templates repositories.DocumentTemplateRepository
	assets    repositories.DocumentAssetRepository
}

// member: ไม่ใช่สมาชิก = ErrCompanyNotFound (ไม่บอกว่ามีบริษัทนี้)
func (uc *TemplateUseCases) member(ctx context.Context, a dto.Actor) error {
	ids, err := uc.members.CompanyIDsOf(ctx, a.UserID)
	if err != nil {
		return err
	}
	if !slices.Contains(ids, a.CompanyID) {
		return domain.ErrCompanyNotFound
	}
	return nil
}

func (uc *TemplateUseCases) List(ctx context.Context, a dto.Actor, f dto.DocumentTemplateFilter) (listing.Result[dto.DocumentTemplate], error) {
	if err := uc.member(ctx, a); err != nil {
		return listing.Result[dto.DocumentTemplate]{}, err
	}
	return uc.templates.List(ctx, a.CompanyID, f)
}

func (uc *TemplateUseCases) Get(ctx context.Context, a dto.Actor, id vo.DocumentTemplateID) (dto.DocumentTemplate, error) {
	if err := uc.member(ctx, a); err != nil {
		return dto.DocumentTemplate{}, err
	}
	return uc.templates.Get(ctx, a.CompanyID, id)
}

// Default: แม่แบบค่าเริ่มต้นของชนิดเอกสาร — ไม่มี = ErrTemplateNotFound (หน้าเว็บใช้แม่แบบมาตรฐานของระบบ)
func (uc *TemplateUseCases) Default(ctx context.Context, a dto.Actor, docType string) (dto.DocumentTemplate, error) {
	dt, err := vo.ParseTemplateDocType(docType)
	if err != nil {
		return dto.DocumentTemplate{}, err
	}
	if err := uc.member(ctx, a); err != nil {
		return dto.DocumentTemplate{}, err
	}
	return uc.templates.GetDefault(ctx, a.CompanyID, dt)
}

func (uc *TemplateUseCases) Create(ctx context.Context, in dto.DocumentTemplateInput) (dto.DocumentTemplate, error) {
	data, err := templateData(in)
	if err != nil {
		return dto.DocumentTemplate{}, err
	}
	t, err := entities.NewDocumentTemplate(in.CompanyID, data)
	if err != nil {
		return dto.DocumentTemplate{}, err
	}
	if err := uc.write(ctx, in.Actor, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		if err := uc.templates.Create(ctx, t, in.UserID); err != nil {
			return err
		}
		if in.Default && t.Data().Status == vo.StatusActive {
			return uc.templates.SetDefault(ctx, in.CompanyID, t.Data().DocType, t.ID())
		}
		return nil
	}); err != nil {
		return dto.DocumentTemplate{}, err
	}
	return uc.templates.Get(ctx, in.CompanyID, t.ID())
}

func (uc *TemplateUseCases) Update(ctx context.Context, id vo.DocumentTemplateID, in dto.DocumentTemplateInput) (dto.DocumentTemplate, error) {
	data, err := templateData(in)
	if err != nil {
		return dto.DocumentTemplate{}, err
	}
	if err := uc.write(ctx, in.Actor, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		t, err := uc.templates.FindByID(ctx, in.CompanyID, id)
		if err != nil {
			return err
		}
		if err := t.Update(data); err != nil {
			return err
		}
		return uc.templates.Update(ctx, t)
	}); err != nil {
		return dto.DocumentTemplate{}, err
	}
	return uc.templates.Get(ctx, in.CompanyID, id)
}

// SetDefault: ตั้งเป็นค่าเริ่มต้นของชนิดเอกสาร (ค่าเริ่มต้นเดิมถูกล้างใน transaction เดียวกัน); แม่แบบปิดใช้งาน = 422
func (uc *TemplateUseCases) SetDefault(ctx context.Context, a dto.Actor, id vo.DocumentTemplateID) (dto.DocumentTemplate, error) {
	if err := uc.write(ctx, a, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		t, err := uc.templates.FindByID(ctx, a.CompanyID, id)
		if err != nil {
			return err
		}
		if t.Data().Status != vo.StatusActive {
			return domain.ErrTemplateInactive
		}
		return uc.templates.SetDefault(ctx, a.CompanyID, t.Data().DocType, id)
	}); err != nil {
		return dto.DocumentTemplate{}, err
	}
	return uc.templates.Get(ctx, a.CompanyID, id)
}

// Delete: ลบแม่แบบค่าเริ่มต้นได้ — ชนิดเอกสารนั้นกลับไปใช้แม่แบบมาตรฐานของระบบ
func (uc *TemplateUseCases) Delete(ctx context.Context, a dto.Actor, id vo.DocumentTemplateID) error {
	return uc.write(ctx, a, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		return uc.templates.Delete(ctx, a.CompanyID, id)
	})
}

func templateData(in dto.DocumentTemplateInput) (entities.DocumentTemplateData, error) {
	paper, err := vo.ParsePaper(in.Paper)
	if err != nil {
		return entities.DocumentTemplateData{}, err
	}
	status := vo.StatusActive
	if in.Status != "" {
		if status, err = vo.ParseStatus(in.Status); err != nil {
			return entities.DocumentTemplateData{}, err
		}
	}
	return entities.DocumentTemplateData{DocType: in.DocType, Name: in.Name, HTML: in.HTML, Paper: paper, Status: status}, nil
}

// ---------- รูป ----------

func (uc *TemplateUseCases) Assets(ctx context.Context, a dto.Actor) ([]dto.DocumentAsset, error) {
	if err := uc.member(ctx, a); err != nil {
		return nil, err
	}
	return uc.assets.List(ctx, a.CompanyID)
}

func (uc *TemplateUseCases) AssetContent(ctx context.Context, a dto.Actor, id vo.DocumentAssetID) (dto.DocumentAssetContent, error) {
	if err := uc.member(ctx, a); err != nil {
		return dto.DocumentAssetContent{}, err
	}
	return uc.assets.Content(ctx, a.CompanyID, id)
}

// UploadAsset: ชนิดไฟล์ตรวจจากเนื้อไฟล์ (png / jpeg / svg ที่ไม่มีสคริปต์) ไม่เกิน 500 KB
func (uc *TemplateUseCases) UploadAsset(ctx context.Context, a dto.Actor, name string, data []byte) (dto.DocumentAsset, error) {
	asset, err := entities.NewDocumentAsset(a.CompanyID, name, data)
	if err != nil {
		return dto.DocumentAsset{}, err
	}
	if err := uc.write(ctx, a, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		return uc.assets.Create(ctx, asset)
	}); err != nil {
		return dto.DocumentAsset{}, err
	}
	return dto.DocumentAsset{ID: uuid.UUID(asset.ID).String(), Name: asset.Name, ContentType: asset.ContentType, Size: len(asset.Data), CreatedAt: time.Now().UTC()}, nil
}

func (uc *TemplateUseCases) DeleteAsset(ctx context.Context, a dto.Actor, id vo.DocumentAssetID) error {
	return uc.write(ctx, a, vo.PermDocumentTemplateManage, func(ctx context.Context) error {
		return uc.assets.Delete(ctx, a.CompanyID, id)
	})
}
