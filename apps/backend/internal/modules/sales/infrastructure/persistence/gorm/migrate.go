package gormpersistence

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/sales/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/integration"
)

// Migrate สร้าง / ปรับตารางของ sales แล้วบังคับ "เลขผู้เสียภาษี 1 เลข = ลูกค้า 1 รหัส"
//
//  1. ใบแจ้งหนี้เดิมที่ยังไม่มีผู้ซื้อ ณ วันออก (bill_to_*) — เติมจากทะเบียนลูกค้า ณ ตอนนี้
//  2. ลูกค้าเลขผู้เสียภาษีซ้ำ (ตัดสินใจ 2026-11-23: รวมเป็นรหัสเดียว) — เก็บรหัสที่สร้างก่อนสุด,
//     สาขาหลักของรหัสที่ซ้ำกลายเป็นสาขาเพิ่มเติม (ถ้าเลขสาขายังไม่มี), ย้ายเอกสารมาที่รหัสที่เก็บ, ลบ (soft) รหัสที่ซ้ำ
//  3. สร้าง unique index (company_id, tax_id) เมื่อ tax_id ไม่ว่าง
//
// คืนคู่ที่ถูกรวม เพื่อให้ module ที่เก็บ id ลูกค้า (finance) ปรับตาม — ทั้งหมดอยู่ใน transaction เดียว
func Migrate(db *gorm.DB) ([]integration.PartyMerge, error) {
	if err := models.AutoMigrate(db); err != nil {
		return nil, err
	}
	var merges []integration.PartyMerge
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE sales_invoices AS i SET bill_to_name = c.name, bill_to_tax_id = c.tax_id,
				bill_to_branch_no = c.branch_no, bill_to_address = c.address
			FROM sales_customers AS c WHERE c.id = i.customer_id AND i.bill_to_name = ''`).Error; err != nil {
			return err
		}
		var err error
		if merges, err = mergeDuplicateCustomers(tx); err != nil {
			return err
		}
		return tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_sal_customers_tax_id ON sales_customers (company_id, tax_id)
			WHERE tax_id <> '' AND deleted_at IS NULL`).Error
	})
	return merges, err
}

type dupCustomer struct {
	ID       uuid.UUID
	Keep     uuid.UUID
	BranchNo string
}

func mergeDuplicateCustomers(tx *gorm.DB) ([]integration.PartyMerge, error) {
	var dups []dupCustomer
	// keep = รหัสแรกตามวันที่สร้างของแต่ละ (บริษัท, เลขผู้เสียภาษี)
	err := tx.Raw(`SELECT id, keep, branch_no FROM (
			SELECT id, branch_no, FIRST_VALUE(id) OVER (PARTITION BY company_id, tax_id ORDER BY created_at, id) AS keep
			FROM sales_customers WHERE tax_id <> '' AND deleted_at IS NULL) AS x
		WHERE id <> keep`).Scan(&dups).Error
	if err != nil || len(dups) == 0 {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]integration.PartyMerge, 0, len(dups))
	for _, d := range dups {
		steps := []struct {
			sql  string
			args []any
		}{
			// สาขาหลักของรหัสที่ซ้ำ -> สาขาเพิ่มเติมของรหัสที่เก็บ (เลขสาขาซ้ำกับที่มีอยู่ = ใช้ของเดิม)
			{`INSERT INTO sales_customer_branches (customer_id, branch_no, name, address, addr_building, addr_room_no, addr_floor_no,
					addr_village, addr_house_no, addr_moo, addr_soi, addr_yaek, addr_street, addr_sub_district, addr_district,
					addr_province, addr_postal_code, position)
				SELECT ?, c.branch_no, '', c.address, c.addr_building, c.addr_room_no, c.addr_floor_no, c.addr_village, c.addr_house_no,
					c.addr_moo, c.addr_soi, c.addr_yaek, c.addr_street, c.addr_sub_district, c.addr_district, c.addr_province,
					c.addr_postal_code, COALESCE((SELECT MAX(position) FROM sales_customer_branches WHERE customer_id = ?), 0) + 1
				FROM sales_customers AS c
				WHERE c.id = ? AND c.branch_no <> '' AND c.branch_no <> (SELECT branch_no FROM sales_customers WHERE id = ?)
				ON CONFLICT (customer_id, branch_no) DO NOTHING`, []any{d.Keep, d.Keep, d.ID, d.Keep}},
			// สาขาเพิ่มเติมของรหัสที่ซ้ำ
			{`UPDATE sales_customer_branches AS b SET customer_id = ? WHERE b.customer_id = ?
				AND NOT EXISTS (SELECT 1 FROM sales_customer_branches AS k WHERE k.customer_id = ? AND k.branch_no = b.branch_no)`,
				[]any{d.Keep, d.ID, d.Keep}},
			{`DELETE FROM sales_customer_branches WHERE customer_id = ?`, []any{d.ID}},
			// เอกสาร: สาขา "หลัก" ของรหัสที่ซ้ำ = เลขสาขาของรหัสนั้น
			{`UPDATE sales_quotations SET customer_id = ?, customer_branch_no = CASE WHEN customer_branch_no = '' THEN ? ELSE customer_branch_no END
				WHERE customer_id = ?`, []any{d.Keep, d.BranchNo, d.ID}},
			{`UPDATE sales_orders SET customer_id = ?, customer_branch_no = CASE WHEN customer_branch_no = '' THEN ? ELSE customer_branch_no END
				WHERE customer_id = ?`, []any{d.Keep, d.BranchNo, d.ID}},
			{`UPDATE sales_deliveries SET customer_id = ? WHERE customer_id = ?`, []any{d.Keep, d.ID}},
			{`UPDATE sales_invoices SET customer_id = ? WHERE customer_id = ?`, []any{d.Keep, d.ID}},
			{`UPDATE sales_credit_notes SET customer_id = ? WHERE customer_id = ?`, []any{d.Keep, d.ID}},
			{`UPDATE sales_customers SET deleted_at = ? WHERE id = ?`, []any{now, d.ID}},
		}
		for _, s := range steps {
			if err := tx.Exec(s.sql, s.args...).Error; err != nil {
				return nil, err
			}
		}
		out = append(out, integration.PartyMerge{From: d.ID, To: d.Keep})
	}
	return out, nil
}
