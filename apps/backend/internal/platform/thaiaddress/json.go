package thaiaddress

// JSON คือรูปแบบ address_detail ใน API (ลูกค้า / ผู้ขาย / ค้นหาผู้จด VAT ใช้รูปแบบเดียวกัน)
type JSON struct {
	Building    string `json:"building"`
	RoomNo      string `json:"room_no"`
	FloorNo     string `json:"floor_no"`
	Village     string `json:"village"`
	HouseNo     string `json:"house_no"`
	Moo         string `json:"moo"`
	Soi         string `json:"soi"`
	Yaek        string `json:"yaek"`
	Street      string `json:"street"`
	SubDistrict string `json:"sub_district"`
	District    string `json:"district"`
	Province    string `json:"province"`
	PostalCode  string `json:"postal_code"`
}

func ToJSON(a Address) JSON { return JSON(a) }

// Address: nil (ไม่ส่ง address_detail) = ที่อยู่ว่าง
func (j *JSON) Address() Address {
	if j == nil {
		return Address{}
	}
	return Address(*j)
}
