package dto

import (
	vo "apexis/backend/internal/modules/purchasing/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
)

// Dashboard คือภาพรวมของแอปจัดซื้อ (GET /dashboards/purchasing) — ยอดซื้อ = ยอดก่อน VAT ของใบแจ้งหนี้ผู้ขายที่ลงบัญชีแล้ว
// (วันที่ = posting_date เหมือนรายงานของจัดซื้อ; ไม่นับที่ยกเลิก)
type Dashboard struct {
	Period            dashperiod.Period
	PurchasesCurrent  vo.Money
	PurchasesPrevious vo.Money // ช่วงก่อนหน้าที่ยาวเท่ากัน
	InvoiceCount      int64
	Monthly           []MonthNet // 12 เดือน เก่า -> ใหม่

	OrderDraftCount     int64
	OrderToReceiveCount int64 // อนุมัติแล้ว มีบรรทัดที่รับยังไม่ครบ
	OrderToInvoiceCount int64 // อนุมัติ / ปิดแล้ว มีบรรทัดที่รับแล้วแต่ยังไม่ได้ใบแจ้งหนี้ (กฎเดียวกับรายงานค้างรับ / ค้างแจ้งหนี้)

	Payables     PayableSummary
	TopSuppliers []SupplierAmount // 5 อันดับตามยอดซื้อในช่วง
}

type MonthNet struct {
	Month string // YYYY-MM
	Net   vo.Money
}

// PayableSummary: เจ้าหนี้ค้าง ณ end_date (หักยอดจ่ายที่ลงวันที่ไม่เกิน end_date) แยกตามอายุนับจากวันครบกำหนด
type PayableSummary struct {
	Outstanding vo.Money
	Overdue     vo.Money
	Due7Days    vo.Money // ยังไม่เลยกำหนด และครบกำหนดภายใน 7 วันนับจาก end_date
	NotDue      vo.Money
	Days1To30   vo.Money
	Days31To60  vo.Money
	Days61To90  vo.Money
	Over90      vo.Money
}

type SupplierAmount struct {
	Supplier Ref
	Net      vo.Money
}
