// Package taxregistry คือการค้นหาผู้ประกอบการจดทะเบียนภาษีมูลค่าเพิ่มจากฐานข้อมูลกรมสรรพากร
//
// ใช้เติมชื่อ / สาขา / ที่อยู่ (แบบสรรพากร) ในฟอร์มลูกค้าและผู้ขายอัตโนมัติ — ไม่พบหรือบริการล่ม = ผู้ใช้กรอกเอง
// module นี้ไม่มีตาราง: อ่านจากบริการภายนอกอย่างเดียว ข้อมูลถูกบันทึกโดย sales / purchasing เมื่อผู้ใช้กดบันทึก
package taxregistry

import (
	"context"
	nethttp "net/http"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/taxregistry/application/usecases"
	"apexis/backend/internal/modules/taxregistry/infrastructure/rd"
	"apexis/backend/internal/modules/taxregistry/infrastructure/services"
	"apexis/backend/internal/modules/taxregistry/presentation/http/handlers"
)

type Identity interface {
	Authenticate(next nethttp.Handler) nethttp.Handler
	CurrentUserID(ctx context.Context) (uuid.UUID, bool)
	Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
}

type Module struct {
	identity Identity
	handler  *handlers.Handler
}

// New: url ว่าง = endpoint ของกรมสรรพากร; permissions = สิทธิ์ที่ใช้ค้นหาได้ (มีข้อใดข้อหนึ่ง)
func New(identity Identity, url string, permissions []string) *Module {
	lookup := usecases.NewLookup(services.NewAuthorizer(identity, permissions), rd.New(url))
	return &Module{identity: identity, handler: handlers.NewHandler(lookup, identity.CurrentUserID)}
}

func (m *Module) RegisterRoutes(mux *nethttp.ServeMux) {
	handlers.RegisterRoutes(mux, m.handler, m.identity.Authenticate)
}
