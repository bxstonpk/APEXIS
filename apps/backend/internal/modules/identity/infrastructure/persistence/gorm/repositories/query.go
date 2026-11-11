package repositories

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/platform/database"
)

// membershipRoleIDs โหลด role id ของหลาย membership ใน query เดียว (เฉพาะ membership ในหน้าปัจจุบัน)
// แยกจาก query หลักเพราะเป็น one-to-many: ถ้า JOIN ตรงๆ แถวจะซ้ำและ LIMIT/OFFSET จะผิด
func membershipRoleIDs(ctx context.Context, db *gorm.DB, membershipIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := make(map[uuid.UUID][]string, len(membershipIDs))
	if len(membershipIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		MembershipID uuid.UUID
		RoleID       uuid.UUID
	}
	err := database.Conn(ctx, db).
		Table("identity_membership_roles").
		Select("membership_id, role_id").
		Where("membership_id IN ?", membershipIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.MembershipID] = append(out[r.MembershipID], r.RoleID.String())
	}
	for id := range out {
		slices.Sort(out[id])
	}
	return out, nil
}

// orEmpty คืน slice ว่างแทน nil เพื่อให้ JSON เป็น [] ไม่ใช่ null
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
