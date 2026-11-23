package gormpersistence

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"apexis/backend/internal/modules/purchasing/infrastructure/persistence/gorm/models"
	"apexis/backend/internal/platform/integration"
)

// Migrate สร้าง / ปรับตารางของ purchasing แล้วบังคับ "เลขผู้เสียภาษี 1 เลข = ผู้ขาย 1 รหัส"
//
//  1. ใบแจ้งหนี้เดิมที่ยังไม่มีผู้ขาย ณ วันบันทึก (bill_from_*) — เติมจากทะเบียนผู้ขาย ณ ตอนนี้
//  2. ผู้ขายเลขผู้เสียภาษีซ้ำ (ตัดสินใจ 2026-11-23: รวมเป็นรหัสเดียว) — เก็บรหัสที่สร้างก่อนสุด,
//     สาขาหลักของรหัสที่ซ้ำกลายเป็นสาขาเพิ่มเติม (ถ้าเลขสาขายังไม่มี), ย้ายเอกสารมาที่รหัสที่เก็บ, ลบ (soft) รหัสที่ซ้ำ
//     ใบแจ้งหนี้ที่ย้ายแล้วจะชนเลขที่ใบแจ้งหนี้ของผู้ขาย (uq_pur_invoices_supplier_no) คงไว้ที่รหัสเดิม + log เตือน
//  3. สร้าง unique index (company_id, tax_id) เมื่อ tax_id ไม่ว่าง
//
// คืนคู่ที่ถูกรวม เพื่อให้ finance ปรับ id คู่ค้าในใบจ่ายเงิน — ทั้งหมดอยู่ใน transaction เดียว
func Migrate(db *gorm.DB) ([]integration.PartyMerge, error) {
	if err := models.AutoMigrate(db); err != nil {
		return nil, err
	}
	var merges []integration.PartyMerge
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE purchasing_invoices AS i SET bill_from_name = s.name, bill_from_tax_id = s.tax_id,
				bill_from_branch_no = s.branch_no, bill_from_address = s.address
			FROM purchasing_suppliers AS s WHERE s.id = i.supplier_id AND i.bill_from_name = ''`).Error; err != nil {
			return err
		}
		var err error
		if merges, err = mergeDuplicateSuppliers(tx); err != nil {
			return err
		}
		return tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_pur_suppliers_tax_id ON purchasing_suppliers (company_id, tax_id)
			WHERE tax_id <> '' AND deleted_at IS NULL`).Error
	})
	return merges, err
}

type dupSupplier struct {
	ID       uuid.UUID
	Keep     uuid.UUID
	BranchNo string
}

func mergeDuplicateSuppliers(tx *gorm.DB) ([]integration.PartyMerge, error) {
	var dups []dupSupplier
	// keep = รหัสแรกตามวันที่สร้างของแต่ละ (บริษัท, เลขผู้เสียภาษี)
	err := tx.Raw(`SELECT id, keep, branch_no FROM (
			SELECT id, branch_no, FIRST_VALUE(id) OVER (PARTITION BY company_id, tax_id ORDER BY created_at, id) AS keep
			FROM purchasing_suppliers WHERE tax_id <> '' AND deleted_at IS NULL) AS x
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
			{`INSERT INTO purchasing_supplier_branches (supplier_id, branch_no, name, address, addr_building, addr_room_no, addr_floor_no,
					addr_village, addr_house_no, addr_moo, addr_soi, addr_yaek, addr_street, addr_sub_district, addr_district,
					addr_province, addr_postal_code, position)
				SELECT ?, s.branch_no, '', s.address, s.addr_building, s.addr_room_no, s.addr_floor_no, s.addr_village, s.addr_house_no,
					s.addr_moo, s.addr_soi, s.addr_yaek, s.addr_street, s.addr_sub_district, s.addr_district, s.addr_province,
					s.addr_postal_code, COALESCE((SELECT MAX(position) FROM purchasing_supplier_branches WHERE supplier_id = ?), 0) + 1
				FROM purchasing_suppliers AS s
				WHERE s.id = ? AND s.branch_no <> '' AND s.branch_no <> (SELECT branch_no FROM purchasing_suppliers WHERE id = ?)
				ON CONFLICT (supplier_id, branch_no) DO NOTHING`, []any{d.Keep, d.Keep, d.ID, d.Keep}},
			// สาขาเพิ่มเติมของรหัสที่ซ้ำ
			{`UPDATE purchasing_supplier_branches AS b SET supplier_id = ? WHERE b.supplier_id = ?
				AND NOT EXISTS (SELECT 1 FROM purchasing_supplier_branches AS k WHERE k.supplier_id = ? AND k.branch_no = b.branch_no)`,
				[]any{d.Keep, d.ID, d.Keep}},
			{`DELETE FROM purchasing_supplier_branches WHERE supplier_id = ?`, []any{d.ID}},
			// ใบสั่งซื้อ: สาขา "หลัก" ของรหัสที่ซ้ำ = เลขสาขาของรหัสนั้น
			{`UPDATE purchasing_orders SET supplier_id = ?, supplier_branch_no = CASE WHEN supplier_branch_no = '' THEN ? ELSE supplier_branch_no END
				WHERE supplier_id = ?`, []any{d.Keep, d.BranchNo, d.ID}},
			{`UPDATE purchasing_receipts SET supplier_id = ? WHERE supplier_id = ?`, []any{d.Keep, d.ID}},
			// ใบแจ้งหนี้: ย้ายเฉพาะใบที่ไม่ชนเลขที่ใบแจ้งหนี้ของผู้ขาย (posted) ของรหัสที่เก็บ
			{`UPDATE purchasing_invoices AS i SET supplier_id = ? WHERE i.supplier_id = ? AND NOT (i.status = 'posted' AND EXISTS (
					SELECT 1 FROM purchasing_invoices AS k WHERE k.company_id = i.company_id AND k.supplier_id = ?
						AND k.status = 'posted' AND k.supplier_invoice_no = i.supplier_invoice_no))`, []any{d.Keep, d.ID, d.Keep}},
			{`UPDATE purchasing_suppliers SET deleted_at = ? WHERE id = ?`, []any{now, d.ID}},
		}
		for _, s := range steps {
			if err := tx.Exec(s.sql, s.args...).Error; err != nil {
				return nil, err
			}
		}
		var left int64
		if err := tx.Raw(`SELECT COUNT(*) FROM purchasing_invoices WHERE supplier_id = ?`, d.ID).Scan(&left).Error; err != nil {
			return nil, err
		}
		if left > 0 {
			slog.Warn("supplier merge: invoices kept on merged supplier (supplier invoice number already used by the kept supplier)",
				"merged_supplier_id", d.ID, "kept_supplier_id", d.Keep, "invoices", left)
		}
		out = append(out, integration.PartyMerge{From: d.ID, To: d.Keep})
	}
	return out, nil
}
