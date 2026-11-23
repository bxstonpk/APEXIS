package taxregistry_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"apexis/backend/internal/modules/taxregistry/application/dto"
	"apexis/backend/internal/modules/taxregistry/application/usecases"
	domain "apexis/backend/internal/modules/taxregistry/domain/errors"
	vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"
	"apexis/backend/internal/modules/taxregistry/infrastructure/rd"
	"apexis/backend/internal/modules/taxregistry/infrastructure/services"
)

// ตัวอย่างคำตอบจริงของกรมสรรพากร (ย่อ) — สาขา 00001 มาก่อนเพื่อทดสอบการเรียง
const found = `{"data":[
 {"nid":"0745569002816","title":"บริษัท","firnam":"ตัวอย่าง จำกัด","brano":"00001","bratitle":"สาขา","branam":"มหาชัย",
  "addno":"9","moono":"-","soinam":"-","thnnam":"เศรษฐกิจ 1","tamnam":"มหาชัย","ampnam":"เมืองสมุทรสาคร","provnam":"สมุทรสาคร","poscod":"74000"},
 {"nid":"0745569002816","title":"บริษัท","firnam":"ตัวอย่าง จำกัด","brano":"00000","bratitle":"บริษัท","branam":"ตัวอย่าง จำกัด",
  "vataprdat":"21/09/2569","roomno":"-","floorno":"-","addno":"72/7","moono":"3","soinam":"-","thnnam":"-",
  "tamnam":"ท่าทราย","ampnam":"เมืองสมุทรสาคร","provnam":"สมุทรสาคร","poscod":"74000","telno":"0614811659"}],
 "status":"000","errdetail":""}`

func fakeRD(t *testing.T, status int, body string, seen *map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			_ = json.NewDecoder(r.Body).Decode(seen)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type allow struct{ ok bool }

func (a allow) Authorize(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return a.ok, nil
}

func lookup(url string, permitted bool) *usecases.Lookup {
	return usecases.NewLookup(services.NewAuthorizer(allow{permitted}, []string{"sales.customer:manage"}), rd.New(url))
}

func input(taxID string) dto.LookupInput {
	return dto.LookupInput{UserID: vo.UserID(uuid.New()), CompanyID: vo.CompanyID(uuid.New()), TaxID: taxID}
}

func TestLookupFound(t *testing.T) {
	var seen map[string]string
	srv := fakeRD(t, 200, found, &seen)
	rows, err := lookup(srv.URL, true).Execute(context.Background(), input("0745569002816"))
	if err != nil {
		t.Fatal(err)
	}
	if seen["nid"] != "0745569002816" || seen["searchType"] != "1" || seen["brano"] != "" {
		t.Fatalf("request body %v", seen)
	}
	if len(rows) != 2 || rows[0].BranchNo != "00000" {
		t.Fatalf("head office first: %+v", rows)
	}
	h := rows[0]
	if h.Name() != "บริษัท ตัวอย่าง จำกัด" || h.TaxType() != "juristic" || h.Phone != "0614811659" {
		t.Fatalf("registrant %+v", h)
	}
	if h.Address.Soi != "" || h.Address.Street != "" {
		t.Fatalf(`"-" must be empty: %+v`, h.Address)
	}
	if got := h.Address.Format(); got != "เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000" {
		t.Fatalf("address %q", got)
	}
	if rows[1].DisplayBranch() != "สาขา มหาชัย" {
		t.Fatalf("branch %q", rows[1].DisplayBranch())
	}
}

func TestLookupErrors(t *testing.T) {
	notFound := fakeRD(t, 200, `{"status":"001","errdetail":"Not Found Data"}`, nil)
	broken := fakeRD(t, 502, `bad gateway`, nil)
	cases := []struct {
		name      string
		url       string
		taxID     string
		permitted bool
		want      error
	}{
		{"not found", notFound.URL, "0745569002816", true, domain.ErrNotFound},
		{"service down", broken.URL, "0745569002816", true, domain.ErrUnavailable},
		{"invalid tax id", notFound.URL, "12345", true, domain.ErrInvalidTaxID},
		{"no permission", notFound.URL, "0745569002816", false, domain.ErrForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := lookup(c.url, c.permitted).Execute(context.Background(), input(c.taxID))
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v want %v", err, c.want)
			}
		})
	}
}
