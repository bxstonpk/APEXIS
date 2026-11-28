// Package models คือ "รูปร่างตาราง" ของ organization สำหรับ gorm เท่านั้น — ไม่มี business rule
//
// ข้อตกลง:
//   - organization_companies คือตาราง tenant ของทั้งระบบ — ตารางอื่นของ organization มี company_id + FK มาที่นี่
//   - module อื่น (เช่น identity) เก็บ company_id โดยไม่ใส่ FK ข้าม module
//     เพื่อให้แต่ละ module ย้าย / แยก schema ได้อิสระ
//   - master data ใช้ soft delete (deleted_at) — รหัสไม่ซ้ำเฉพาะแถวที่ยังไม่ถูกลบ (partial unique index)
//   - FK ภายใน module ใช้ ON DELETE RESTRICT, CHECK constraint บังคับค่า status / ช่วงวันที่
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AddressColumns ฝังในตารางที่มีที่อยู่ (prefix address_)
type AddressColumns struct {
	Line        string `gorm:"size:300;not null;default:''"`
	SubDistrict string `gorm:"size:100;not null;default:''"`
	District    string `gorm:"size:100;not null;default:''"`
	Province    string `gorm:"size:100;not null;default:''"`
	PostalCode  string `gorm:"size:20;not null;default:''"`
	Country     string `gorm:"size:2;not null;default:'TH'"`
}

// CompanyModel: บริษัทไม่มี soft delete (ปิดใช้งานด้วย status แทน) — รหัสไม่ซ้ำทั้งระบบ
type CompanyModel struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Code                 string         `gorm:"size:20;not null;uniqueIndex:uq_org_companies_code"`
	Name                 string         `gorm:"size:200;not null"`
	TaxID                string         `gorm:"size:20;not null;default:''"`
	LegalNameTH          string         `gorm:"column:legal_name_th;size:300;not null"`
	LegalNameEN          string         `gorm:"column:legal_name_en;size:300;not null;default:''"`
	Address              AddressColumns `gorm:"embedded;embeddedPrefix:address_"`
	Phone                string         `gorm:"size:30;not null;default:''"`
	Email                string         `gorm:"size:320;not null;default:''"`
	Website              string         `gorm:"size:300;not null;default:''"`
	BaseCurrency         string         `gorm:"size:3;not null"`
	FiscalYearStartMonth int            `gorm:"not null;check:chk_org_companies_fy_month,fiscal_year_start_month BETWEEN 1 AND 12"`
	AmountDecimals       int            `gorm:"not null;default:2;check:chk_org_companies_amount_decimals,amount_decimals BETWEEN 0 AND 6"`
	QuantityDecimals     int            `gorm:"not null;default:2;check:chk_org_companies_quantity_decimals,quantity_decimals BETWEEN 0 AND 6"`
	// BusinessType: default 'trading' ใช้เติมค่าให้บริษัทเดิมตอน migrate (บริษัทเดิมได้ผังบัญชีที่มีบัญชีสินค้าแล้ว) — บริษัทใหม่ต้องระบุเสมอ
	BusinessType string    `gorm:"size:20;not null;default:'trading';check:chk_org_companies_business_type,business_type IN ('service','trading','manufacturing','construction')"`
	Status       string    `gorm:"size:20;not null;check:chk_org_companies_status,status IN ('active','inactive')"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (CompanyModel) TableName() string { return "organization_companies" }

type BranchModel struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_org_branches_code,priority:1,where:deleted_at IS NULL;uniqueIndex:uq_org_branches_tax_no,priority:1,where:deleted_at IS NULL"`
	Company     *CompanyModel  `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code        string         `gorm:"size:20;not null;uniqueIndex:uq_org_branches_code,priority:2,where:deleted_at IS NULL"`
	Name        string         `gorm:"size:200;not null"`
	TaxBranchNo string         `gorm:"size:5;not null;uniqueIndex:uq_org_branches_tax_no,priority:2,where:deleted_at IS NULL"`
	Address     AddressColumns `gorm:"embedded;embeddedPrefix:address_"`
	Phone       string         `gorm:"size:30;not null;default:''"`
	Status      string         `gorm:"size:20;not null;check:chk_org_branches_status,status IN ('active','inactive')"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (BranchModel) TableName() string { return "organization_branches" }

type BusinessUnitModel struct {
	ID        uuid.UUID          `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:uq_org_business_units_code,priority:1,where:deleted_at IS NULL"`
	Company   *CompanyModel      `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code      string             `gorm:"size:20;not null;uniqueIndex:uq_org_business_units_code,priority:2,where:deleted_at IS NULL"`
	Name      string             `gorm:"size:200;not null"`
	ParentID  *uuid.UUID         `gorm:"type:uuid;index"`
	Parent    *BusinessUnitModel `gorm:"foreignKey:ParentID;constraint:OnDelete:RESTRICT"`
	Status    string             `gorm:"size:20;not null;check:chk_org_business_units_status,status IN ('active','inactive')"`
	CreatedAt time.Time          `gorm:"not null"`
	UpdatedAt time.Time          `gorm:"not null"`
	DeletedAt gorm.DeletedAt     `gorm:"index"`
}

func (BusinessUnitModel) TableName() string { return "organization_business_units" }

type DepartmentModel struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey"`
	CompanyID      uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:uq_org_departments_code,priority:1,where:deleted_at IS NULL"`
	Company        *CompanyModel      `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code           string             `gorm:"size:20;not null;uniqueIndex:uq_org_departments_code,priority:2,where:deleted_at IS NULL"`
	Name           string             `gorm:"size:200;not null"`
	ParentID       *uuid.UUID         `gorm:"type:uuid;index"`
	Parent         *DepartmentModel   `gorm:"foreignKey:ParentID;constraint:OnDelete:RESTRICT"`
	BusinessUnitID *uuid.UUID         `gorm:"type:uuid;index"`
	BusinessUnit   *BusinessUnitModel `gorm:"foreignKey:BusinessUnitID;constraint:OnDelete:RESTRICT"`
	BranchID       *uuid.UUID         `gorm:"type:uuid;index"`
	Branch         *BranchModel       `gorm:"foreignKey:BranchID;constraint:OnDelete:RESTRICT"`
	Status         string             `gorm:"size:20;not null;check:chk_org_departments_status,status IN ('active','inactive')"`
	CreatedAt      time.Time          `gorm:"not null"`
	UpdatedAt      time.Time          `gorm:"not null"`
	DeletedAt      gorm.DeletedAt     `gorm:"index"`
}

func (DepartmentModel) TableName() string { return "organization_departments" }

type CostCenterModel struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey"`
	CompanyID    uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:uq_org_cost_centers_code,priority:1,where:deleted_at IS NULL"`
	Company      *CompanyModel    `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code         string           `gorm:"size:20;not null;uniqueIndex:uq_org_cost_centers_code,priority:2,where:deleted_at IS NULL"`
	Name         string           `gorm:"size:200;not null"`
	DepartmentID *uuid.UUID       `gorm:"type:uuid;index"`
	Department   *DepartmentModel `gorm:"foreignKey:DepartmentID;constraint:OnDelete:RESTRICT"`
	ValidFrom    time.Time        `gorm:"type:date;not null"`
	ValidTo      *time.Time       `gorm:"type:date;check:chk_org_cost_centers_valid,valid_to IS NULL OR valid_to >= valid_from"`
	Status       string           `gorm:"size:20;not null;check:chk_org_cost_centers_status,status IN ('active','inactive')"`
	CreatedAt    time.Time        `gorm:"not null"`
	UpdatedAt    time.Time        `gorm:"not null"`
	DeletedAt    gorm.DeletedAt   `gorm:"index"`
}

func (CostCenterModel) TableName() string { return "organization_cost_centers" }

type ProjectSiteModel struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey"`
	CompanyID    uuid.UUID        `gorm:"type:uuid;not null;uniqueIndex:uq_org_project_sites_code,priority:1,where:deleted_at IS NULL"`
	Company      *CompanyModel    `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Code         string           `gorm:"size:20;not null;uniqueIndex:uq_org_project_sites_code,priority:2,where:deleted_at IS NULL"`
	Name         string           `gorm:"size:200;not null"`
	BranchID     *uuid.UUID       `gorm:"type:uuid;index"`
	Branch       *BranchModel     `gorm:"foreignKey:BranchID;constraint:OnDelete:RESTRICT"`
	CostCenterID *uuid.UUID       `gorm:"type:uuid;index"`
	CostCenter   *CostCenterModel `gorm:"foreignKey:CostCenterID;constraint:OnDelete:RESTRICT"`
	Address      AddressColumns   `gorm:"embedded;embeddedPrefix:address_"`
	StartDate    time.Time        `gorm:"type:date;not null"`
	EndDate      *time.Time       `gorm:"type:date;check:chk_org_project_sites_dates,end_date IS NULL OR end_date >= start_date"`
	Status       string           `gorm:"size:20;not null;check:chk_org_project_sites_status,status IN ('planned','active','closed')"`
	CreatedAt    time.Time        `gorm:"not null"`
	UpdatedAt    time.Time        `gorm:"not null"`
	DeletedAt    gorm.DeletedAt   `gorm:"index"`
}

func (ProjectSiteModel) TableName() string { return "organization_project_sites" }

// FiscalYearModel: ปีบัญชีไม่มี soft delete (ข้อมูลบัญชีห้ามหาย)
type FiscalYearModel struct {
	ID        uuid.UUID           `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex:uq_org_fiscal_years_year,priority:1"`
	Company   *CompanyModel       `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Year      int                 `gorm:"not null;uniqueIndex:uq_org_fiscal_years_year,priority:2"`
	StartDate time.Time           `gorm:"type:date;not null"`
	EndDate   time.Time           `gorm:"type:date;not null;check:chk_org_fiscal_years_dates,end_date > start_date"`
	Periods   []FiscalPeriodModel `gorm:"foreignKey:FiscalYearID;constraint:OnDelete:RESTRICT"`
	CreatedAt time.Time           `gorm:"not null"`
	UpdatedAt time.Time           `gorm:"not null"`
}

func (FiscalYearModel) TableName() string { return "organization_fiscal_years" }

type FiscalPeriodModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	FiscalYearID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_org_fiscal_periods_no,priority:1"`
	PeriodNo     int       `gorm:"not null;uniqueIndex:uq_org_fiscal_periods_no,priority:2;check:chk_org_fiscal_periods_no,period_no BETWEEN 1 AND 12"`
	StartDate    time.Time `gorm:"type:date;not null;index:idx_org_fiscal_periods_dates,priority:1"`
	EndDate      time.Time `gorm:"type:date;not null;index:idx_org_fiscal_periods_dates,priority:2;check:chk_org_fiscal_periods_dates,end_date >= start_date"`
	Status       string    `gorm:"size:20;not null;check:chk_org_fiscal_periods_status,status IN ('open','closed')"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (FiscalPeriodModel) TableName() string { return "organization_fiscal_periods" }

// AutoMigrate สร้าง/อัปเดตตารางของ organization (เรียงตาม dependency ของ FK)
// สำหรับ production แนะนำเปลี่ยนเป็น versioned migration (เช่น goose / golang-migrate)
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&CompanyModel{},
		&BranchModel{},
		&BusinessUnitModel{},
		&DepartmentModel{},
		&CostCenterModel{},
		&ProjectSiteModel{},
		&FiscalYearModel{},
		&FiscalPeriodModel{},
		&NumberingModel{},
		&DocumentTemplateModel{},
		&DocumentAssetModel{},
	)
}

// NumberingModel: รูปแบบเลขที่เอกสารที่บริษัทตั้งเอง (ไม่มีแถว = ใช้ค่าเริ่มต้นของชนิดเอกสาร)
// doc_type คือ key ที่ module ลงทะเบียน เช่น "purchasing.payment" — ลำดับเลขเก็บที่ module เจ้าของเอกสาร
type NumberingModel struct {
	CompanyID uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Company   *CompanyModel `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	DocType   string        `gorm:"size:50;primaryKey"`
	Prefix    string        `gorm:"size:10;not null;default:''"`
	Template  string        `gorm:"size:60;not null"`
	UpdatedAt time.Time     `gorm:"not null"`
}

func (NumberingModel) TableName() string { return "organization_document_numberings" }

// DocumentTemplateModel: แม่แบบเอกสารพิมพ์ที่บริษัทสร้าง (HTML + Handlebars)
// ค่าเริ่มต้นได้ 1 แม่แบบต่อ (บริษัท, ชนิดเอกสาร) — partial unique index; ค้นรายการตาม (company_id, doc_type)
type DocumentTemplateModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CompanyID uuid.UUID      `gorm:"type:uuid;not null;index:idx_org_doc_templates_type,priority:1;uniqueIndex:uq_org_doc_templates_default,priority:1,where:is_default AND deleted_at IS NULL"`
	Company   *CompanyModel  `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	DocType   string         `gorm:"size:50;not null;index:idx_org_doc_templates_type,priority:2;uniqueIndex:uq_org_doc_templates_default,priority:2,where:is_default AND deleted_at IS NULL"`
	Name      string         `gorm:"size:100;not null"`
	HTML      string         `gorm:"type:text;not null"`
	Paper     string         `gorm:"size:20;not null;check:chk_org_doc_templates_paper,paper IN ('a4_portrait','a4_landscape','a5_portrait')"`
	IsDefault bool           `gorm:"not null;default:false"`
	Status    string         `gorm:"size:20;not null;check:chk_org_doc_templates_status,status IN ('active','inactive')"`
	CreatedBy uuid.UUID      `gorm:"type:uuid;not null"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (DocumentTemplateModel) TableName() string { return "organization_document_templates" }

// DocumentAssetModel: รูปของแม่แบบ (โลโก้ / ลายเซ็น / ตราประทับ) เก็บใน DB — ชื่อไม่ซ้ำในบริษัท, ลบจริง (ไม่มีข้อมูลธุรกิจอ้างถึง)
type DocumentAssetModel struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey"`
	CompanyID   uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex:uq_org_doc_assets_name,priority:1"`
	Company     *CompanyModel `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
	Name        string        `gorm:"size:40;not null;uniqueIndex:uq_org_doc_assets_name,priority:2"`
	ContentType string        `gorm:"size:40;not null;check:chk_org_doc_assets_type,content_type IN ('image/png','image/jpeg','image/svg+xml')"`
	Size        int           `gorm:"not null;check:chk_org_doc_assets_size,size > 0 AND size <= 512000"`
	Data        []byte        `gorm:"type:bytea;not null"`
	CreatedAt   time.Time     `gorm:"not null"`
	UpdatedAt   time.Time     `gorm:"not null"`
}

func (DocumentAssetModel) TableName() string { return "organization_document_assets" }
