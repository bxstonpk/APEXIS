// Package dto คือ input ของ use case ใน taxregistry
package dto

import vo "apexis/backend/internal/modules/taxregistry/domain/value_objects"

type LookupInput struct {
	UserID    vo.UserID
	CompanyID vo.CompanyID
	TaxID     string
	BranchNo  string // ว่าง = ทุกสาขา
}
