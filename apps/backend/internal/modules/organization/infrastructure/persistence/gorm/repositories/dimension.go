package repositories

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/organization/application/dto"
	ports "apexis/backend/internal/modules/organization/application/ports/repositories"
	vo "apexis/backend/internal/modules/organization/domain/value_objects"
	"apexis/backend/internal/platform/database"
)

var _ ports.DimensionRepository = (*DimensionRepository)(nil)

type DimensionRepository struct{ db *gorm.DB }

func NewDimensionRepository(db *gorm.DB) *DimensionRepository { return &DimensionRepository{db: db} }

// Missing — 1 query: นับ id ที่พบของแต่ละมิติด้วย sub-select แล้วเทียบกับจำนวน id ที่ขอ (ไม่ซ้ำ)
func (r *DimensionRepository) Missing(ctx context.Context, companyID vo.CompanyID, d dto.Dimensions) ([]string, error) {
	kinds := []struct {
		name, table string
		ids         []uuid.UUID
	}{
		{"branch", "organization_branches", uniqueUUIDs(d.Branches)},
		{"department", "organization_departments", uniqueUUIDs(d.Departments)},
		{"cost_center", "organization_cost_centers", uniqueUUIDs(d.CostCenters)},
		{"project_site", "organization_project_sites", uniqueUUIDs(d.ProjectSites)},
	}
	sql := "SELECT "
	var args []any
	asked := false
	for i, k := range kinds {
		if i > 0 {
			sql += ", "
		}
		if len(k.ids) == 0 {
			sql += "0 AS " + k.name
			continue
		}
		asked = true
		sql += "(SELECT COUNT(*) FROM " + k.table + " WHERE company_id = ? AND id IN ? AND deleted_at IS NULL) AS " + k.name
		args = append(args, uuidOf(companyID), k.ids)
	}
	if !asked {
		return nil, nil
	}
	var found struct{ Branch, Department, CostCenter, ProjectSite int }
	if err := database.Conn(ctx, r.db).Raw(sql, args...).Scan(&found).Error; err != nil {
		return nil, err
	}
	counts := []int{found.Branch, found.Department, found.CostCenter, found.ProjectSite}
	var missing []string
	for i, k := range kinds {
		if counts[i] != len(k.ids) {
			missing = append(missing, k.name)
		}
	}
	return missing, nil
}

func uniqueUUIDs[T ~[16]byte](in []T) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if u := uuid.UUID(id); !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
}
