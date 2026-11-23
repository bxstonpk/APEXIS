package dto

import (
	vo "apexis/backend/internal/modules/sales/domain/value_objects"
	"apexis/backend/internal/platform/dashperiod"
)

// Dashboard คือภาพรวมของแอปขาย (GET /dashboards/sales) — ยอดเป็นยอดก่อน VAT ของใบแจ้งหนี้ที่ลงบัญชีแล้ว (ไม่นับที่ยกเลิก)
type Dashboard struct {
	Period        dashperiod.Period
	SalesCurrent  vo.Money
	SalesPrevious vo.Money // ช่วงก่อนหน้าที่ยาวเท่ากัน
	InvoiceCount  int64
	Monthly       []MonthNet // 12 เดือนของปีบัญชี (หรือ 12 เดือนล่าสุด) เก่า -> ใหม่ (เดือนที่ไม่มีข้อมูล = 0)
	PreviousYear  []MonthNet // 12 เดือนของปีบัญชีก่อนหน้า (ช่องตรงกับ Monthly) — nil = ไม่มีปีบัญชี หรือปีก่อนไม่มียอด

	QuotationOpenCount  int64    // ส่งแล้ว รอลูกค้าตอบ
	QuotationOpenAmount vo.Money // ยอดรวม VAT ของใบเหล่านั้น
	OrderDraftCount     int64
	OrderToDeliverCount int64 // อนุมัติแล้ว มีบรรทัดที่ส่งยังไม่ครบ
	OrderToInvoiceCount int64 // อนุมัติแล้ว มีบรรทัดที่ออกบิลยังไม่ครบ

	Receivables  ReceivableSummary
	TopCustomers []CustomerAmount // 5 อันดับตามยอดขายในช่วง
}

type MonthNet struct {
	Month string // YYYY-MM
	Net   vo.Money
}

// ReceivableSummary: ลูกหนี้ค้าง ณ end_date (หักยอดรับที่ลงวันที่ไม่เกิน end_date) แยกตามอายุนับจากวันครบกำหนด
type ReceivableSummary struct {
	Outstanding vo.Money
	Overdue     vo.Money
	NotDue      vo.Money
	Days1To30   vo.Money
	Days31To60  vo.Money
	Days61To90  vo.Money
	Over90      vo.Money
}

type CustomerAmount struct {
	Customer Ref
	Net      vo.Money
}
