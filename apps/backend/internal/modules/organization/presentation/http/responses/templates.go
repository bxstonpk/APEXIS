package responses

import (
	"time"

	"apexis/backend/internal/modules/organization/application/dto"
)

// DocumentTemplate: รายการไม่ส่ง html (ส่งเฉพาะ GET รายตัว / default / หลังบันทึก)
type DocumentTemplate struct {
	ID        string    `json:"id"`
	DocType   string    `json:"doc_type"`
	Name      string    `json:"name"`
	HTML      *string   `json:"html,omitempty"`
	Paper     string    `json:"paper"`
	IsDefault bool      `json:"is_default"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func FromDocumentTemplateSummary(t dto.DocumentTemplate) DocumentTemplate {
	return DocumentTemplate{
		ID: t.ID, DocType: t.DocType, Name: t.Name, Paper: t.Paper, IsDefault: t.IsDefault, Status: t.Status,
		CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func FromDocumentTemplate(t dto.DocumentTemplate) DocumentTemplate {
	out := FromDocumentTemplateSummary(t)
	html := t.HTML
	out.HTML = &html
	return out
}

type DocumentAsset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int       `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

func FromDocumentAsset(a dto.DocumentAsset) DocumentAsset {
	return DocumentAsset{ID: a.ID, Name: a.Name, ContentType: a.ContentType, Size: a.Size, CreatedAt: a.CreatedAt}
}
