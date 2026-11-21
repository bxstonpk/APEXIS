# 4. API

Base URL: `/api/v1` — JSON เท่านั้น, ทุก endpoint (ยกเว้นสมัคร / ล็อกอิน / `GET /healthz`) ต้องส่ง

```http
Authorization: Bearer <access_token>
```

## 4.1 ข้อตกลง REST

| Method | ใช้กับ | Status เมื่อสำเร็จ |
|---|---|---|
| `GET` | อ่าน / ค้นหา (ค้นหาผ่าน URL query เท่านั้น — ไม่มี `POST /search`) | 200 |
| `POST` | สร้าง resource ใหม่ใน collection | 201 |
| `PUT` | แทนที่ข้อมูลทั้งก้อน | 200 (ยกเว้น `PUT /me/password` → 204) |
| `PATCH` | แก้บางส่วน (ตอนนี้ใช้เปลี่ยน status) | 200 |
| `DELETE` | ลบ | 204 |

path เป็นคำนามพหูพจน์ kebab-case (`/business-units`), ไม่มีคำกริยาใน path

## 4.2 การค้นหาผ่าน URL query

frontend ใช้ query string ของหน้าเป็น state ของการค้นหา แล้วส่งต่อให้ API ตรงๆ — กด Back / refresh / bookmark แล้วได้ผลเดิม

```text
หน้า frontend:  /organization/departments?q=acc&status=active&sort=-name&page=2
เรียก API:      GET /api/v1/companies/{id}/departments?q=acc&status=active&sort=-name&page=2
```

| Parameter | ความหมาย | ตัวอย่าง |
|---|---|---|
| `q` | คำค้น (contains, ไม่สนตัวพิมพ์) ใน code / name | `q=bkk` |
| `status` | หลายค่าคั่นด้วย `,` | `status=active,inactive` |
| `sort` | หลาย key คั่นด้วย `,`, นำหน้า `-` = มากไปน้อย, ใช้ได้เฉพาะ key ที่กำหนด | `sort=-start_date,code` |
| `page` | เริ่มที่ 1 | `page=2` |
| `page_size` | ค่าเริ่มต้น 20, สูงสุด 100 | `page_size=50` |
| วันที่ | `YYYY-MM-DD` (ค.ศ.) | `start_date=2026-12-19&end_date=2026-12-31` |

- parameter ที่ไม่รู้จัก → ไม่สนใจ
- ค่าผิดรูปแบบของ parameter ที่รู้จัก → `400` พร้อม `error.code = "invalid_query_parameter"`

## 4.3 รูปแบบ response

```json
// รายการเดียว
{ "data": { "id": "…", "code": "HQ", "name": "สำนักงานใหญ่" } }

// รายการ
{ "data": [ … ], "meta": { "page": 1, "page_size": 20, "total": 57 } }

// error
{ "error": { "code": "code_already_exists", "message": "code already exists" } }
```

| Status | ความหมาย |
|---|---|
| 400 | JSON / query / ค่า field ไม่ถูกต้อง |
| 401 | ไม่มี token, token ผิด / หมดอายุ, ล็อกอินผิด |
| 403 | ไม่มีสิทธิ์, ไม่ใช่สมาชิกของบริษัท หรือผู้ใช้ / สมาชิกไม่ active |
| 404 | ไม่พบ (รวมถึงข้อมูลของบริษัทอื่น) |
| 409 | ซ้ำ, ยังถูกใช้งานอยู่, ช่วงปีบัญชีซ้อน |
| 422 | ผิดกฎธุรกิจ (บริษัท inactive, วนลูปลำดับชั้น, สำนักงานใหญ่, ลำดับปิดงวด, system role ...) |
| 500 | ข้อผิดพลาดภายใน — ไม่ส่งรายละเอียด SQL / ระบบให้ client |

## 4.4 Endpoint — identity

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| POST | `/users` | public | `{email, name, password}` |
| POST | `/auth/tokens` | public | `{email, password}` → token |
| GET | `/me` | login | |
| PUT | `/me/password` | login | `{old_password, new_password}` |
| GET | `/companies/{companyID}/access` | สมาชิก | role + permission ของตัวเอง (frontend ใช้ซ่อน/แสดงเมนู) |
| GET | `/companies/{companyID}/roles` | `identity.role:read` | `?q=&is_system=&sort=code\|name\|is_system` |
| POST | `/companies/{companyID}/roles` | `identity.role:manage` | `{code, name, description, permissions[]}` |
| PUT | `/companies/{companyID}/roles/{roleID}` | `identity.role:manage` | `{name, description, permissions[]}` |
| DELETE | `/companies/{companyID}/roles/{roleID}` | `identity.role:manage` | |
| GET | `/companies/{companyID}/members` | `identity.member:read` | `?q=&status=active,suspended&role_id=&sort=name\|email\|status\|joined_at` |
| POST | `/companies/{companyID}/members` | `identity.member:manage` | `{email, role_ids[]}` |
| PATCH | `/companies/{companyID}/members/{userID}` | `identity.member:manage` | `{status: "active"\|"suspended"}` |
| PUT | `/companies/{companyID}/members/{userID}/roles` | `identity.member:manage` | `{role_ids[]}` แทนที่ทั้งชุด |

## 4.5 Endpoint — organization

### บริษัท

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| POST | `/companies` | login | ผู้สร้างเป็น owner — 201 |
| GET | `/me/companies` | login | บริษัทที่ตัวเองเป็นสมาชิก `?q=&status=&sort=name\|code&page=&page_size=` |
| GET | `/companies/{companyID}` | `organization.company:read` | |
| PUT | `/companies/{companyID}` | `organization.company:manage` | แทนที่ทั้งก้อน (ยกเว้น `code`), ใช้ `status` เปิด/ปิดบริษัท; `amount_decimals` / `quantity_decimals` (0–6) เพิ่มได้ ลดไม่ได้ |

### ข้อมูลภายใต้บริษัท

prefix: `/api/v1/companies/{companyID}` — อ่านต้องมีสิทธิ์ `:read`, เขียนต้องมี `:manage` ของ resource นั้น และบริษัทต้อง `active`

| Resource | Endpoints | URL query (นอกจาก `q, sort, page, page_size`) | sort key |
|---|---|---|---|
| Branch | `GET`, `POST /branches`; `GET`, `PUT`, `DELETE /branches/{branchID}` | `status` | `code, name, tax_branch_no` |
| Business unit | `/business-units[/{businessUnitID}]` (เหมือนด้านบน) | `status, parent_id` | `code, name` |
| Department | `/departments[/{departmentID}]` | `status, parent_id, business_unit_id, branch_id` | `code, name` |
| Cost center | `/cost-centers[/{costCenterID}]` | `status, department_id, active_on` | `code, name, valid_from` |
| Project site | `/project-sites[/{projectSiteID}]` | `status` (planned/active/closed), `branch_id, cost_center_id, start_date, end_date` (ช่วงที่ซ้อนทับ) | `code, name, start_date` |
| Fiscal year | `GET`, `POST /fiscal-years`; `GET /fiscal-years/{fiscalYearID}` | `year` | `year` (ค่าเริ่มต้น `-year`) |
| Fiscal period | `GET /fiscal-periods`; `GET`, `PATCH /fiscal-periods/{fiscalPeriodID}` | `fiscal_year, status` (open/closed), `date` | `start_date` |

### Request body (organization)

```jsonc
// POST /companies   (ต้องมีแค่ code, name — ที่เหลือมีค่าเริ่มต้น)
{ "code": "ACME", "name": "ACME", "tax_id": "0105556000000" }

// PUT /companies/{companyID}   (แทนที่ทั้งก้อน — ต้องส่ง name, base_currency, fiscal_year_start_month, status; code แก้ไม่ได้)
{ "name": "ACME", "tax_id": "0105556000000",
  "legal_name_th": "บริษัท แอคมี จำกัด", "legal_name_en": "ACME Co., Ltd.",
  "address": { "line": "", "sub_district": "", "district": "", "province": "", "postal_code": "10110", "country": "TH" },
  "phone": "", "email": "", "website": "", "base_currency": "THB", "fiscal_year_start_month": 1,
  "amount_decimals": 2, "quantity_decimals": 2, "status": "active" }

// POST/PUT /branches   (status: ค่าเริ่มต้น active ตอน POST, บังคับส่งตอน PUT)
{ "code": "HQ", "name": "สำนักงานใหญ่", "tax_branch_no": "00000", "address": { … }, "phone": "", "status": "active" }

// POST/PUT /business-units
{ "code": "RETAIL", "name": "ค้าปลีก", "parent_id": null, "status": "active" }

// POST/PUT /departments
{ "code": "ACC", "name": "บัญชี", "parent_id": null, "business_unit_id": null, "branch_id": null, "status": "active" }

// POST/PUT /cost-centers
{ "code": "CC100", "name": "…", "department_id": null, "valid_from": "2026-01-01", "valid_to": null, "status": "active" }

// POST/PUT /project-sites   (status: planned | active | closed — closed ต้องมี end_date)
{ "code": "P001", "name": "…", "branch_id": null, "cost_center_id": null, "address": { … },
  "start_date": "2026-01-01", "end_date": null, "status": "planned" }

// POST /fiscal-years   (start_date ต้องเป็นวันที่ 1 ของเดือน → สร้าง 12 งวด)
{ "year": 2026, "start_date": "2026-01-01" }

// PATCH /fiscal-periods/{id}   (ต้องมีสิทธิ์ organization.fiscal_period:close)
{ "status": "closed" }
```

## 4.6 Endpoint — ledger

prefix: `/api/v1/companies/{companyID}` — การเขียนทุกตัวต้องให้บริษัท `active`

**จำนวนเงิน / จำนวนสินค้า** (ใช้กับ ledger และ inventory):
- request รับเป็น string `"1234.5"` (แนะนำ) หรือ number — ทศนิยมไม่เกินที่บริษัทกำหนด (`amount_decimals` / `quantity_decimals`) ไม่งั้น 400
- response ส่งเป็น string ตามค่าจริง เช่น `"1200"`, `"30000.5"` — frontend จัดรูปแบบการแสดงผลตามทศนิยมของบริษัท

### ผังบัญชี / สมุดรายวัน

| Resource | Endpoints | URL query (นอกจาก `q, sort, page, page_size`) | sort key | Permission |
|---|---|---|---|---|
| Account | `GET`, `POST /accounts`; `GET`, `PUT`, `DELETE /accounts/{accountID}` | `type` (asset,liability,…), `status`, `postable=true\|false`, `parent_id` | `code, name, type` | `ledger.account:read\|manage` |
| Journal | `GET`, `POST /journals`; `GET`, `PUT`, `DELETE /journals/{journalID}` | `status` | `code, name` | `ledger.journal:read\|manage` |

### รายการบัญชี

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| GET | `/journal-entries` | `ledger.entry:read` | `?q=&status=draft,posted,reversed&journal_id=&account_id=&start_date=&end_date=&sort=entry_date\|entry_no\|created_at` (ค่าเริ่มต้น `-entry_date,-entry_no`) |
| POST | `/journal-entries` | `ledger.entry:manage` (+ `ledger.entry:post` ถ้า `status: "posted"`) | สร้าง draft หรือสร้างแล้วผ่านทันที — 201 |
| GET | `/journal-entries/{entryID}` | `ledger.entry:read` | พร้อมบรรทัด (ชื่อบัญชีจาก JOIN; มิติวิเคราะห์เป็น id) |
| PUT | `/journal-entries/{entryID}` | `ledger.entry:manage` | แทนที่ draft ทั้งก้อน |
| PATCH | `/journal-entries/{entryID}` | `ledger.entry:post` | `{"status": "posted"}` — เปลี่ยนได้ทางเดียว |
| DELETE | `/journal-entries/{entryID}` | `ledger.entry:manage` | เฉพาะ draft — 204 |
| POST | `/journal-entries/{entryID}/reversals` | `ledger.entry:post` | `{"date": "2026-12-31", "description": ""}` → คืนรายการกลับที่ผ่านแล้ว — 201 |

### รายงาน (`ledger.report:read`) — endpoint เดียว เลือกด้วย `book`

หน้ารายงานของ frontend เป็นหน้าเดียว:
- ฟอร์มเลือกรายงานจะตั้ง `book` และใช้ parameter ชื่อเดียวกันทุกรายงาน
- parameter ที่รายงานนั้นไม่ใช้ถูกเพิกเฉย จึงเปลี่ยน `book` โดยคงค่าอื่นใน URL ไว้ได้

```text
หน้า frontend:  /ledger/reports?book=balance-sheet&start_date=2026-01-01&end_date=2026-12-31
เรียก API:      GET /api/v1/companies/{id}/reports?book=balance-sheet&start_date=2026-01-01&end_date=2026-12-31
```

| `book` | parameter ที่ใช้ | ผลลัพธ์ |
|---|---|---|
| `trial-balance` | `start_date`, `end_date` (บังคับ, ปีบัญชีเดียวกัน) | งบทดลอง — ยอดเดบิตเป็นบวก; `prior_years_earnings` + ผลรวม `closing_balance` = 0 |
| `balance-sheet` | `end_date` (บังคับ = ณ วันที่; `start_date` ไม่ใช้) | งบแสดงฐานะการเงิน — ยอดตามด้านปกติของหมวด, บรรทัดมี `level`, `is_header` |
| `income-statement` | `start_date`, `end_date` (บังคับ) | งบกำไรขาดทุน |
| `general-ledger` | `account_id`, `start_date`, `end_date` (บังคับ), `page`, `page_size` | บัญชีแยกประเภทของบัญชีเดียว — มี `meta` สำหรับแบ่งหน้าบรรทัด; `balance` คือยอดสะสมรวมยอดยกมา |

- response ทุกรายงานมี `data.book` บอกชนิดรายงาน — frontend ใช้เลือกรูปแบบการแสดงผล
- `book` ไม่มี / ไม่รู้จัก, หรือขาด parameter ที่บังคับ → `400 invalid_query_parameter` (ข้อความบอกค่าที่ใช้ได้)

### Request body (ledger)

```jsonc
// POST/PUT /accounts   (postable: ค่าเริ่มต้น true ตอน POST, บังคับส่งตอน PUT)
{ "code": "1101", "name": "เงินสด", "name_en": "Cash", "type": "asset", "parent_id": "<id บัญชีคุม>", "postable": true, "status": "active" }

// POST/PUT /journals
{ "code": "JV", "name": "สมุดรายวันทั่วไป", "status": "active" }

// POST/PUT /journal-entries   (status เฉพาะ POST: "draft" ค่าเริ่มต้น | "posted")
{ "journal_id": "…", "date": "2026-01-10", "description": "ค่าเช่า", "reference": "INV-001", "status": "draft",
  "lines": [
    { "account_id": "…", "debit": "1200", "description": "ค่าเช่าเดือนมกราคม",
      "branch_id": null, "department_id": null, "cost_center_id": null, "project_site_id": null },
    { "account_id": "…", "credit": "1200" } ] }
```

## 4.7 Endpoint — inventory

prefix: `/api/v1/companies/{companyID}` — การเขียนทุกตัวต้องให้บริษัท `active`

| Resource | Endpoints | URL query (นอกจาก `q, sort, page, page_size`) | Permission |
|---|---|---|---|
| หน่วยนับ | `GET`, `POST /uoms`; `GET`, `PUT`, `DELETE /uoms/{uomID}` | `status` | `inventory.uom:read\|manage` |
| หมวดสินค้า | `/item-categories[/{categoryID}]` | `status` | `inventory.category:read\|manage` |
| คลัง | `/warehouses[/{warehouseID}]` | `status, branch_id` | `inventory.warehouse:read\|manage` |
| สินค้า | `/items[/{itemID}]` | `type` (stock,non_stock,service), `status, category_id, uom_id` | `inventory.item:read\|manage` |

### เอกสารสต็อก

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| GET | `/stock-documents` | `inventory.document:read` | `?q=&type=receipt,issue,transfer,adjustment&status=&warehouse_id=&item_id=&start_date=&end_date=&sort=date\|document_no\|created_at` |
| POST | `/stock-documents` | `inventory.document:manage` (+ `post` ถ้า `status: "posted"`) | 201 |
| GET | `/stock-documents/{documentID}` | `inventory.document:read` | พร้อมบรรทัด (มูลค่า FIFO หลังผ่าน) |
| PUT | `/stock-documents/{documentID}` | `inventory.document:manage` | แทนที่ draft ทั้งก้อน |
| PATCH | `/stock-documents/{documentID}` | `inventory.document:post` | `{"status": "posted"}` ผ่าน / `{"status": "cancelled"}` ยกเลิก |
| DELETE | `/stock-documents/{documentID}` | `inventory.document:manage` | เฉพาะ draft — 204 |

```jsonc
// POST /stock-documents
{ "type": "receipt", "date": "2026-01-05", "warehouse_id": "…", "reference": "PO-001", "status": "draft",
  "lines": [ { "item_id": "…", "quantity": "10", "unit_cost": "12.50" } ] }
// receipt: unit_cost บังคับ | issue / transfer: ห้ามส่ง unit_cost (FIFO)
// transfer: ต้องมี "to_warehouse_id" | adjustment: quantity บวก = เพิ่ม (ต้องมี unit_cost), ลบ = ลด (FIFO)
```

### รายงาน (`inventory.report:read`) — ผ่าน `GET /reports?book=` เดียวกับ ledger

| `book` | parameter | ผลลัพธ์ |
|---|---|---|
| `stock-on-hand` | `end_date` (บังคับ = ณ วันที่), `warehouse_id, item_id, category_id, q, sort` (item_code, item_name, warehouse_code, quantity, value), `page, page_size` | แถวต่อสินค้า × คลัง: จำนวน + มูลค่า; `total_value` = มูลค่ารวมทุกแถวที่ตรงเงื่อนไข; `meta` แบ่งหน้า |
| `stock-card` | `item_id, start_date, end_date` (บังคับ), `warehouse_id` (ไม่ส่ง = ทุกคลัง), `page, page_size` | ยอดยกมา + การเคลื่อนไหว (± จำนวน / มูลค่า) พร้อมยอดสะสม; `meta` แบ่งหน้า |

### ค่าตั้งบัญชีสินค้า / ปรับปรุงสิ้นงวด

| Method | Path | สิทธิ์ | หมายเหตุ |
|---|---|---|---|
| GET / PUT | `/inventory-settings` | `inventory.settings:read\|manage` | `default_purchase_account_id, default_inventory_account_id, cogs_account_id, journal_id` + `accounting_systems` (อ่านอย่างเดียว) |
| PUT | `/inventory-settings/accounting-systems/{fiscalYear}` | `inventory.settings:manage` | `{"system": "periodic"}` — ใช้ตั้งแต่ปีนั้น; ปีที่มีเอกสารผ่านแล้ว = 422 |
| GET | `/inventory-closings` | `inventory.closing:read` | `?fiscal_year=&status=posted,cancelled&sort=period_end\|created_at` |
| POST | `/inventory-closings` | `inventory.closing:post` | `{"date": "2026-01-31"}` (วันใดก็ได้ในงวด) → 201 พร้อม `adjustment`, `entry_no` |
| PATCH | `/inventory-closings/{closingID}` | `inventory.closing:post` | `{"status": "cancelled"}` กลับรายการบัญชี |

หมวดสินค้า / สินค้า รับ `purchase_account_id`, `inventory_account_id` เพิ่ม (ไม่ส่ง = ใช้ของหมวด / ค่าเริ่มต้น)

## 4.8 Endpoint — purchasing

| Resource | Path | Filter เพิ่ม | สิทธิ์ |
|---|---|---|---|
| ค่าตั้ง | `GET`, `PUT /purchasing-settings` | — | `purchasing.settings:read\|manage` |
| ผู้ขาย | `/suppliers[/{supplierID}]` | `status, vat_registered` | `purchasing.supplier:read\|manage` |
| ใบสั่งซื้อ | `GET`, `POST /purchase-orders`; `GET`, `PUT`, `PATCH`, `DELETE /purchase-orders/{orderID}` | `status, supplier_id, item_id, start_date, end_date`; sort `date\|order_no\|created_at` | `purchasing.order:read\|manage`; `PATCH {"status":"approved"}` ต้องมี `purchasing.order:approve` |
| ใบรับสินค้า | `GET`, `POST /goods-receipts`; `GET`, `PATCH /goods-receipts/{receiptID}` | `status, supplier_id, order_id, start_date, end_date` | `purchasing.receipt:read\|post` |
| ใบแจ้งหนี้ | `GET`, `POST /supplier-invoices`; `GET`, `PATCH /supplier-invoices/{invoiceID}` | เหมือนใบรับสินค้า | `purchasing.invoice:read\|post` |

- ใบสั่งซื้อ `PATCH` รับ `approved` / `closed` / `cancelled`; ใบรับ / ใบแจ้งหนี้ `PATCH` รับเฉพาะ `cancelled`
- ใบรับ / ใบแจ้งหนี้ไม่มี draft — `POST` = ลงบัญชีทันที (response มี `entry_no` ของ ledger, ใบรับมี `stock_document_no` ของ inventory)

```jsonc
// POST /purchase-orders
{ "supplier_id": "…", "date": "2026-02-01", "delivery_date": "2026-02-10", "price_includes_vat": false,
  "lines": [ { "item_id": "…", "quantity": "10", "unit_price": "100" } ] }
// POST /goods-receipts — warehouse_id บังคับเมื่อมีสินค้า stock
{ "order_id": "…", "date": "2026-02-10", "warehouse_id": "…", "lines": [ { "order_line_no": 1, "quantity": "4" } ] }
// POST /supplier-invoices — unit_price ไม่ส่ง = ราคาใบสั่งซื้อ; invoice_date ไม่ส่ง = date
{ "order_id": "…", "supplier_invoice_no": "INV-001", "date": "2026-02-15",
  "lines": [ { "order_line_no": 1, "quantity": "4", "unit_price": "110" } ] }
```

### รายงาน (`purchasing.report:read`)

| `book` | parameter | ผลลัพธ์ |
|---|---|---|
| `outstanding-purchase-orders` | `supplier_id, item_id, q, sort` (date, order_no, supplier_code, delivery_date), `page, page_size` | บรรทัดใบสั่งซื้อที่ค้างรับ (`to_receive`) หรือค้างแจ้งหนี้ (`to_invoice`) |
| `ap-aging` | `end_date` (บังคับ), `supplier_id, q, page, page_size` | ยอดค้างต่อผู้ขาย: `not_due, days_1_30, days_31_60, days_61_90, over_90, total` + `totals` รวมทุกผู้ขาย |
