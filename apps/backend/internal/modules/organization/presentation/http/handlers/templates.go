package handlers

import (
	"net/http"
	"strconv"

	"apexis/backend/internal/modules/organization/application/dto"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/presentation/http/requests"
	"apexis/backend/internal/modules/organization/presentation/http/responses"
	"apexis/backend/internal/platform/httpx"
)

// GET /api/v1/companies/{companyID}/document-templates?doc_type=&q=&status=&page=&page_size= (ไม่ส่ง html)
func (h *Handler) ListDocumentTemplates(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		f, err := requests.ParseDocumentTemplateFilter(r.URL.Query())
		if err != nil {
			return nil, err
		}
		res, err := h.uc.Templates.List(r.Context(), a, f)
		return httpx.ToList(res, responses.FromDocumentTemplateSummary), err
	})
}

// GET /api/v1/companies/{companyID}/document-templates/default?doc_type= — ไม่มี = 404 (ใช้แม่แบบมาตรฐานของระบบ)
func (h *Handler) GetDefaultDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		t, err := h.uc.Templates.Default(r.Context(), a, r.URL.Query().Get("doc_type"))
		return one(responses.FromDocumentTemplate(t), err)
	})
}

// GET /api/v1/companies/{companyID}/document-templates/{templateID}
func (h *Handler) GetDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentTemplateID](r, "templateID")
		if err != nil {
			return nil, err
		}
		t, err := h.uc.Templates.Get(r.Context(), a, id)
		return one(responses.FromDocumentTemplate(t), err)
	})
}

// POST /api/v1/companies/{companyID}/document-templates
func (h *Handler) CreateDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.DocumentTemplate](w, r)
		if err != nil {
			return nil, err
		}
		t, err := h.uc.Templates.Create(r.Context(), req.ToInput(a))
		return one(responses.FromDocumentTemplate(t), err)
	})
}

// PUT /api/v1/companies/{companyID}/document-templates/{templateID}
func (h *Handler) UpdateDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentTemplateID](r, "templateID")
		if err != nil {
			return nil, err
		}
		req, err := decode[requests.DocumentTemplate](w, r)
		if err != nil {
			return nil, err
		}
		t, err := h.uc.Templates.Update(r.Context(), id, req.ToInput(a))
		return one(responses.FromDocumentTemplate(t), err)
	})
}

// PUT /api/v1/companies/{companyID}/document-templates/{templateID}/default
func (h *Handler) SetDefaultDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentTemplateID](r, "templateID")
		if err != nil {
			return nil, err
		}
		t, err := h.uc.Templates.SetDefault(r.Context(), a, id)
		return one(responses.FromDocumentTemplate(t), err)
	})
}

// DELETE /api/v1/companies/{companyID}/document-templates/{templateID}
func (h *Handler) DeleteDocumentTemplate(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentTemplateID](r, "templateID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Templates.Delete(r.Context(), a, id)
	})
}

// GET /api/v1/companies/{companyID}/document-assets (ไม่ส่งเนื้อไฟล์)
func (h *Handler) ListDocumentAssets(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusOK, func(a dto.Actor) (any, error) {
		list, err := h.uc.Templates.Assets(r.Context(), a)
		if err != nil {
			return nil, err
		}
		out := make([]responses.DocumentAsset, len(list))
		for i, x := range list {
			out[i] = responses.FromDocumentAsset(x)
		}
		return httpx.Data[[]responses.DocumentAsset]{Data: out}, nil
	})
}

// POST /api/v1/companies/{companyID}/document-assets {name, data(base64)}
func (h *Handler) UploadDocumentAsset(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusCreated, func(a dto.Actor) (any, error) {
		req, err := decode[requests.DocumentAsset](w, r)
		if err != nil {
			return nil, err
		}
		data, err := req.Bytes()
		if err != nil {
			return nil, err
		}
		x, err := h.uc.Templates.UploadAsset(r.Context(), a, req.Name, data)
		return one(responses.FromDocumentAsset(x), err)
	})
}

// DELETE /api/v1/companies/{companyID}/document-assets/{assetID}
func (h *Handler) DeleteDocumentAsset(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, http.StatusNoContent, func(a dto.Actor) (any, error) {
		id, err := pathID[vo.DocumentAssetID](r, "assetID")
		if err != nil {
			return nil, err
		}
		return nil, h.uc.Templates.DeleteAsset(r.Context(), a, id)
	})
}

// GET /api/v1/companies/{companyID}/document-assets/{assetID}/content — ตัวไฟล์รูป
//
// ส่ง CSP ที่ห้ามสคริปต์ / ทรัพยากรภายนอกเสมอ (กัน SVG ที่หลุดการตรวจ) + nosniff; cache ส่วนตัว 1 วัน (ETag = เวลาแก้ไข)
func (h *Handler) DocumentAssetContent(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(r.Context())
	if !ok {
		errorMapper.Write(w, r, errUnauthenticated)
		return
	}
	companyID, err := pathID[vo.CompanyID](r, "companyID")
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	id, err := pathID[vo.DocumentAssetID](r, "assetID")
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	c, err := h.uc.Templates.AssetContent(r.Context(), dto.Actor{UserID: vo.UserID(userID), CompanyID: companyID}, id)
	if err != nil {
		errorMapper.Write(w, r, err)
		return
	}
	etag := `"` + strconv.FormatInt(c.UpdatedAt.UnixNano(), 36) + `"`
	hd := w.Header()
	hd.Set("Content-Type", c.ContentType)
	hd.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Cache-Control", "private, max-age=86400")
	hd.Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	hd.Set("Content-Length", strconv.Itoa(len(c.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(c.Data)
}
