package permissions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalog(t *testing.T) {
	c := NewCatalog()
	c.Register("sales.order:read", "sales.order:approve", "identity.role:manage")

	all := c.All()
	if len(all) != 3 || all[1] != (Entry{Code: "sales.order:approve", Module: "sales", Resource: "order", Action: "approve"}) {
		t.Fatalf("all = %+v", all)
	}

	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/permissions", nil))
	var body struct{ Data []Entry }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 || len(body.Data) != 3 || body.Data[2].Module != "identity" {
		t.Fatalf("response %d %s", rec.Code, rec.Body)
	}
}

func TestCatalogRejectsBadOrDuplicateCodes(t *testing.T) {
	for _, codes := range [][]string{{"*"}, {"sales.order"}, {"Sales.order:read"}, {"sales.order:read", "sales.order:read"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for %v", codes)
				}
			}()
			NewCatalog().Register(codes...)
		}()
	}
}
