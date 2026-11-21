package usecases

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
	"apexis/backend/internal/modules/organization/application/ports/repositories"
	domain "apexis/backend/internal/modules/organization/domain/errors"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/integration"
	"apexis/backend/internal/platform/numbering"
)

// DocumentTypes คือทะเบียนชนิดเอกสารที่ module ต่างๆ ลงทะเบียนตอนเริ่มโปรแกรม
// (organization รู้แค่ key / ชื่อ / prefix เริ่มต้น — ไม่รู้จักตารางของ module นั้น)
type DocumentTypes struct {
	mu    sync.RWMutex
	byKey map[string]integration.DocumentType
}

func NewDocumentTypes() *DocumentTypes {
	return &DocumentTypes{byKey: map[string]integration.DocumentType{}}
}

// Register: key ซ้ำ = panic (ผิดพลาดตอนต่อสาย module)
func (r *DocumentTypes) Register(types ...integration.DocumentType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range types {
		if _, dup := r.byKey[t.Key]; dup {
			panic(fmt.Sprintf("organization: document type %q registered twice", t.Key))
		}
		if _, err := numbering.Normalize(numbering.Format{Prefix: t.DefaultPrefix, Template: numbering.DefaultTemplate}); err != nil {
			panic(fmt.Sprintf("organization: document type %q: %v", t.Key, err))
		}
		r.byKey[t.Key] = t
	}
}

func (r *DocumentTypes) get(key string) (integration.DocumentType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.byKey[key]
	return t, ok
}

func (r *DocumentTypes) all() []integration.DocumentType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]integration.DocumentType, 0, len(r.byKey))
	for _, t := range r.byKey {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// NumberingUseCases: รูปแบบเลขที่เอกสารต่อชนิดเอกสาร (ไม่ตั้ง = prefix เริ่มต้น + numbering.DefaultTemplate)
type NumberingUseCases struct {
	base
	repo  repositories.NumberingRepository
	types *DocumentTypes
	now   func() time.Time
}

// List: ทุกชนิดเอกสารที่ลงทะเบียน พร้อมรูปแบบที่ใช้อยู่และตัวอย่างเลขที่
func (uc *NumberingUseCases) List(ctx context.Context, a dto.Actor) ([]dto.DocumentNumbering, error) {
	if err := uc.authorize(ctx, a, vo.PermNumberingRead); err != nil {
		return nil, err
	}
	saved, err := uc.repo.List(ctx, a.CompanyID)
	if err != nil {
		return nil, err
	}
	types := uc.types.all()
	out := make([]dto.DocumentNumbering, len(types))
	for i, t := range types {
		f, custom := saved[t.Key]
		if !custom {
			f = defaultFormat(t)
		}
		out[i] = uc.dto(t, f, custom)
	}
	return out, nil
}

// Update: มีผลกับเอกสารที่ออกเลขหลังจากนี้ — เลขเดิมไม่เปลี่ยน
func (uc *NumberingUseCases) Update(ctx context.Context, a dto.Actor, docType string, f numbering.Format) (dto.DocumentNumbering, error) {
	t, ok := uc.types.get(docType)
	if !ok {
		return dto.DocumentNumbering{}, domain.ErrDocumentTypeNotFound
	}
	n, err := numbering.Normalize(f)
	if err != nil {
		return dto.DocumentNumbering{}, fmt.Errorf("%w: %v", domain.ErrInvalidNumbering, err)
	}
	if err := uc.write(ctx, a, vo.PermNumberingManage, func(ctx context.Context) error {
		return uc.repo.Save(ctx, a.CompanyID, docType, n)
	}); err != nil {
		return dto.DocumentNumbering{}, err
	}
	return uc.dto(t, n, true), nil
}

// Format: รูปแบบที่ใช้ออกเลข (เรียกผ่าน public API, ไม่ตรวจสิทธิ์)
func (uc *NumberingUseCases) Format(ctx context.Context, companyID vo.CompanyID, docType string) (numbering.Format, error) {
	t, ok := uc.types.get(docType)
	if !ok {
		return numbering.Format{}, fmt.Errorf("organization: document type %q is not registered", docType)
	}
	f, found, err := uc.repo.Get(ctx, companyID, docType)
	if err != nil || !found {
		return defaultFormat(t), err
	}
	return f, nil
}

func defaultFormat(t integration.DocumentType) numbering.Format {
	return numbering.Format{Prefix: t.DefaultPrefix, Template: numbering.DefaultTemplate}
}

func (uc *NumberingUseCases) dto(t integration.DocumentType, f numbering.Format, custom bool) dto.DocumentNumbering {
	today := uc.now()
	return dto.DocumentNumbering{
		DocType: t.Key, Name: t.Name, DefaultPrefix: t.DefaultPrefix, Prefix: f.Prefix, Template: f.Template, Custom: custom,
		Example: f.Render(numbering.Context{Date: today, FiscalYear: today.Year()}, 1),
	}
}
