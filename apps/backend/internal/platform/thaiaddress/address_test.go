package thaiaddress

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatProvince(t *testing.T) {
	a, err := New(Address{HouseNo: "72/7", Moo: "3", Soi: "-", Street: "-", RoomNo: "-", SubDistrict: "ท่าทราย", District: "เมืองสมุทรสาคร", Province: "สมุทรสาคร", PostalCode: "74000"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Soi != "" || a.Street != "" || a.RoomNo != "" {
		t.Fatalf(`"-" must become empty: %+v`, a)
	}
	want := "เลขที่ 72/7 หมู่ที่ 3 ตำบลท่าทราย อำเภอเมืองสมุทรสาคร จังหวัดสมุทรสาคร 74000"
	if got := a.Format(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatBangkokAndPrefixes(t *testing.T) {
	a := Address{Building: "อาคารสาทรซิตี้", FloorNo: "12", HouseNo: "175", Soi: "ซอยสาทร 1", Street: "สาทรใต้", SubDistrict: "ทุ่งมหาเมฆ", District: "เขตสาทร", Province: "กรุงเทพมหานคร", PostalCode: "10120"}
	want := "อาคารสาทรซิตี้ ชั้นที่ 12 เลขที่ 175 ซอยสาทร 1 ถนนสาทรใต้ แขวงทุ่งมหาเมฆ เขตสาทร กรุงเทพมหานคร 10120"
	if got := a.Format(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !(Address{}).IsEmpty() || a.IsEmpty() {
		t.Fatal("IsEmpty")
	}
}

func TestValidation(t *testing.T) {
	if _, err := New(Address{PostalCode: "7400"}); !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("postal code: %v", err)
	}
	if _, err := New(Address{Street: strings.Repeat("ก", MaxField+1)}); !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("field length: %v", err)
	}
}
