// Package services คือ domain service ของ organization — กฎที่ไม่ได้อยู่ใน entity เดียว
package services

import domain "apexis/backend/internal/modules/organization/domain/errors"

// EnsureNoCycle ตรวจว่าการตั้ง parent ใหม่จะไม่ทำให้ลำดับชั้นวนเป็นวง
//
// ancestry คือ id ของ parent ใหม่และบรรพบุรุษทั้งหมดของมัน (repository โหลดด้วย recursive query เดียว)
// ถ้า self อยู่ใน ancestry แปลว่า parent ใหม่คือตัวเองหรือลูกหลานของตัวเอง
func EnsureNoCycle[T comparable](self T, ancestry []T) error {
	for _, id := range ancestry {
		if id == self {
			return domain.ErrHierarchyCycle
		}
	}
	return nil
}
