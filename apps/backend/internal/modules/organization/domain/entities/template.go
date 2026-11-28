package entities

import (
	"strings"
	"unicode/utf8"

	domain "apexis/backend/internal/modules/organization/domain/errors"
	"apexis/backend/internal/modules/organization/domain/services"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
)

// DocumentTemplateData คือแม่แบบเอกสารพิมพ์ของบริษัท (HTML + Handlebars)
// แม่แบบมาตรฐานของระบบอยู่ในโค้ดหน้าเว็บ — ตารางนี้เก็บเฉพาะแม่แบบที่บริษัทสร้าง / นำเข้า
type DocumentTemplateData struct {
	DocType   string
	Name      string
	HTML      string
	Paper     vo.Paper
	IsDefault bool
	Status    vo.Status
}

type DocumentTemplate struct {
	id        vo.DocumentTemplateID
	companyID vo.CompanyID
	data      DocumentTemplateData
}

func NewDocumentTemplate(companyID vo.CompanyID, d DocumentTemplateData) (*DocumentTemplate, error) {
	n, err := normalizeTemplate(d)
	if err != nil {
		return nil, err
	}
	return &DocumentTemplate{id: vo.NewID[vo.DocumentTemplateID](), companyID: companyID, data: n}, nil
}

func RehydrateDocumentTemplate(id vo.DocumentTemplateID, companyID vo.CompanyID, d DocumentTemplateData) *DocumentTemplate {
	return &DocumentTemplate{id: id, companyID: companyID, data: d}
}

func (t *DocumentTemplate) ID() vo.DocumentTemplateID  { return t.id }
func (t *DocumentTemplate) CompanyID() vo.CompanyID    { return t.companyID }
func (t *DocumentTemplate) Data() DocumentTemplateData { return t.data }

// Update แทนที่ชื่อ / HTML / กระดาษ / สถานะ — ชนิดเอกสารเปลี่ยนไม่ได้, การเป็นค่าเริ่มต้นเปลี่ยนผ่าน SetDefault เท่านั้น
func (t *DocumentTemplate) Update(d DocumentTemplateData) error {
	d.DocType, d.IsDefault = t.data.DocType, t.data.IsDefault
	n, err := normalizeTemplate(d)
	if err != nil {
		return err
	}
	if n.Status != vo.StatusActive {
		n.IsDefault = false // แม่แบบที่ปิดใช้งานเป็นค่าเริ่มต้นไม่ได้
	}
	t.data = n
	return nil
}

func normalizeTemplate(d DocumentTemplateData) (DocumentTemplateData, error) {
	var err error
	if d.DocType, err = vo.ParseTemplateDocType(d.DocType); err != nil {
		return d, err
	}
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || utf8.RuneCountInString(d.Name) > 100 {
		return d, domain.ErrInvalidName
	}
	if err := services.CheckTemplateHTML(d.HTML); err != nil {
		return d, err
	}
	if d.Paper == "" {
		d.Paper = vo.PaperA4Portrait
	}
	if d.Status == "" {
		d.Status = vo.StatusActive
	}
	return d, nil
}

// DocumentAsset คือรูปที่แม่แบบอ้างถึงด้วยชื่อ ({{asset "logo"}}) — ชนิดไฟล์ตรวจจากเนื้อไฟล์จริง
type DocumentAsset struct {
	ID          vo.DocumentAssetID
	CompanyID   vo.CompanyID
	Name        string
	ContentType string
	Data        []byte
}

func NewDocumentAsset(companyID vo.CompanyID, name string, data []byte) (*DocumentAsset, error) {
	name = strings.TrimSpace(name)
	if err := services.CheckAssetName(name); err != nil {
		return nil, err
	}
	ct, err := services.DetectAssetType(data)
	if err != nil {
		return nil, err
	}
	return &DocumentAsset{ID: vo.NewID[vo.DocumentAssetID](), CompanyID: companyID, Name: name, ContentType: ct, Data: data}, nil
}
