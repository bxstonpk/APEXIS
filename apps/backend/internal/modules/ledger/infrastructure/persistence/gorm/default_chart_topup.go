package gormpersistence

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/domain/entities"
	vo "apexis/backend/internal/modules/ledger/domain/value_objects"
	"apexis/backend/internal/modules/ledger/infrastructure/persistence/gorm/models"
)

// addedStandardCodes คือบัญชีที่เพิ่มเข้าผังมาตรฐานภายหลัง (สินค้าคงเหลือ / ต้นทุนขาย / รายได้จากการขาย ตาม TAS 2)
// เรียงบัญชีคุมก่อนบัญชีย่อย
var addedStandardCodes = []string{
	"1105040000", "1105050000", "1105060000", "1105070000", "1105080000", "1105090000",
	"2102030000",
	"4050000000", "4051000000", "4052000000",
	"5050000000", "5051000000", "5052000000", "5053000000", "5054000000", "5055000000", "5056000000",
}

// topUpBatch คือบัญชีมาตรฐานชุดที่เพิ่มภายหลัง: ให้บริษัทประเภท Types ที่มีบัญชี Marker (ใช้ผังมาตรฐาน)
// และยังไม่มีรหัสในชุดนี้เลยสักรหัส (นับรวมที่ลบแล้ว)
type topUpBatch struct {
	Marker string
	Types  []vo.BusinessType
	Codes  []string // เรียงบัญชีคุมก่อนบัญชีย่อย
}

var topUpBatches = []topUpBatch{
	// สินค้าคงเหลือ / ต้นทุนขาย / รายได้จากการขาย ตาม TAS 2
	{Marker: "1105000000", Types: []vo.BusinessType{vo.BusinessTrading, vo.BusinessManufacturing}, Codes: addedStandardCodes},
	// ค่าแรงและค่าใช้จ่ายการผลิตปันเข้างาน (แอปผลิต) — บริษัทผลิตที่สร้างก่อนมีบัญชีนี้
	{Marker: "5060000000", Types: []vo.BusinessType{vo.BusinessManufacturing}, Codes: []string{"5065000000"}},
}

// TopUpDefaultChart เพิ่มบัญชีมาตรฐานใหม่ให้บริษัทที่ใช้ผังมาตรฐานอยู่แล้ว ทีละชุด (topUpBatches)
// เฉพาะบริษัทประเภทที่ชุดนั้นกำหนด และเฉพาะบัญชีที่อยู่ในผังของประเภทนั้น (entities.InChart)
//
// ข้อยกเว้นของการแยก module: migration นี้อ่าน organization_companies.business_type โดยตรง
// เพราะทำงานตอนเริ่มโปรแกรม (ก่อนมี module / use case) และเป็นการปรับข้อมูลครั้งเดียว — อ่านอย่างเดียว ไม่แก้ตารางของ organization
// ถ้ายังไม่มี column (organization ยังไม่ migrate) จะข้ามทั้งหมด (บริษัทบริการ / ก่อสร้างต้องไม่ได้บัญชีสินค้า)
//
// ทำเฉพาะบริษัทที่ยังไม่มีรหัสของชุดเลยสักรหัส (นับรวมที่ลบแล้ว) — บริษัทที่ได้รับไปแล้ว / ลบทิ้งเอง / สร้างรหัสซ้ำเองจะไม่ถูกแตะ
// บัญชีที่หา parent ไม่เจอ (ผู้ใช้ลบบัญชีคุมทิ้ง) จะข้าม; ทั้งหมดทำในทรานแซกชันเดียว
func TopUpDefaultChart(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.AccountModel{}) {
		return nil
	}
	defs := make(map[string]entities.DefaultAccount, len(entities.DefaultChart))
	for _, d := range entities.DefaultChart {
		defs[d.Code] = d
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasColumn("organization_companies", "business_type") {
			return nil
		}
		for _, b := range topUpBatches {
			if err := topUp(tx, defs, b); err != nil {
				return err
			}
		}
		return nil
	})
}

func topUp(tx *gorm.DB, defs map[string]entities.DefaultAccount, b topUpBatch) error {
	types := make([]string, len(b.Types))
	for i, t := range b.Types {
		types[i] = string(t)
	}
	var companies []struct {
		CompanyID    uuid.UUID
		BusinessType string
	}
	if err := tx.Table("ledger_accounts AS a").
		Select("DISTINCT a.company_id, c.business_type").
		Joins("JOIN organization_companies c ON c.id = a.company_id").
		Where("a.code = ? AND a.deleted_at IS NULL", b.Marker).
		Where("c.business_type IN ?", types).
		Where("a.company_id NOT IN (?)", tx.Unscoped().Model(&models.AccountModel{}).Select("company_id").Where("code IN ?", b.Codes)).
		Scan(&companies).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, co := range companies {
		companyID, bt := co.CompanyID, vo.BusinessType(co.BusinessType)
		var rows []models.AccountModel
		if err := tx.Select("id", "code").Where("company_id = ? AND deleted_at IS NULL", companyID).Find(&rows).Error; err != nil {
			return err
		}
		ids := make(map[string]uuid.UUID, len(rows))
		for _, r := range rows {
			ids[r.Code] = r.ID
		}
		for _, code := range b.Codes {
			if !entities.InChart(code, bt) {
				continue
			}
			d := defs[code]
			parent, ok := ids[d.ParentCode]
			if !ok {
				continue
			}
			m := models.AccountModel{
				ID: uuid.New(), CompanyID: companyID, Code: d.Code, Name: d.Name, NameEN: d.NameEN, Type: string(d.Type),
				ParentID: &parent, Postable: d.Postable, Status: "active", Description: d.Description, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			ids[code] = m.ID
		}
	}
	return nil
}
