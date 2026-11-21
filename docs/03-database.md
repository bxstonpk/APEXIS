# 3. Database

- PostgreSQL (ทดสอบกับ 16) — ทุกเวลาเก็บเป็น UTC, primary key เป็น UUID ที่สร้างฝั่งแอป
- ชื่อตารางขึ้นต้นด้วยชื่อ module (`identity_*`, `organization_*`)
- schema สร้างด้วย GORM `AutoMigrate` ตอนเริ่มโปรแกรม (ดูข้อควรระวังใน [06-deployment.md](06-deployment.md))

## 3.1 ER Diagram

```text
identity (เก็บ company_id แต่ไม่มี FK ข้าม module ไป organization_companies)
────────
identity_users ─┐
                │1     n┌──────────────────────┐
                └──────►│ identity_memberships │  company_id
                        └──────────┬───────────┘
                                   │1
                                   │n
                     ┌─────────────▼─────────────┐  n   1┌─────────────────┐
                     │ identity_membership_roles │──────►│ identity_roles  │  company_id
                     └───────────────────────────┘       └────────┬────────┘
                                                                  │1..n
                                                     identity_role_permissions

organization (─< = หนึ่งต่อหลาย ผ่าน company_id + FK)
────────────
organization_companies                      ← tenant ของทั้งระบบ
 ├─< organization_branches
 ├─< organization_business_units            parent_id → business_units (self)
 ├─< organization_departments               parent_id → departments (self), business_unit_id → business_units, branch_id → branches
 ├─< organization_cost_centers              department_id → departments
 ├─< organization_project_sites             branch_id → branches, cost_center_id → cost_centers
 └─< organization_fiscal_years
        └─< organization_fiscal_periods
```

## 3.2 ตาราง — identity

| ตาราง | คอลัมน์หลัก | Constraint |
|---|---|---|
| `identity_users` | id, email, name, password_hash, status | UNIQUE `email` |
| `identity_roles` | id, company_id, code, name, description, is_system | UNIQUE `(company_id, code)` |
| `identity_role_permissions` | role_id, permission | PK `(role_id, permission)`, FK role CASCADE |
| `identity_memberships` | id, company_id, user_id, status | UNIQUE `(company_id, user_id)`, index `user_id`, FK user RESTRICT |
| `identity_membership_roles` | membership_id, role_id | PK คู่, FK membership CASCADE, FK role **RESTRICT** (ลบ role ที่มีคนใช้ไม่ได้) |

`company_id` ใน identity ไม่มี FK — บริษัทกับ owner ถูกสร้างใน transaction เดียวกันเสมอ จึงไม่มี membership ที่ไม่มีบริษัท

## 3.3 ตาราง — organization

| ตาราง | คอลัมน์หลัก | Constraint / Index |
|---|---|---|
| `organization_companies` | id, code, name, tax_id, legal_name_th/en, address_*, phone, email, website, base_currency, fiscal_year_start_month, amount_decimals, quantity_decimals, status | UNIQUE `code` (ทั้งระบบ); CHECK month 1–12, CHECK decimals 0–6, CHECK status; **ไม่มี soft delete** (ปิดด้วย status) |
| `organization_branches` | id, company_id, code, name, tax_branch_no, address_*, phone, status, deleted_at | partial UNIQUE `(company_id, code)` และ `(company_id, tax_branch_no)` `WHERE deleted_at IS NULL`; CHECK status |
| `organization_business_units` | id, company_id, code, name, parent_id, status, deleted_at | partial UNIQUE code; FK parent (self) RESTRICT; CHECK status |
| `organization_departments` | id, company_id, code, name, parent_id, business_unit_id, branch_id, status, deleted_at | partial UNIQUE code; FK parent / BU / branch RESTRICT; CHECK status |
| `organization_cost_centers` | id, company_id, code, name, department_id, valid_from, valid_to, status, deleted_at | partial UNIQUE code; FK department; CHECK `valid_to IS NULL OR valid_to >= valid_from` |
| `organization_project_sites` | id, company_id, code, name, branch_id, cost_center_id, address_*, start_date, end_date, status, deleted_at | partial UNIQUE code; FK branch / cost center; CHECK วันที่, CHECK status `planned/active/closed` |
| `organization_fiscal_years` | id, company_id, year, start_date, end_date | UNIQUE `(company_id, year)`; CHECK `end_date > start_date`; **ไม่มี soft delete** |
| `organization_fiscal_periods` | id, fiscal_year_id, period_no, start_date, end_date, status | UNIQUE `(fiscal_year_id, period_no)`; CHECK period_no 1–12, วันที่, status `open/closed`; index `(start_date, end_date)` |

- ทุกตาราง (ยกเว้น `organization_companies`, `organization_fiscal_periods`) มี FK `company_id → organization_companies` (RESTRICT)
- `address_*` = `address_line, address_sub_district, address_district, address_province, address_postal_code, address_country` (ค่าเริ่มต้น `TH`)

## 3.3.1 ตาราง — ledger

```text
ledger_accounts ◄─ parent_id (self)
   ▲ account_id
ledger_journal_lines ─► ledger_journal_entries ─► ledger_journals ◄─ ledger_journal_sequences
   (entry_id, line_no)     reversal_of_id / reversed_by_id → ledger_journal_entries (self)
```

| ตาราง | คอลัมน์หลัก | Constraint / Index |
|---|---|---|
| `ledger_accounts` | id, company_id, code, name, name_en, type, parent_id, postable, status, deleted_at | partial UNIQUE `(company_id, code)`; FK parent (self) RESTRICT; CHECK type 5 หมวด, CHECK status |
| `ledger_journals` | id, company_id, code, name, status, deleted_at | partial UNIQUE `(company_id, code)`; CHECK status |
| `ledger_journal_entries` | id, company_id, journal_id, entry_no, entry_date, fiscal_period_id, description, reference, status, total_debit, total_credit, reversal_of_id, reversed_by_id, posted_at, posted_by, created_by | UNIQUE `(company_id, entry_no) WHERE entry_no IS NOT NULL`; index `(company_id, entry_date)`; FK journal / reversal (self) RESTRICT; CHECK status; **CHECK ไม่ใช่ draft ⇒ มี entry_no / งวด / ผู้ผ่าน**; **CHECK ไม่ใช่ draft ⇒ total_debit = total_credit > 0** |
| `ledger_journal_lines` | entry_id, line_no, account_id, description, debit, credit, branch_id, department_id, cost_center_id, project_site_id | PK `(entry_id, line_no)`; FK entry CASCADE, FK account RESTRICT; index account_id; **CHECK debit ≥ 0, credit ≥ 0 และมีด้านเดียวที่ > 0** |
| `ledger_journal_sequences` | journal_id, fiscal_year, company_id, last_no | PK `(journal_id, fiscal_year)`; FK journal |

- จำนวนเงินเป็น `NUMERIC(24,6)` — ทศนิยมที่ยอมรับกำหนดต่อบริษัท (`amount_decimals`)
- `company_id`, `fiscal_period_id` และมิติวิเคราะห์ไม่มี FK ข้าม module — ตรวจผ่าน organization ตอนบันทึก / ผ่านรายการ
- ข้อจำกัดที่รู้: organization ลบ (soft delete) สาขา / แผนก ฯลฯ ที่ถูกใช้ในบรรทัดรายการได้ เพราะไม่รู้จักตารางของ ledger — id ในรายการเดิมยังอยู่ครบ
- **รายงานคำนวณจากบรรทัดรายการทุกครั้ง** (ยังไม่มีตารางยอดสะสม):
  - รายงานแต่ละตัวใช้ `SUM(CASE ...)` ต่อบัญชี 1 query ครอบทุกช่วงวันที่
  - บวกผังบัญชีอีก 1 query

## 3.3.2 ตาราง — inventory

| ตาราง | คอลัมน์หลัก | Constraint / Index |
|---|---|---|
| `inventory_uoms`, `inventory_item_categories` | id, company_id, code, name, status, deleted_at | partial UNIQUE `(company_id, code)`; CHECK status |
| `inventory_warehouses` | + branch_id (organization, ไม่มี FK) | partial UNIQUE code |
| `inventory_items` | id, company_id, code, name, name_en, type, uom_id, category_id, reorder_point, status, deleted_at | partial UNIQUE code; FK uom / category RESTRICT; CHECK type, CHECK reorder ≥ 0 |
| `inventory_documents` | id, company_id, doc_type, document_no, doc_date, warehouse_id, to_warehouse_id, fiscal_period_id, reference, description, status, total_value, posted_at/by, cancelled_at/by, created_by | UNIQUE `(company_id, document_no) WHERE document_no IS NOT NULL`; index `(company_id, doc_date)`; **CHECK ผ่านแล้วต้องมีเลขที่ / งวด / ผู้ผ่าน**; **CHECK transfer ต้องมีคลังปลายทางที่ต่างกัน**; CHECK ยกเลิกต้องมีผู้ยกเลิก |
| `inventory_document_lines` | document_id, line_no, item_id, quantity, unit_cost, value, description | PK `(document_id, line_no)`; FK CASCADE / item RESTRICT; CHECK quantity ≠ 0, unit_cost ≥ 0, value ≥ 0 |
| `inventory_layers` | id, company_id, item_id, warehouse_id, layer_date, document_id, line_no, quantity, value, remaining_quantity, remaining_value | **partial index `(company_id, item_id, warehouse_id, layer_date, id) WHERE remaining_quantity > 0`**; **CHECK 0 ≤ remaining ≤ quantity** |
| `inventory_movements` | id, company_id, item_id, warehouse_id, movement_date, document_id, line_no, quantity (±), value (±) | index `(company_id, item_id, warehouse_id, movement_date)`; CHECK quantity ≠ 0 |
| `inventory_allocations` | movement_id, layer_id, quantity, value | PK คู่; FK RESTRICT ทั้งสองฝั่ง |
| `inventory_sequences` | company_id, doc_type, fiscal_year, last_no | PK `(company_id, doc_type, fiscal_year)` |
| `inventory_settings` | company_id, default_purchase_account_id, default_inventory_account_id, cogs_account_id, journal_id | PK company_id — แถวนี้ใช้ lock การเปลี่ยนระบบบัญชี (FOR UPDATE) เทียบกับการลงบัญชีซื้อ (FOR SHARE) |
| `inventory_accounting_systems` | company_id, from_year, system | PK `(company_id, from_year)`; CHECK system IN (perpetual, periodic) |
| `inventory_period_closings` | id, company_id, fiscal_period_id, fiscal_year, period_end, entry_id, entry_no, adjustment, status, created_by, cancelled_at/by | **partial UNIQUE `(company_id, fiscal_period_id) WHERE status = 'posted'`** (งวดละครั้ง); index `(company_id, period_end)` |

เพิ่มในตารางเดิม: `inventory_items` / `inventory_item_categories` มี `purchase_account_id`, `inventory_account_id` (บัญชีของ ledger, ไม่มี FK); `inventory_documents` มี `fiscal_year` (ใช้ตรวจการเปลี่ยนระบบบัญชี — เอกสารเก่าเติมจากเลขที่ตอน migrate)

- จำนวน / มูลค่าทั้งหมดเป็น `NUMERIC(24,6)`
- ยอดคงเหลือ = ผลรวมของ `inventory_movements` (เท่ากับ remaining ของชั้นทั้งหมด) — ไม่มีตารางยอดสะสมแยก
- ยกเลิกเอกสาร: ชั้นที่เอกสารสร้างถูกปิด (remaining = 0, ไม่ลบ — ประวัติ allocation อ้างถึง), ชั้นที่ถูกตัดได้คืน, เพิ่มการเคลื่อนไหวกลับรายการ

## 3.3.3 ตาราง — purchasing

| ตาราง | คอลัมน์หลัก | Constraint / Index |
|---|---|---|
| `purchasing_settings` | company_id, vat_rate, journal_id, grir_account_id, ap_account_id, input_vat_account_id, price_variance_account_id | PK company_id; CHECK vat_rate 0–100 |
| `purchasing_suppliers` | id, company_id, code, name, tax_id, branch_no, address, payment_term_days, vat_registered, status, deleted_at | partial UNIQUE code; CHECK term 0–365 |
| `purchasing_orders` | id, company_id, order_no, fiscal_year, supplier_id, order_date, delivery_date, **vat_registered, vat_rate, price_includes_vat, payment_term_days** (เงื่อนไขที่เก็บตอนสร้าง), total_net/vat/gross, status, approved/closed/cancelled by/at | UNIQUE `(company_id, order_no) WHERE order_no IS NOT NULL`; FK supplier RESTRICT; **CHECK ผู้ขายไม่จด VAT ⇒ อัตรา 0 และราคาไม่รวม VAT**; CHECK อนุมัติแล้วต้องมีเลขที่ |
| `purchasing_order_lines` | order_id, line_no, item_id, quantity, unit_price, net, vat, gross, received_qty, invoiced_qty, received_value, cleared_value | **CHECK 0 ≤ received ≤ quantity, 0 ≤ invoiced ≤ received** |
| `purchasing_receipts` (+ `_lines`) | id, document_no, fiscal_year, fiscal_period_id, order_id, supplier_id, receipt_date, warehouse_id, system, stock_document_id/no, total_value, entry_id/no, status; บรรทัด: order_line_no, item_id, stock, quantity, unit_cost, value, account_id | UNIQUE `(company_id, document_no)`; FK order / supplier RESTRICT |
| `purchasing_invoices` (+ `_lines`) | id, document_no, supplier_invoice_no, invoice_date, posting_date, due_date, vat_rate, price_includes_vat, total_net/vat/gross, entry_id/no, status; บรรทัด: quantity, unit_price, net, vat, gross, cleared, variance_account_id | UNIQUE `(company_id, document_no)`; **partial UNIQUE `(company_id, supplier_id, supplier_invoice_no) WHERE status = 'posted'`**; CHECK gross = net + vat |
| `purchasing_sequences` | company_id, prefix (PO / RC / PI), fiscal_year, last_no | PK `(company_id, prefix, fiscal_year)` |

- สินค้า / คลัง / บัญชี / งวด / เอกสารสต็อก / รายการบัญชี เก็บเป็น id ของ module เจ้าของ (ไม่มี FK ข้าม module)

## 3.4 ข้อตกลงด้านข้อมูล

| เรื่อง | ข้อตกลง |
|---|---|
| Soft delete | master data ของ organization ใช้ `deleted_at`; unique index เป็นแบบ partial จึงนำรหัสกลับมาใช้ได้หลังลบ |
| Dependency check | ก่อนลบตรวจว่ามีแถวที่ยังไม่ถูกลบอ้างถึงหรือไม่ (409) — FK RESTRICT เป็นด่านสุดท้าย |
| Uniqueness | use case ตรวจซ้ำเพื่อให้ error อ่านง่าย แต่ UNIQUE ใน DB คือตัวตัดสินจริง (repository แปลง unique violation → domain error) |
| ลำดับชั้น | BU / แผนกตรวจวนลูปด้วย recursive CTE (คิวรีเดียว) ก่อนบันทึก |
| ข้าม module | FK อยู่ภายใน module เท่านั้น — identity เก็บ `company_id` โดยไม่มี FK ไป `organization_companies` เพื่อให้ module แยก schema ได้อิสระ |
| Timestamp | `created_at` / `updated_at` เป็นเรื่องของ persistence (GORM ตั้งให้) — entity ไม่รู้ |

## 3.5 Query pattern

- List ทุกตัว: คิวรีเดียวได้ทั้งหน้าและ `total` ด้วย `COUNT(*) OVER()`; นับแยกเฉพาะเมื่อขอหน้าที่เกินหน้าสุดท้าย
- ข้อมูลที่อ้างถึง (parent, branch, department ...) ดึงด้วย `LEFT JOIN` คืนเป็น `{id, code, name}`
- ข้อมูลแบบ one-to-many (เช่น role ของสมาชิก, permission ของ role) ดึงคิวรีที่สองแบบ `IN (ids ของหน้านั้น)` — ไม่ใช่ N+1
- `GET /me/companies` ใช้ 2 คิวรี เพราะข้อมูลอยู่คนละ module (JOIN ข้าม module ไม่ได้):
  1. identity: `company_id` ที่ผู้ใช้เป็นสมาชิก
  2. organization: `WHERE id IN (...)` + ค้นหา / เรียง / แบ่งหน้าใน DB
- การเขียนข้อมูลของบริษัทอ่าน `status` ของบริษัทเพิ่ม 1 คิวรี (primary key) เพื่อบังคับกฎ inactive = อ่านอย่างเดียว
- `ORDER BY` สร้างจาก whitelist ของ sort key เสมอ และมี tie-breaker (`id`) ให้การแบ่งหน้านิ่ง
