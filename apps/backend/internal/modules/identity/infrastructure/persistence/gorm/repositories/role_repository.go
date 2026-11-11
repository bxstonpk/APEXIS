package repositories

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"apexis/backend/internal/modules/identity/application/dto"
	ports "apexis/backend/internal/modules/identity/application/ports/repositories"
	"apexis/backend/internal/modules/identity/domain/entities"
	domain "apexis/backend/internal/modules/identity/domain/errors"
	vo "apexis/backend/internal/modules/identity/domain/value_objects"
	gp "apexis/backend/internal/modules/identity/infrastructure/persistence/gorm"
	"apexis/backend/internal/modules/identity/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/database"
	"apexis/backend/internal/platform/listing"
)

var _ ports.RoleRepository = (*RoleRepository)(nil)

type RoleRepository struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) *RoleRepository { return &RoleRepository{db: db} }

// Create เขียน role + permissions (2 ตาราง) — ต้องอยู่ใน transaction ของ use case
func (r *RoleRepository) Create(ctx context.Context, role *entities.Role) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.RoleToModel(role)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrRoleCodeExists
		}
		return err
	}
	return insertRolePermissions(tx, m.Permissions)
}

// Update แก้ชื่อ/คำอธิบาย และแทนที่ permission ทั้งชุด — ต้องอยู่ใน transaction ของ use case
func (r *RoleRepository) Update(ctx context.Context, role *entities.Role) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.RoleToModel(role)
	res := tx.Model(&m).
		Where("company_id = ?", m.CompanyID).
		Select("name", "description", "updated_at").
		Omit(clause.Associations).
		Updates(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoleNotFound
	}
	if err := tx.Where("role_id = ?", m.ID).Delete(&models.RolePermissionModel{}).Error; err != nil {
		return err
	}
	return insertRolePermissions(tx, m.Permissions)
}

func (r *RoleRepository) Delete(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) error {
	res := database.Conn(ctx, r.db).
		Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).
		Delete(&models.RoleModel{})
	if res.Error != nil {
		if database.IsForeignKeyViolation(res.Error) {
			return domain.ErrRoleInUse
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

// FindByID / FindByIDs โหลด aggregate เต็มตัว (role + permissions) สำหรับฝั่งเขียน
// Preload = 2 query (roles, permissions) — แยกเพราะเป็น one-to-many
func (r *RoleRepository) FindByID(ctx context.Context, companyID vo.CompanyID, id vo.RoleID) (*entities.Role, error) {
	var m models.RoleModel
	err := database.Conn(ctx, r.db).Preload("Permissions").
		Where("company_id = ? AND id = ?", uuid.UUID(companyID), uuid.UUID(id)).
		Take(&m).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}
	return gp.RoleToEntity(m), nil
}

func (r *RoleRepository) FindByIDs(ctx context.Context, companyID vo.CompanyID, ids []vo.RoleID) ([]*entities.Role, error) {
	if len(ids) == 0 {
		return []*entities.Role{}, nil
	}
	var ms []models.RoleModel
	err := database.Conn(ctx, r.db).Preload("Permissions").
		Where("company_id = ? AND id IN ?", uuid.UUID(companyID), gp.ToUUIDs(ids)).
		Order("code").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return gp.RolesToEntities(ms), nil
}

var roleSortColumns = map[string]string{
	"code":      "r.code",
	"name":      "r.name",
	"is_system": "r.is_system",
}

// List — 2 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล):
//  1. roles กรอง/เรียง/แบ่งหน้าใน DB พร้อม COUNT(*) OVER()
//  2. permissions ของ role ในหน้านั้น (one-to-many แยก query เพื่อไม่ให้ pagination ผิด)
func (r *RoleRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.RoleFilter) (listing.Result[dto.Role], error) {
	order, err := database.OrderBy(f.Sort, roleSortColumns, "r.id")
	if err != nil {
		return listing.Result[dto.Role]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).
			Table("identity_roles AS r").
			Where("r.company_id = ?", uuid.UUID(companyID))
		if f.Search != "" {
			p := database.LikeContains(f.Search)
			q = q.Where("(LOWER(r.code) LIKE ? OR LOWER(r.name) LIKE ?)", p, p)
		}
		if f.IsSystem != nil {
			q = q.Where("r.is_system = ?", *f.IsSystem)
		}
		return q
	}
	type row struct {
		ID          uuid.UUID
		Code        string
		Name        string
		Description string
		IsSystem    bool
		Total       int64
	}
	rows, total, err := database.FetchPage(base,
		"r.id, r.code, r.name, r.description, r.is_system",
		order, f.Page, func(r row) int64 { return r.Total })
	if err != nil {
		return listing.Result[dto.Role]{}, err
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	perms, err := r.permissionsOf(ctx, ids)
	if err != nil {
		return listing.Result[dto.Role]{}, err
	}

	items := make([]dto.Role, len(rows))
	for i, row := range rows {
		items[i] = dto.Role{
			ID: row.ID.String(), CompanyID: companyID.String(), Code: row.Code, Name: row.Name,
			Description: row.Description, Permissions: orEmpty(perms[row.ID]), IsSystem: row.IsSystem,
		}
	}
	return listing.Result[dto.Role]{Items: items, Total: total, Page: f.Page}, nil
}

func (r *RoleRepository) permissionsOf(ctx context.Context, roleIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := make(map[uuid.UUID][]string, len(roleIDs))
	if len(roleIDs) == 0 {
		return out, nil
	}
	var rows []models.RolePermissionModel
	err := database.Conn(ctx, r.db).
		Select("role_id", "permission").
		Where("role_id IN ?", roleIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.RoleID] = append(out[p.RoleID], p.Permission)
	}
	for id := range out {
		slices.Sort(out[id])
	}
	return out, nil
}

func insertRolePermissions(tx *gorm.DB, perms []models.RolePermissionModel) error {
	if len(perms) == 0 {
		return nil
	}
	return tx.Create(&perms).Error
}
