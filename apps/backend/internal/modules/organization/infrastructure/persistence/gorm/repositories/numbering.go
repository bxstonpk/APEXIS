package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	ports "apexis/backend/internal/modules/organization/application/ports/repositories"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/modules/organization/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/numbering"
)

var _ ports.NumberingRepository = (*NumberingRepository)(nil)

type NumberingRepository struct{ db *gorm.DB }

func NewNumberingRepository(db *gorm.DB) *NumberingRepository { return &NumberingRepository{db: db} }

func (r *NumberingRepository) Get(ctx context.Context, companyID vo.CompanyID, docType string) (numbering.Format, bool, error) {
	var ms []models.NumberingModel
	err := database.Conn(ctx, r.db).Where("company_id = ? AND doc_type = ?", uuidOf(companyID), docType).Limit(1).Find(&ms).Error
	if err != nil || len(ms) == 0 {
		return numbering.Format{}, false, err
	}
	return numbering.Format{Prefix: ms[0].Prefix, Template: ms[0].Template}, true, nil
}

func (r *NumberingRepository) List(ctx context.Context, companyID vo.CompanyID) (map[string]numbering.Format, error) {
	var ms []models.NumberingModel
	if err := database.Conn(ctx, r.db).Where("company_id = ?", uuidOf(companyID)).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make(map[string]numbering.Format, len(ms))
	for _, m := range ms {
		out[m.DocType] = numbering.Format{Prefix: m.Prefix, Template: m.Template}
	}
	return out, nil
}

func (r *NumberingRepository) Save(ctx context.Context, companyID vo.CompanyID, docType string, f numbering.Format) error {
	m := models.NumberingModel{CompanyID: uuidOf(companyID), DocType: docType, Prefix: f.Prefix, Template: f.Template, UpdatedAt: time.Now().UTC()}
	return database.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}, {Name: "doc_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"prefix", "template", "updated_at"}),
	}).Create(&m).Error
}
