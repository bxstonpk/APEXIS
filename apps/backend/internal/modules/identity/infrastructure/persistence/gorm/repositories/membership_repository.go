package repositories

import (
	"context"
	"time"

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

var _ ports.MembershipRepository = (*MembershipRepository)(nil)

type MembershipRepository struct{ db *gorm.DB }

func NewMembershipRepository(db *gorm.DB) *MembershipRepository {
	return &MembershipRepository{db: db}
}

// Create เขียน membership + roles (2 ตาราง) — ต้องอยู่ใน transaction ของ use case
func (r *MembershipRepository) Create(ctx context.Context, ms *entities.Membership) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.MembershipToModel(ms)
	if err := tx.Omit(clause.Associations).Create(&m).Error; err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrMembershipExists
		}
		return err
	}
	return insertMembershipRoles(tx, m.Roles)
}

// Update แก้สถานะ และแทนที่ role ทั้งชุด — ต้องอยู่ใน transaction ของ use case
func (r *MembershipRepository) Update(ctx context.Context, ms *entities.Membership) error {
	tx, err := database.TxConn(ctx)
	if err != nil {
		return err
	}
	m := gp.MembershipToModel(ms)
	res := tx.Model(&m).
		Where("company_id = ?", m.CompanyID).
		Select("status", "updated_at").
		Omit(clause.Associations).
		Updates(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrMembershipNotFound
	}
	if err := tx.Where("membership_id = ?", m.ID).Delete(&models.MembershipRoleModel{}).Error; err != nil {
		return err
	}
	return insertMembershipRoles(tx, m.Roles)
}

// Find โหลด aggregate เต็มตัว (membership + role ids) สำหรับฝั่งเขียน
func (r *MembershipRepository) Find(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) (*entities.Membership, error) {
	var m models.MembershipModel
	err := database.Conn(ctx, r.db).Preload("Roles").
		Select("id", "company_id", "user_id", "status").
		Where("company_id = ? AND user_id = ?", uuid.UUID(companyID), uuid.UUID(userID)).
		Take(&m).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, domain.ErrMembershipNotFound
		}
		return nil, err
	}
	return gp.MembershipToEntity(m), nil
}

var memberSortColumns = map[string]string{
	"name":      "u.name",
	"email":     "u.email",
	"status":    "m.status",
	"joined_at": "m.created_at",
}

// CompanyIDsOf — 1 query ใช้ index uq_identity_memberships_company_user / idx_identity_memberships_user
func (r *MembershipRepository) CompanyIDsOf(ctx context.Context, userID vo.UserID) ([]vo.CompanyID, error) {
	var ids []uuid.UUID
	err := database.Conn(ctx, r.db).Model(&models.MembershipModel{}).
		Where("user_id = ? AND status = ?", uuid.UUID(userID), vo.MembershipStatusActive).
		Pluck("company_id", &ids).Error
	if err != nil {
		return nil, err
	}
	out := make([]vo.CompanyID, len(ids))
	for i, id := range ids {
		out[i] = vo.CompanyID(id)
	}
	return out, nil
}

// List — 2 query (+1 COUNT เฉพาะกรณีขอหน้าที่เกินข้อมูล):
//  1. memberships JOIN users กรอง/เรียง/แบ่งหน้าใน DB พร้อม COUNT(*) OVER()
//  2. role id ของ membership ในหน้านั้น (one-to-many แยก query เพื่อไม่ให้ pagination ผิด)
func (r *MembershipRepository) List(ctx context.Context, companyID vo.CompanyID, f dto.MemberFilter) (listing.Result[dto.Member], error) {
	order, err := database.OrderBy(f.Sort, memberSortColumns, "m.id")
	if err != nil {
		return listing.Result[dto.Member]{}, err
	}
	base := func() *gorm.DB {
		q := database.Conn(ctx, r.db).
			Table("identity_memberships AS m").
			Joins("JOIN identity_users AS u ON u.id = m.user_id").
			Where("m.company_id = ?", uuid.UUID(companyID))
		if f.Search != "" {
			p := database.LikeContains(f.Search)
			q = q.Where("(LOWER(u.name) LIKE ? OR LOWER(u.email) LIKE ?)", p, p)
		}
		if len(f.Statuses) > 0 {
			q = q.Where("m.status IN ?", f.Statuses)
		}
		if f.RoleID != nil {
			q = q.Where("EXISTS (SELECT 1 FROM identity_membership_roles AS mr WHERE mr.membership_id = m.id AND mr.role_id = ?)",
				uuid.UUID(*f.RoleID))
		}
		return q
	}
	type row struct {
		MembershipID uuid.UUID
		UserID       uuid.UUID
		Email        string
		Name         string
		UserStatus   string
		MustChange   bool
		Status       string
		JoinedAt     time.Time
		Total        int64
	}
	rows, total, err := database.FetchPage(base,
		memberColumns,
		order, f.Page, func(r row) int64 { return r.Total })
	if err != nil {
		return listing.Result[dto.Member]{}, err
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.MembershipID
	}
	roles, err := membershipRoleIDs(ctx, r.db, ids)
	if err != nil {
		return listing.Result[dto.Member]{}, err
	}

	items := make([]dto.Member, len(rows))
	for i, row := range rows {
		items[i] = dto.Member{
			User:     dto.User{ID: row.UserID.String(), Email: row.Email, Name: row.Name, Status: row.UserStatus, MustChangePassword: row.MustChange},
			Status:   row.Status,
			RoleIDs:  orEmpty(roles[row.MembershipID]),
			JoinedAt: row.JoinedAt.UTC(),
		}
	}
	return listing.Result[dto.Member]{Items: items, Total: total, Page: f.Page}, nil
}

const memberColumns = "m.id AS membership_id, u.id AS user_id, u.email, u.name, u.status AS user_status, " +
	"u.must_change_password AS must_change, m.status, m.created_at AS joined_at"

// Get: 2 query — สมาชิก JOIN ผู้ใช้ (1 แถว) + role ids ของสมาชิก (ใช้ helper เดียวกับ List)
func (r *MembershipRepository) Get(ctx context.Context, companyID vo.CompanyID, userID vo.UserID) (dto.Member, error) {
	var row struct {
		MembershipID uuid.UUID
		UserID       uuid.UUID
		Email        string
		Name         string
		UserStatus   string
		MustChange   bool
		Status       string
		JoinedAt     time.Time
	}
	res := database.Conn(ctx, r.db).
		Table("identity_memberships AS m").
		Joins("JOIN identity_users AS u ON u.id = m.user_id").
		Select(memberColumns).
		Where("m.company_id = ? AND m.user_id = ?", uuid.UUID(companyID), uuid.UUID(userID)).
		Limit(1).
		Scan(&row)
	if res.Error != nil {
		return dto.Member{}, res.Error
	}
	if res.RowsAffected == 0 {
		return dto.Member{}, domain.ErrMembershipNotFound
	}
	roles, err := membershipRoleIDs(ctx, r.db, []uuid.UUID{row.MembershipID})
	if err != nil {
		return dto.Member{}, err
	}
	return dto.Member{
		User:     dto.User{ID: row.UserID.String(), Email: row.Email, Name: row.Name, Status: row.UserStatus, MustChangePassword: row.MustChange},
		Status:   row.Status,
		RoleIDs:  orEmpty(roles[row.MembershipID]),
		JoinedAt: row.JoinedAt.UTC(),
	}, nil
}

func insertMembershipRoles(tx *gorm.DB, roles []models.MembershipRoleModel) error {
	if len(roles) == 0 {
		return nil
	}
	return tx.Create(&roles).Error
}
