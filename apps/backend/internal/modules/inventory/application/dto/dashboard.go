package dto

import (
	vo "apexis/backend/internal/modules/inventory/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
)

// Dashboard คือภาพรวมของแอปคลังสินค้า (GET /dashboards/inventory)
//
//   - มูลค่าคงเหลือ = ผลรวมการเคลื่อนไหว (มูลค่ามีเครื่องหมาย) ถึง end_date — สูตรเดียวกับรายงานสินค้าคงเหลือ (= ชั้น FIFO ที่เหลือ)
//   - รับ / จ่าย = จำนวนเอกสารคลังที่ผ่านรายการแล้ว (ไม่นับร่าง / ยกเลิก) ชนิด receipt / issue ในช่วง
//     (ใบรับสินค้าของจัดซื้อและใบส่งของของขายสร้างเอกสารคลังชนิดเหล่านี้; โอนย้ายและปรับปรุงไม่นับ)
//   - มูลค่าเข้า / ออกรายเดือน = การเคลื่อนไหวของเอกสารที่ผ่านรายการแล้ว ไม่รวมโอนย้ายระหว่างคลัง (เข้า-ออกหักล้างกัน)
type Dashboard struct {
	Period            dashperiod.Period
	StockValue        vo.Money
	StockItemCount    int64 // สินค้าชนิดนับสต็อกที่ใช้งาน
	Warehouses        []WarehouseValue
	BelowReorderCount int64
	BelowReorder      []BelowReorder // ไม่เกิน 10 รายการ เรียงตามจำนวนที่ขาดมากไปน้อย
	ReceiptCount      int64
	IssueCount        int64
	Monthly           []MonthFlow // 12 เดือน เก่า -> ใหม่
}

type WarehouseValue struct {
	Warehouse Ref
	Value     vo.Money
}

type BelowReorder struct {
	Item         Ref
	OnHand       vo.Quantity
	ReorderPoint vo.Quantity
}

type MonthFlow struct {
	Month    string // YYYY-MM
	InValue  vo.Money
	OutValue vo.Money
}
