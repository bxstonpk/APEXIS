// Package responses คือรูปแบบ JSON ของ taxregistry
package responses

import (
	"apexis/backend/internal/modules/taxregistry/domain/entities"
	"apexis/backend/internal/platform/thaiaddress"
)

type Registrant struct {
	TaxID         string           `json:"tax_id"`
	BranchNo      string           `json:"branch_no"`
	BranchName    string           `json:"branch_name"`
	Name          string           `json:"name"`
	TaxType       string           `json:"tax_type"`
	Phone         string           `json:"phone"`
	VatRegistered string           `json:"vat_registered_on"`
	Address       string           `json:"address"`
	AddressDetail thaiaddress.JSON `json:"address_detail"`
}

func FromRegistrant(r entities.VatRegistrant) Registrant {
	return Registrant{
		TaxID: r.TaxID, BranchNo: r.BranchNo, BranchName: r.DisplayBranch(), Name: r.Name(), TaxType: r.TaxType(),
		Phone: r.Phone, VatRegistered: r.RegisteredOn, Address: r.Address.Format(), AddressDetail: thaiaddress.ToJSON(r.Address),
	}
}
