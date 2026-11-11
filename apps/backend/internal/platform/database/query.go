package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"apexis/backend/internal/platform/listing"
)

// OrderBy แปลง sort key สาธารณะ -> column ผ่าน whitelist เท่านั้น (ห้ามนำ input ของ client ไปต่อ SQL ตรงๆ)
// และต่อท้ายด้วย tieBreaker (column unique) เพื่อให้ลำดับคงที่ข้ามหน้า
func OrderBy(sorts []listing.Sort, columns map[string]string, tieBreaker string) (string, error) {
	parts := make([]string, 0, len(sorts)+1)
	for _, s := range sorts {
		col, ok := columns[s.Key]
		if !ok {
			return "", fmt.Errorf("database: unsupported sort key %q", s.Key)
		}
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		parts = append(parts, col+" "+dir)
	}
	return strings.Join(append(parts, tieBreaker+" ASC"), ", "), nil
}

// FetchPage รัน query หนึ่งหน้า: base() ต้องคืน query (FROM / JOIN / WHERE) ใหม่ทุกครั้งที่เรียก
//
// ใช้ COUNT(*) OVER() เพื่อได้ทั้งข้อมูลหน้าและจำนวนทั้งหมดใน query เดียว
// row ต้องมี field Total (column "total") — คืนค่า total ผ่าน totalOf
// ถ้าหน้าที่ขอเกินจำนวนข้อมูล (ไม่มีแถวกลับมา) จะ COUNT แยกอีกครั้ง เพื่อให้ meta.total ถูกต้อง
func FetchPage[R any](
	base func() *gorm.DB,
	columns string,
	order string,
	page listing.Page,
	totalOf func(R) int64,
) ([]R, int64, error) {
	var rows []R
	err := base().
		Select(columns + ", COUNT(*) OVER() AS total").
		Order(order).
		Limit(page.Limit()).
		Offset(page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	if len(rows) > 0 {
		return rows, totalOf(rows[0]), nil
	}
	if page.Offset() == 0 {
		return rows, 0, nil
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
