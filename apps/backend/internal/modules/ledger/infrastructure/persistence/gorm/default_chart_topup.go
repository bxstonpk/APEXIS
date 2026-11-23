package gormpersistence

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/ledger/domain/entities"
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

// TopUpDefaultChart เพิ่มบัญชีมาตรฐานใหม่ให้บริษัทที่ใช้ผังมาตรฐานอยู่แล้ว (มีบัญชี 1105000000)
//
// ทำเฉพาะบริษัทที่ยังไม่มีรหัสชุดใหม่เลยสักรหัส (นับรวมที่ลบแล้ว) — บริษัทที่ได้รับไปแล้ว / ลบทิ้งเอง / สร้างรหัสซ้ำเองจะไม่ถูกแตะ
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
		var companies []uuid.UUID
		if err := tx.Model(&models.AccountModel{}).
			Where("code = ? AND deleted_at IS NULL", "1105000000").
			Where("company_id NOT IN (?)", tx.Unscoped().Model(&models.AccountModel{}).Select("company_id").Where("code IN ?", addedStandardCodes)).
			Distinct().Pluck("company_id", &companies).Error; err != nil {
			return err
		}
		now := time.Now()
		for _, companyID := range companies {
			var rows []models.AccountModel
			if err := tx.Select("id", "code").Where("company_id = ? AND deleted_at IS NULL", companyID).Find(&rows).Error; err != nil {
				return err
			}
			ids := make(map[string]uuid.UUID, len(rows))
			for _, r := range rows {
				ids[r.Code] = r.ID
			}
			for _, code := range addedStandardCodes {
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
	})
}
