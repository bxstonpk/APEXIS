package handlers

import (
	"context"
	"net/http"
	"strings"

	"apexis/backend/internal/modules/identity/application/ports"
	"apexis/backend/internal/modules/identity/application/usecases"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	"apexis/backend/internal/modules/identity/domain/services"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
)

type ctxKey int

const (
	userIDKey ctxKey = iota
	accessKey
)

// UserIDFrom คืน user id ที่ผ่าน Authenticate แล้ว
func UserIDFrom(ctx context.Context) (vo.UserID, bool) {
	id, ok := ctx.Value(userIDKey).(vo.UserID)
	return id, ok
}

// AccessFrom คืนสิทธิ์ในบริษัทปัจจุบัน (ต้องผ่าน RequireCompany ก่อน)
// module อื่นของ ERP ใช้ตัวนี้เอา CompanyID ไป filter query และเช็ค permission
func AccessFrom(ctx context.Context) (services.AccessContext, bool) {
	a, ok := ctx.Value(accessKey).(services.AccessContext)
	return a, ok
}

type Middleware struct {
	tokens ports.TokenService
	access *usecases.AccessResolver
}

func NewMiddleware(tokens ports.TokenService, access *usecases.AccessResolver) *Middleware {
	return &Middleware{tokens: tokens, access: access}
}

// Authenticate ตรวจ "Authorization: Bearer <token>" แล้วใส่ user id ลง context
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			writeError(w, r, domain.ErrUnauthenticated)
			return
		}
		userID, err := m.tokens.Verify(strings.TrimSpace(token))
		if err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	})
}

// RequireCompany resolve สิทธิ์ของ user ในบริษัทจาก path "{companyID}"
// แล้วใส่ AccessContext ลง context (ต้องใช้หลัง Authenticate)
func (m *Middleware) RequireCompany(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFrom(r.Context())
		if !ok {
			writeError(w, r, domain.ErrUnauthenticated)
			return
		}
		companyID, err := pathCompanyID(r)
		if err != nil {
			writeError(w, r, err)
			return
		}
		access, err := m.access.Resolve(r.Context(), userID, companyID)
		if err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accessKey, access)))
	})
}

// RequirePermission ตรวจ permission จาก AccessContext (ต้องใช้หลัง RequireCompany)
//
//	mux.Handle("POST /api/v1/companies/{companyID}/journals",
//	    mw.Authenticate(mw.RequireCompany(mw.RequirePermission("accounting.journal:create", h))))
func (m *Middleware) RequirePermission(perm vo.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access, ok := AccessFrom(r.Context())
		if !ok {
			writeError(w, r, domain.ErrForbidden)
			return
		}
		if err := access.Require(perm); err != nil {
			writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
