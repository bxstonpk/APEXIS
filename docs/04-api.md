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
| GET | `/me` | login | ผู้ใช้มี `must_change_password` (true = บัญชีที่ผู้ดูแลสร้างให้ ยังใช้รหัสผ่านชั่วคราว) |
| PUT | `/me/password` | login | `{old_password, new_password}` |
| GET | `/companies/{companyID}/access` | สมาชิก | role + permission ของตัวเอง (frontend ใช้ซ่อน/แสดงเมนู) |
| GET | `/companies/{companyID}/roles` | `identity.role:read` | `?q=&is_system=&sort=code\|name\|is_system` |
| GET | `/companies/{companyID}/roles/{roleID}` | `identity.role:read` | บทบาทหนึ่งรายการ (หน้าแก้ไขบทบาท) |
| POST | `/companies/{companyID}/roles` | `identity.role:manage` | `{code, name, description, permissions[]}` |
| PUT | `/companies/{companyID}/roles/{roleID}` | `identity.role:manage` | `{name, description, permissions[]}` |
| DELETE | `/companies/{companyID}/roles/{roleID}` | `identity.role:manage` | |
| GET | `/companies/{companyID}/members` | `identity.member:read` | `?q=&status=active,suspended&role_id=&sort=name\|email\|status\|joined_at` |
| POST | `/companies/{companyID}/members` | `identity.member:manage` | `{email, role_ids[], new_user?: {name, password}}` — ไม่มี `new_user` + ไม่พบบัญชี = 404 `user_not_found`; มี `new_user` = สร้างบัญชี (รหัสผ่านชั่วคราว, `must_change_password`) และสมาชิกใน transaction เดียว, อีเมลมีบัญชีแล้ว = 409 |
| GET | `/companies/{companyID}/members/{userID}` | `identity.member:read` | สมาชิกหนึ่งคน (หน้าจัดการสมาชิก) |
| PATCH | `/companies/{companyID}/members/{userID}` | `identity.member:manage` | `{status: "active"\|"suspended"}` |
| PUT | `/companies/{companyID}/members/{userID}/roles` | `identity.member:manage` | `{role_ids[]}` แทนที่ทั้งชุด |

### รหัสผ่านชั่วคราว

บัญชีที่ผู้ดูแลสร้างให้ (`new_user`) มี `must_change_password = true` จนกว่าจะเรียก `PUT /me/password`
- ระหว่างนั้นทุก API ใต้บริษัทตอบ 403 `password_change_required` (ตรวจใน access resolution เดียวกับสิทธิ์ — ไม่มีคิวรีเพิ่ม)
- `/me`, `/me/password`, `/me/companies` ยังใช้ได้

### รายการสิทธิ์ (`platform/permissions`)

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| GET | `/permissions` | login | สิทธิ์ทั้งระบบ `[{code, module, resource, action}]` ตามลำดับที่ module ลงทะเบียน — หน้ากำหนดบทบาทสร้างตารางสิทธิ์จากรายการนี้ |

แต่ละ module ประกาศ `AllPermissions` ใน `domain/value_objects` และ `Module.Permissions()`; `main.go` ลงทะเบียนทุก module (รหัสผิดรูปแบบ / ซ้ำ = panic ตอนเริ่มโปรแกรม)

## 4.5 Endpoint — organization

### บริษัท

| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| POST | `/companies` | login | ผู้สร้างเป็น owner + ตั้งผังบัญชีมาตรฐานและค่าตั้งขาย / จัดซื้อ / คลัง / การเงินตาม `business_type` ใน transaction เดียว — 201 (ขั้นใดล้มเหลว = ไม่มีบริษัท) |
| GET | `/me/companies` | login | บริษัทที่ตัวเองเป็นสมาชิก `?q=&status=&sort=name\|code&page=&page_size=` |
| GET | `/me/companies/{companyID}` | สมาชิก | ข้อมูลสรุป `{id, code, name, tax_id, business_type, status}` (รายการ `/me/companies` มี `business_type` เช่นกัน) — ไม่ต้องมีสิทธิ์อ่านบริษัท (frontend ใช้แสดงชื่อบริษัท); ไม่ใช่สมาชิก = 404 |
| GET | `/me/companies/{companyID}/letterhead` | สมาชิก | หัวกระดาษเอกสารพิมพ์ `{name, legal_name_th, legal_name_en, tax_id, address{line, sub_district, district, province, postal_code, country}, phone, email, website}` — ไม่ใช่สมาชิก = 404 |
| GET | `/companies/{companyID}` | `organization.company:read` | |
| PUT | `/companies/{companyID}` | `organization.company:manage` | แทนที่ทั้งก้อน (ยกเว้น `code` / `business_type` — ส่งมาจะถูกละเว้น), ใช้ `status` เปิด/ปิดบริษัท; `amount_decimals` / `quantity_decimals` (0–6) เพิ่มได้ ลดไม่ได้ |

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
| เลขที่เอกสาร | `GET /document-numberings`; `PUT /document-numberings/{docType}` | — (ทุกชนิดเอกสาร พร้อม `example`) | `organization.numbering:read\|manage` |

### Request body (organization)

```jsonc
// POST /companies   (ต้องมี code, name, business_type — ที่เหลือมีค่าเริ่มต้น)
// business_type: service (บริการทั่วไป) | trading (ซื้อขายสินค้า) | manufacturing (ผลิตสินค้า) | construction (ก่อสร้าง) — แก้ไม่ได้ภายหลัง
{ "code": "ACME", "name": "ACME", "business_type": "trading", "tax_id": "0105556000000" }

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
| Account | `GET`, `POST /accounts`; `GET`, `PUT`, `DELETE /accounts/{accountID}` | `type` (asset,liability,equity,revenue,expense,suspense), `status`, `postable=true\|false`, `parent_id` | `code, name, type` | `ledger.account:read\|manage` |
| Method | Path | Permission | หมายเหตุ |
|---|---|---|---|
| GET | `/accounts/tree` | `ledger.account:read` | ทุกบัญชีของบริษัท (ทุกสถานะ) เรียงตามรหัส ไม่แบ่งหน้า — 1 คิวรี; หน้าผังบัญชีสร้างต้นไม้จาก `parent` (ผังบัญชีมีหลักร้อยบัญชี จึงส่งทั้งชุด) |
| POST | `/accounts/defaults` | `ledger.account:manage` | สร้างผังบัญชีมาตรฐานตามประเภทธุรกิจของบริษัท (บริการ 245 / ซื้อขาย 259 / ผลิต 274 / ก่อสร้าง 261 บัญชี — ดู 01-requirements LED-10; บริษัทใหม่ได้ผังนี้ตอนสร้างบริษัทแล้ว) + สมุด GEN / CRJ / CDJ (ข้ามรหัสสมุดที่มีอยู่) ใน transaction เดียว → 201 `{accounts, journals}`; บริษัทมีบัญชีอยู่แล้ว = 409 `chart_of_accounts_already_has_accounts` |

| Resource | Endpoints | URL query (นอกจาก `q, sort, page, page_size`) | sort key | Permission |
|---|---|---|---|---|
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
| `balance-sheet` | `end_date` (บังคับ = ณ วันที่; `start_date` ไม่ใช้) | งบแสดงฐานะการเงิน — ยอดตามด้านปกติของหมวด, บรรทัดมี `level`, `is_header`; `suspense` / `total_suspense` = บัญชีพักที่ยังมียอด (เดบิตเป็นบวก): `total_assets + total_suspense = total_liabilities_and_equity` |
| `income-statement` | `start_date`, `end_date` (บังคับ) | งบกำไรขาดทุน — `suspense` / `total_suspense` = ยอดเคลื่อนไหวของบัญชีพักในช่วง (เดบิตเป็นบวก) แสดงแยก **ไม่รวมใน `net_income`** |
| `general-ledger` | `account_id`, `start_date`, `end_date` (บังคับ), `page`, `page_size` | บัญชีแยกประเภทของบัญชีเดียว — มี `meta` สำหรับแบ่งหน้าบรรทัด; `balance` คือยอดสะสมรวมยอดยกมา |

- response ทุกรายงานมี `data.book` บอกชนิดรายงาน — frontend ใช้เลือกรูปแบบการแสดงผล
- `book` ไม่มี / ไม่รู้จัก, หรือขาด parameter ที่บังคับ → `400 invalid_query_parameter` (ข้อความบอกค่าที่ใช้ได้)

### บัญชีพัก (`type: suspense`)

หมวดที่ 6 ของผังมาตรฐาน — พักรายการที่ยังระบุไม่ได้ เช่น เงินเข้าบัญชีธนาคารแต่ยังไม่รู้ว่าเป็นของใคร (พักรับ) แล้วโอนไปบัญชีที่ถูกต้องเมื่อตรวจสอบได้ ให้ยอดตามสมุดเงินฝากตรงกับสมุดบัญชี
- ยอดสะสมต่อเนื่องแบบบัญชีงบฐานะการเงิน (ไม่เริ่มใหม่ทุกปีบัญชี) และ**ไม่เข้างบกำไรขาดทุน**
- งบแสดงฐานะการเงินแสดงแยกเป็นส่วน "บัญชีพักรอตรวจสอบ" — ควรเป็นศูนย์; หน้ารายงานเตือนเมื่อยังมียอด
- งบกำไรขาดทุนแสดงยอดเคลื่อนไหวของบัญชีพักในช่วงเป็นส่วนแยกท้ายงบ (ไม่รวมในกำไรสุทธิ) — งบระหว่างปีเห็นรายการพักชัดเจน, ก่อนปิดงบโอนไปบัญชีที่ถูกต้องจนเป็นศูนย์ (ตัดสินใจ 2026-11-23: บัญชี 6100–6300 คงเป็นบัญชีพัก)

### Request body (ledger)

```jsonc
// POST/PUT /accounts   (postable: ค่าเริ่มต้น true ตอน POST, บังคับส่งตอน PUT)
{ "code": "1101", "name": "เงินสด", "name_en": "Cash", "type": "asset", "parent_id": "<id บัญชีคุม>", "postable": true, "status": "active", "description": "เงินสดในมือ" }

// POST/PUT /journals — number_template ไม่ส่ง = "{prefix}{fy}-{seq:6}" (prefix = code)
{ "code": "JV", "name": "สมุดรายวันทั่วไป", "number_template": "{prefix}{bb}{mm}-{seq:4}", "status": "active" }

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
| ใบลดหนี้ผู้ขาย | `GET`, `POST /supplier-credit-notes`; `GET`, `PATCH /supplier-credit-notes/{creditNoteID}` | `status, supplier_id, invoice_id, start_date, end_date`; sort `date\|document_no` | `purchasing.invoice:read\|post` |

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
// POST /supplier-credit-notes — return: quantity (+ unit_price ≤ ราคาใบแจ้งหนี้), allowance: amount; credit_note_date ไม่ส่ง = date
{ "invoice_id": "…", "supplier_credit_note_no": "CN-77", "date": "2026-02-20", "kind": "return", "reason": "สินค้าชำรุด",
  "warehouse_id": "…", "lines": [ { "invoice_line_no": 1, "quantity": "1" } ] }
```

- **ใบลดหนี้ผู้ขาย** (`purchasing.credit_note`, PCN): ลงบัญชีทันที — Dr เจ้าหนี้ / Cr ภาษีซื้อ + (perpetual + ส่งคืนสินค้า stock: Cr สินค้าคงเหลือตามต้นทุน FIFO ที่ตัดออก, ส่วนต่างลงผลต่างราคา; อื่น ๆ: Cr `purchase_returns_account_id` หรือบัญชีซื้อของสินค้า)
  - ตัดยอดค้างของใบแจ้งหนี้เหมือนการจ่ายเงิน (ไม่เกินยอดค้าง, aging / ใบค้างจ่ายหักให้อัตโนมัติ); ใบแจ้งหนี้ที่มีใบลดหนี้ยกเลิกไม่ได้ (422 `invoice_has_credit_notes_(cancel_the_credit_notes_first)`)
  - คืนได้ไม่เกินจำนวนที่ยังไม่คืน / ลดได้ไม่เกินมูลค่าบรรทัดที่ยังไม่ลด (422 `credit_exceeds_what_is_left_on_the_invoice`); เลขที่ใบลดหนี้ผู้ขายซ้ำ = 409
  - ใบแจ้งหนี้มี `credited_amount` (รวม VAT) และต่อบรรทัด `line_no`, `returned_quantity`, `credited_amount`; `outstanding` = ยอดรวม − จ่าย − ลดหนี้

### รายงาน (`purchasing.report:read`)

| `book` | parameter | ผลลัพธ์ |
|---|---|---|
| `outstanding-purchase-orders` | `supplier_id, item_id, q, sort` (date, order_no, supplier_code, delivery_date), `page, page_size` | บรรทัดใบสั่งซื้อที่ค้างรับ (`to_receive`) หรือค้างแจ้งหนี้ (`to_invoice`) |
| `ap-aging` | `end_date` (บังคับ), `supplier_id, q, page, page_size` | ยอดค้างต่อผู้ขาย (หักยอดจ่าย / ลดหนี้ถึงวันที่รายงาน): `not_due, days_1_30, days_31_60, days_61_90, over_90, total` + `totals` รวมทุกผู้ขาย |
| `input-vat` | `start_date, end_date` (บังคับ, ตามวันที่บันทึกบัญชี), `supplier_id, sort` (date, document_no, supplier_name), `page, page_size` | ภาษีซื้อ: ใบแจ้งหนี้ที่มี VAT + ใบลดหนี้ผู้ขาย (ติดลบ) — `kind, tax_invoice_no, tax_invoice_date, reference_no, supplier, tax_id, branch_no, base, tax`; `total_base`, `total_tax` |

- ใบแจ้งหนี้มี `paid_amount`, `outstanding` เพิ่ม — ยอดจ่ายมาจากใบสำคัญจ่ายของการเงิน (ดู 4.10)

## 4.9 Endpoint — sales

| Resource | Path | Filter เพิ่ม | สิทธิ์ |
|---|---|---|---|
| ค่าตั้ง | `GET`, `PUT /sales-settings` | — | `sales.settings:read\|manage` |
| ลูกค้า | `/customers[/{customerID}]` | `status` | `sales.customer:read\|manage` |
| ใบเสนอราคา | `GET`, `POST /quotations`; `GET`, `PUT`, `PATCH`, `DELETE /quotations/{quotationID}` | `status, customer_id, item_id, start_date, end_date` | `sales.quotation:read\|manage` |
| ใบสั่งขาย | `GET`, `POST /sales-orders`; `GET`, `PUT`, `PATCH`, `DELETE /sales-orders/{orderID}` | เหมือนใบเสนอราคา | `sales.order:read\|manage`; `PATCH {"status":"approved"}` ต้องมี `sales.order:approve` |
| ขายสินค้า (IV: ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี) | `GET`, `POST /sales-invoices`; `GET`, `PATCH /sales-invoices/{invoiceID}` | `status, customer_id, order_id, start_date, end_date` | `sales.invoice:read\|post` |
| ขายบริการ (SV: ใบแจ้งหนี้) | `GET`, `POST /service-invoices`; `GET`, `PATCH /service-invoices/{invoiceID}`; `POST /service-invoices/{invoiceID}/tax-invoice` | เหมือนขายสินค้า | `sales.invoice:read\|post` |
| รับเงินมัดจำ (DP: ใบรับเงินมัดจำ / ใบกำกับภาษี) | `GET`, `POST /sales-deposits`; `GET /sales-deposits/open?customer_id=`; `GET`, `PATCH /sales-deposits/{invoiceID}` | เหมือนขายสินค้า | `sales.invoice:read\|post` |
| ใบวางบิล (BN) | `GET`, `POST /billing-notes`; `GET`, `PATCH /billing-notes/{billingNoteID}` | `status` (open,paid,cancelled), `customer_id, start_date, end_date` | `sales.billing_note:read\|manage` |
| ใบลดหนี้ | `GET`, `POST /sales-credit-notes`; `GET`, `PATCH /sales-credit-notes/{creditNoteID}` | `status, customer_id, invoice_id, start_date, end_date` | `sales.invoice:read\|post` |

- ลูกค้า / ผู้ขาย: ที่อยู่ส่งเป็น `address_detail` ตามแบบกรมสรรพากร `{building, room_no, floor_no, village, house_no, moo, soi, yaek, street, sub_district, district, province, postal_code}` ("-" = ว่าง, รหัสไปรษณีย์ 5 หลัก, ผิด = 400 `invalid_address`); response มี `address` = ข้อความที่ประกอบแล้ว (กรุงเทพฯ ใช้ แขวง / เขต) ใช้พิมพ์ใบกำกับภาษีและเก็บ snapshot ในเอกสารการเงิน

- **เลขผู้เสียภาษี 1 เลข = 1 รหัส** (ลูกค้า / ผู้ขาย, เฉพาะที่ไม่ว่าง): unique index `uq_sal_customers_tax_id` / `uq_pur_suppliers_tax_id` — ซ้ำ = 409 `tax_id_already_belongs_to_another_customer` / `..._supplier` (message มีรหัสเดิม)
- **สาขา**: `branch_no` / `address_detail` ของลูกค้า = สาขาหลัก; สาขาอื่น `branches: [{branch_no, name, address_detail}]` (PUT แทนที่ทั้งชุด — ไม่ส่ง = ไม่มีสาขาอื่น; response มี `address` และส่งเฉพาะ GET รายตัว); เลขสาขาซ้ำ (รวมสาขาหลัก) = 400 `duplicate_customer_branch_number`
- ใบเสนอราคา / ใบสั่งขายรับ `customer_branch_no` (ว่าง = สาขาหลัก; ไม่มีในลูกค้า = 422 `customer_branch_not_found`) — ใบสั่งขายจากใบเสนอราคาใช้สาขาตามใบเสนอราคา
- ใบแจ้งหนี้ / ใบกำกับภาษีมี `bill_to {name, tax_id, branch_no, address}` = ผู้ซื้อ ณ วันออก (สาขาตามใบสั่งขาย) — แก้ทะเบียนลูกค้าภายหลังไม่กระทบ; รายงานภาษีขายใช้ค่านี้
- **ใบลดหนี้** (`sales.credit_note`, CN — ม.86/10): `POST {invoice_id, date, kind: return|allowance, reason, description?, warehouse_id?, lines:[{invoice_line_no, quantity?, unit_price?, amount?}]}`
  - ลงบัญชีทันที: Dr รับคืน / ส่วนลด (`sales_returns_account_id` หรือบัญชีรายได้ของบรรทัด) + Dr ภาษีขาย / Cr ลูกหนี้; รับคืนสินค้า stock เข้าคลังที่ต้นทุนเดิม (perpetual: Dr สินค้าคงเหลือ / Cr ต้นทุนขาย)
  - ตัดยอดค้างของใบแจ้งหนี้เหมือนรับเงิน (ไม่เกินยอดค้าง) — aging / ใบค้างรับ / วงเงินเครดิต / dashboard หักให้อัตโนมัติ; ใบแจ้งหนี้ที่มีใบลดหนี้ยกเลิกไม่ได้
  - detail มี `original_totals`, `credited_before`, `correct_totals` (มูลค่าเดิม / ที่ถูกต้อง) ใช้พิมพ์ใบลดหนี้; ใบแจ้งหนี้มี `credited_amount` และต่อบรรทัด `line_no`, `returned_quantity`, `credited_amount`; `received_amount` = รับเงินจริงเท่านั้น
  - รายงาน `output-vat`: ใบลดหนี้เป็นแถวติดลบ (`kind: credit_note`, `reference_no` = ใบกำกับเดิม)

### ค้นหาผู้จดทะเบียน VAT (กรมสรรพากร — module `taxregistry`)

| Method | Path | สิทธิ์ | หมายเหตุ |
|---|---|---|---|
| GET | `/companies/{companyID}/vat-registrants?tax_id=&branch_no=` | `sales.customer:manage` หรือ `purchasing.supplier:manage` | ค้นจากระบบผู้ประกอบการจด VAT ของกรมสรรพากร (ไม่บันทึกอะไร) — ตอบ `[{tax_id, branch_no, branch_name, name, tax_type, phone, vat_registered_on, address, address_detail}]` สำนักงานใหญ่ก่อน; ไม่พบ = 404 `vat_registrant_not_found`; ระบบสรรพากรติดต่อไม่ได้ / ช้าเกิน 10 วินาที = 503 `vat_registry_unavailable` |

- ใบเสนอราคา `PATCH` รับ `sent` / `accepted` / `rejected`; ใบสั่งขาย `PATCH` รับ `approved` / `closed` / `cancelled` (approved ตอบ `warnings`)
- เอกสารขายไม่มี draft — `POST` = ลงบัญชีทันที; `PATCH` รับเฉพาะ `{"status":"cancelled"}`; ไม่มีใบส่งของแยก (`/deliveries` และสิทธิ์ `sales.delivery:*` ถูกลบ — บทบาทเดิมถูกถอดสิทธิ์ตอน migrate)
- **จุดเกิดภาษี / ประเภทธุรกิจ** (ตัดสินใจ 2026-11-26): ขายสินค้า = ภาษีขายตามวันที่ใบ, ขายบริการ = ภาษีขายยังไม่ถึงกำหนด (`undue_output_vat_account_id`) จนรับเงินที่ finance / ออกใบกำกับก่อนรับเงิน; ซื้อขาย / ผลิต ออก IV, บริการ / ก่อสร้าง ออก SV — อีกชนิด = 422 `document_not_available_for_business_type` (อ่าน / ยกเลิกได้); เงินเข้าทั้งหมดรับที่ finance (`POST /receipts`)
- ค่าตั้งขายเพิ่ม `undue_output_vat_account_id` (2102020900) / `deposit_account_id` (2102021000) — ตั้งให้อัตโนมัติตอนสร้างบริษัททุกประเภท; เอกสารที่ต้องใช้แต่ไม่ได้ตั้ง = 422 `sales_settings_are_incomplete`
- response เอกสารขาย: `kind` (invoice / service_invoice / deposit), `order {id, order_no}`, `bill_to`, `lines[].item.type` (service / stock / non_stock), `lines[].service`, `totals`, `deposit_deducted`, `received_amount`, `credited_amount`, `outstanding`, `vat_status` (output / undue / partial), `vat_recognized`, `vat_undue`, `tax_invoice {no, date} | null`, `deposits_applied [{deposit {id, document_no}, amount, vat}]`, `vat_recognitions [{kind: receipt|tax_invoice, document {id, document_no}, date, base, vat}]`; มัดจำเพิ่ม `deposit_remaining`, `applied_amount`
- ใบลดหนี้มี `vat_kind` (output / undue) และ `invoice.kind`; ใบลดหนี้ของ SV ที่ภาษียังไม่ถึงกำหนดลด Dr ภาษีขายยังไม่ถึงกำหนด (ไม่อยู่ใน `output-vat`)

```jsonc
// POST /sales-orders จากใบเสนอราคา (ไม่ส่ง customer_id / lines)
{ "quotation_id": "…", "date": "2026-03-05" }
// POST /sales-invoices (ขายสินค้า) จากใบสั่งขาย — ตัดสต็อกเสมอ (warehouse_id บังคับเมื่อมีสินค้า stock)
{ "order_id": "…", "date": "2026-03-07", "warehouse_id": "…",
  "lines": [ { "order_line_no": 1, "quantity": "5" }, { "order_line_no": 2, "quantity": "1" } ] }
// POST /service-invoices (ขายบริการ) ออกตรง — vat_rate ไม่ส่ง = ตามค่าตั้ง; deposits = มัดจำที่หัก (ฐานก่อน VAT, ต้องรับเงินครบแล้ว)
{ "customer_id": "…", "customer_branch_no": "", "date": "2026-03-22", "price_includes_vat": false,
  "lines": [ { "item_id": "<บริการ>", "description": "", "quantity": "1", "unit_price": "1000" } ],
  "deposits": [ { "deposit_id": "…", "amount": "300" } ] }
// POST /service-invoices/{id}/tax-invoice — ใบกำกับภาษีก่อนรับเงิน (TX): ภาษีที่เหลือทั้งหมดถึงกำหนด; ซ้ำ = 422
{ "date": "2026-03-16" }
// POST /sales-deposits — amount ตาม price_includes_vat; order_id (ไม่บังคับ) = ลูกค้า / VAT ตามใบสั่งขาย
{ "customer_id": "…", "customer_branch_no": "", "order_id": null, "date": "2026-03-20", "description": "มัดจำงาน", "amount": "300", "price_includes_vat": false }
// GET /sales-deposits/open?customer_id= — มัดจำที่รับเงินครบแล้วและยังหักได้
[ { "id": "…", "document_no": "DP2026-000001", "date": "2026-03-20", "vat_rate": "7", "base": "300", "vat": "21", "remaining_amount": "300" } ]
// POST /billing-notes — ใบที่วางบิลไม่ได้ (ลูกค้าอื่น / ไม่ค้าง / อยู่ในใบวางบิลอื่นที่เปิด) = 422 invoice_cannot_be_put_on_this_billing_note
{ "customer_id": "…", "date": "2026-03-25", "due_date": "2026-04-05", "description": "", "invoice_ids": ["…", "…"] }
// response: { id, document_no, date, due_date, customer, status: open|paid|cancelled, total, outstanding, description,
//             invoices: [ { id, document_no, kind, date, due_date, gross, outstanding } ] }
```

| `book` (`sales.report:read`) | parameter | ผลลัพธ์ |
|---|---|---|
| `outstanding-sales-orders` | `customer_id, item_id, q, sort, page, page_size` | บรรทัดค้างออกเอกสารขาย (`to_invoice`; `to_deliver` เท่ากัน — ไม่มีใบส่งของแยก) |
| `ar-aging` | `end_date` (บังคับ), `customer_id, q, page, page_size` | ยอดค้างต่อลูกค้า (หักยอดรับถึงวันที่) แยกช่วงอายุ + `totals` |
| `output-vat` | `start_date, end_date` (บังคับ), `customer_id, sort, page, page_size` | เฉพาะเอกสารที่ภาษีเกิดแล้ว — `kind`: `invoice` / `deposit` (ตามวันที่ใบ, หักภาษีมัดจำที่หัก), `tax_invoice` (`document_id` = SV), `receipt` (`document_id` = ใบรับเงิน), `credit_note` (vat_kind output, ติดลบ); `reference_no` / `reference_id` = เอกสารต้นทาง; ฐาน + ภาษี + ยอดรวม |
| `undue-output-vat` | `end_date` (บังคับ), `customer_id, page, page_size` | SV ที่ภาษียังไม่ถึงกำหนด ณ วันที่: `rows [{invoice, date, customer, gross, vat_total, vat_recognized, vat_undue}]`, `totals` |

หมวดสินค้า / สินค้า รับ `revenue_account_id` เพิ่ม

## 4.10 Endpoint — finance (การเงิน)

เงินเข้า-ออกทั้งหมดทำผ่านการเงิน (ตัดสินใจ 2026-11-23) — ใบเสร็จรับเงินย้ายมาจาก sales, ใบจ่ายชำระ / ประเภทภาษีหัก ณ ที่จ่าย / ภ.ง.ด. ย้ายมาจาก purchasing, ทะเบียนเงินจาก cashbank

| Resource | Path | Filter เพิ่ม | สิทธิ์ |
|---|---|---|---|
| ค่าตั้ง | `GET`, `PUT /finance-settings` | — | `finance.settings:read\|manage` |
| ทะเบียนเงินสด / ธนาคาร | `/cash-accounts[/{cashAccountID}]` | `type` (cash,bank), `status` | `finance.cash_account:read\|manage` |
| ประเภทภาษีหัก ณ ที่จ่าย | `/withholding-tax-types[/{whtTypeID}]` | `status` | `finance.settings:read\|manage` |
| ประเภทภาษีสำหรับหน้าจ่ายเงิน | `GET /payments/wht-types` | (เฉพาะ active) | `finance.payment:read` |
| รับเงิน | `GET`, `POST /receipts`; `GET`, `PATCH /receipts/{receiptID}` | `kind` (customer,other), `status, party_id, cash_account_id, start_date, end_date`; sort `date\|document_no` | `finance.receipt:read\|post` |
| เอกสารขายค้างรับ | `GET /receipts/open-invoices?customer_id=&billing_note_id=` | `billing_note_id` = เฉพาะใบในใบวางบิล | `finance.receipt:read` |
| จ่ายเงิน | `GET`, `POST /payments`; `GET`, `PATCH /payments/{paymentID}` | `kind` (supplier,other) + เหมือนรับเงิน | `finance.payment:read\|post` |
| ใบแจ้งหนี้ซื้อค้างจ่าย | `GET /payments/open-invoices?supplier_id=` | — | `finance.payment:read` |
| โอนเงินระหว่างบัญชี | `GET`, `POST /cash-transfers`; `GET`, `PATCH /cash-transfers/{transferID}` | `status, cash_account_id, start_date, end_date` | `finance.transfer:read\|post` |

- เอกสารไม่มี draft — `POST` = ลงบัญชีทันที; `PATCH {"status":"cancelled"}` = กลับรายการ ณ วันที่เดิม (งวดต้องเปิด) และคืนยอดค้างให้ใบแจ้งหนี้
- เลขที่: `finance.receipt` (RV), `finance.payment` (PV), `finance.transfer` (TF) — ตั้งรูปแบบได้ที่ `document-numberings`
- สมุดรายวัน: `receipt_journal_id` / `payment_journal_id` / `transfer_journal_id` ของค่าตั้งการเงิน (ไม่ได้ตั้ง = 422 `finance_settings_are_incomplete`)
- บัญชีลูกหนี้ / เจ้าหนี้มาจากค่าตั้งขาย / จัดซื้อ (เจ้าของบัญชีคุม) — การเงินขอผ่าน `sales.SettleReceivables` / `purchasing.SettlePayables` ใน transaction เดียวกัน; กฎของขาย / จัดซื้อ (เกินยอดค้าง ฯลฯ) ตอบ 422 `rejected`
- **รับเงินขายบริการ (ม.78/1)**: sales คืนภาษีที่ถึงกำหนดตามสัดส่วนที่รับ (ยอดตัดหนี้รวมภาษีถูกหัก / ยอดค้างก่อนรับ; รับครบ = ส่วนที่เหลือ) — ใบรับเงินลง Dr ภาษีขายยังไม่ถึงกำหนด / Cr ภาษีขายในรายการเดียวกัน; response ใบรับเงินมี `tax_invoice` (พิมพ์ "ใบเสร็จรับเงิน / ใบกำกับภาษี"), `vat_lines [{invoice, base, vat}]`, `lines[].invoice_kind / vat_base / vat`; open-invoices มี `kind`, `vat_undue`, `tax_invoice`; `POST /receipts` รับ `billing_note_id` (อ้างอิงเท่านั้น ไม่บันทึก)
- คู่ค้า ณ วันเอกสาร (ชื่อ เลขผู้เสียภาษี สาขา ที่อยู่ ประเภทผู้มีเงินได้) เก็บติดเอกสาร — รายงานภาษีไม่ต้องอ่านทะเบียนข้าม module
- จ่ายเงินที่หักภาษี ณ ที่จ่ายต้องรู้เลขผู้เสียภาษีและประเภท (`individual` = ภ.ง.ด.3, `juristic` = ภ.ง.ด.53) — ผู้ขายใช้ข้อมูลทะเบียน, จ่ายเงินอื่นกรอกใน `payee`
- **หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ)**: ใบจ่ายที่มีภาษีหักได้ `wht_certificate_no` (`finance.wht_certificate`, WT) ใน transaction เดียวกัน; ประเภทภาษีหัก ณ ที่จ่ายมี `income_type` = ช่องในแบบ (`40_1, 40_2, 40_3, 40_4a, 40_4b, 3_tres, other`; ของเดิม = `3_tres`); หน้าเว็บพิมพ์ 2 ฉบับจากข้อมูลใบจ่าย

```jsonc
// PUT /finance-settings
{ "receipt_journal_id": "<CRJ>", "payment_journal_id": "<CDJ>", "transfer_journal_id": "<GEN>",
  "wht_received_account_id": "…", "wht_pnd3_account_id": "…", "wht_pnd53_account_id": "…" }
// POST /receipts — รับชำระหนี้: wht = ภาษีที่ลูกค้าหัก (ไม่ส่ง = 0); ยอดรวม methods = ยอดรวม lines - wht
{ "kind": "customer", "customer_id": "…", "date": "2026-03-20",
  "lines": [ { "invoice_id": "…", "amount": "321", "wht": "9" } ],
  "methods": [ { "cash_account_id": "…", "amount": "312", "reference": "" } ] }
// POST /receipts — รับเงินอื่น
{ "kind": "other", "date": "2026-04-01", "payer": { "name": "ธนาคาร" },
  "lines": [ { "account_id": "<ดอกเบี้ยรับ>", "description": "ดอกเบี้ย", "amount": "12.50" } ], "methods": [ … ] }
// POST /payments — จ่ายชำระหนี้: ภาษีคำนวณจากยอดก่อน VAT ตามสัดส่วนที่จ่าย, อัตรา ณ วันจ่าย
{ "kind": "supplier", "supplier_id": "…", "date": "2026-03-10",
  "lines": [ { "invoice_id": "…", "amount": "5350", "wht_type_id": "…" }, { "invoice_id": "…", "amount": "2675" } ],
  "methods": [ { "cash_account_id": "<เงินสด>", "amount": "2000" }, { "cash_account_id": "<ธนาคาร>", "amount": "5875", "reference": "CHQ-001" } ] }
// POST /payments — จ่ายเงินอื่น หัก ณ ที่จ่าย (ฐาน = ยอดจ่าย)
{ "kind": "other", "date": "2026-03-12", "payee": { "name": "นาย ก", "tax_id": "1101700000000", "tax_type": "individual" },
  "lines": [ { "account_id": "<ค่าบริการ>", "amount": "2500", "wht_type_id": "…" } ], "methods": [ { "cash_account_id": "…", "amount": "2425" } ] }
// POST /cash-transfers
{ "date": "2026-04-02", "from_cash_account_id": "<เงินสด>", "to_cash_account_id": "<ธนาคาร>", "amount": "500", "reference": "DEP-1" }
```

| `book` (`finance.report:read`) | parameter | ผลลัพธ์ |
|---|---|---|
| `withholding-tax` | `start_date, end_date` (บังคับ), `form` (pnd3 / pnd53), `party_id, sort` (date, document_no, party_name), `page, page_size` | บรรทัดใบจ่ายที่หักภาษี: ผู้รับเงิน (ชื่อ เลขผู้เสียภาษี สาขา ที่อยู่), `tax_form`, ประเภท, อัตรา, ฐาน, ภาษี; `total_base`, `total_wht` |
| `withholding-tax-received` | `start_date, end_date` (บังคับ), `party_id, sort, page, page_size` | ใบรับที่ลูกค้าหัก ณ ที่จ่าย: ฐาน (ยอดตัดหนี้), ภาษีถูกหัก + ยอดรวม |
| `vat-summary` | `start_date, end_date` (บังคับ) | สรุป ภ.พ.30: `output` / `input` = `{base, tax, documents}` (ใบกำกับหักใบลดหนี้ — ขอผ่าน `sales.OutputVat` / `purchasing.InputVat`), `net_tax`, `payable`, `refundable` |

### สิทธิ์เดิม

ตอนเริ่มโปรแกรม `identity.RenamePermissions` แปลงสิทธิ์ของบทบาทเดิม (ทำซ้ำได้): `cashbank.account:*` → `finance.cash_account:*`, `sales.receipt:*` → `finance.receipt:*`,
`purchasing.payment:*` → `finance.payment:*`; ผู้มี `purchasing.settings:*` / `purchasing.report:read` / `sales.report:read` ได้ `finance.settings:*` / `finance.report:read` เพิ่ม

## 4.11 Endpoint — manufacturing (ผลิต)

ใช้ได้เฉพาะบริษัทประเภท `manufacturing` — ประเภทอื่นตอบ 422 `the_manufacturing_app_is_available_only_to_manufacturing_companies`

| Resource | Path | Filter เพิ่ม | สิทธิ์ |
|---|---|---|---|
| ค่าตั้ง | `GET`, `PUT /manufacturing-settings` | — | `manufacturing.settings:read\|manage` |
| สูตรการผลิต | `GET`, `POST /boms`; `GET`, `PUT`, `DELETE /boms/{bomID}` | `q, item_id, status`; sort `code\|name\|updated_at` | `manufacturing.bom:read\|manage` |
| ใบสั่งผลิต | `GET`, `POST /production-orders`; `GET`, `PUT /production-orders/{orderID}` | `q, status, item_id, from, to`; sort `date\|document_no\|created_at` (ค่าเริ่มต้น `-date,-document_no`) | `manufacturing.order:read\|manage` |
| การทำงานของใบสั่งผลิต | `POST /production-orders/{orderID}/release`, `/issue`, `/return`, `/complete`, `/cancel` | — | `manufacturing.order:manage` |

```jsonc
// POST /boms, PUT /boms/{id} — แทนที่ทั้งก้อน; scrap_percent ไม่ส่ง = 0; account_id ไม่ส่ง = เครดิตบัญชีปันเข้างานในค่าตั้ง
{ "code": "FG-1", "name": "สูตรโต๊ะ", "item_id": "…", "output_quantity": "2", "status": "active", "is_default": true,
  "components": [ { "item_id": "…", "quantity": "4", "scrap_percent": "10" } ],
  "conversion_costs": [ { "description": "ค่าแรง", "account_id": null, "amount": "50" } ] }
// POST /production-orders — เลขที่ออกทันที (ชนิด manufacturing.production_order ค่าเริ่มต้น MO{fy}-{seq:6}, ตั้งรูปแบบได้ที่ /document-numberings)
{ "date": "2026-03-01", "item_id": "…", "bom_id": "…", "quantity": "4", "warehouse_id": "…",
  "planned_start": "2026-03-02", "planned_end": "2026-03-10", "description": "" }
// PUT /production-orders/{id} — draft เท่านั้น; body เดียวกัน + materials (ไม่ส่ง = คำนวณจากสูตรใหม่)
{ "...": "...", "materials": [ { "item_id": "…", "required_quantity": "9" } ] }
// POST /production-orders/{id}/issue | /return
{ "date": "2026-03-02", "lines": [ { "item_id": "…", "quantity": "12" } ] }
// POST /production-orders/{id}/complete
{ "date": "2026-03-05", "completed_quantity": "3" }
// POST /production-orders/{id}/cancel
{ "reason": "ผลิตผิดสเปก" }
```

- สูตร: `{id, code, name, item{id,code,name,uom}, output_quantity, status, is_default, components[{line_no, item, quantity, scrap_percent}], conversion_costs[{line_no, description, account{id,code,name}|null, amount}], created_at, updated_at}` (รายการไม่มี components / conversion_costs); รหัสซ้ำ / สูตรหลักที่ใช้งานอยู่ซ้ำของสินค้าเดียวกัน = 409; ลบสูตรที่มีใบสั่งผลิตอ้าง = 409
- ใบสั่งผลิต: `{id, document_no, date, item, bom{id,code,name}, quantity, warehouse{id,code,name}, planned_start, planned_end, status, description, materials[{line_no, item, required_quantity, issued_quantity, returned_quantity, net_quantity, cost}], conversion_cost, movements[{id, kind: issue|return|receipt, date, document_id, document_no, value, entry_no, cancelled}], costs{materials, conversion, total, unit_cost|null}, completed_quantity, completed_at, cancelled_at, cancel_reason, created_by, created_at}` — รายการมี `total_cost` แทน materials / movements / costs
  - `conversion_cost` = ค่าแรง / ค่าใช้จ่ายมาตรฐานตามจำนวนสั่ง; `costs.conversion` = ที่ปันเข้างานแล้ว (ตอนปิดงาน ตามจำนวนที่ผลิตได้); `unit_cost` = รวม / จำนวนที่ผลิตได้ (6 ตำแหน่ง)
  - `movements[].document_no` = เลขที่เอกสารสต็อกของ inventory (เบิก GI / รับเข้า GR); `entry_no` ว่าง = ปีบัญชีระบบสิ้นงวด (ไม่ลงบัญชี)
- ระบบบัญชีตามปีบัญชีของ inventory — **periodic (ค่าเริ่มต้นทุกประเภทธุรกิจ)**: สต็อก / ต้นทุน FIFO / ต้นทุนจริงของสินค้าสำเร็จรูปครบ ไม่มีรายการบัญชี; **perpetual**: เบิก Dr งานระหว่างผลิต / Cr สินค้าคงเหลือ, คืนกลับด้าน, ปิดงาน Dr สินค้าสำเร็จรูป / Cr งานระหว่างผลิต + Cr บัญชีปันเข้างาน (ดู 01-requirements MFG-08)
- 422: สถานะไม่ถูกต้อง (`production_order_is_not_a_draft` / `..._not_released`), วัตถุดิบไม่อยู่ในใบ, คืนเกินที่เบิกสุทธิ, วันที่ก่อนวันที่ใบสั่งผลิต, สต็อกไม่พอ / งวดปิด / สินค้าสำเร็จรูปถูกใช้แล้วตอนยกเลิก (`rejected_by_inventory…`), ค่าตั้งไม่ครบ (`manufacturing_settings_are_incomplete: wip_account_id`), สินค้าไม่มีบัญชีสินค้าคงเหลือ (perpetual)
- ค่าตั้ง: `GET` คืน `{journal_id, wip_account_id, conversion_account_id, default_warehouse_id, journal{id,code,name}, wip_account, conversion_account, default_warehouse}`; `PUT` ส่ง 4 id (ไม่ส่ง = ไม่ตั้ง) — บริษัทผลิตใหม่ได้ GEN / 1105050000 / 5065000000 ให้อัตโนมัติ

### รายงาน (`manufacturing.report:read`)

| `book` | parameter | ผลลัพธ์ |
|---|---|---|
| `production-cost` | `start_date, end_date` (บังคับ, วันที่ปิดงาน), `item_id, page, page_size` | ใบสั่งผลิตที่ปิดงาน: `order{id,document_no}, date, item, quantity, materials, conversion, total, unit_cost` + `totals{materials, conversion, total}` ทั้งช่วง |
| `material-usage` | เหมือน `production-cost` | ใบสั่งผลิต × วัตถุดิบ: `standard_quantity` (ปริมาณที่ต้องใช้ × จำนวนที่ผลิตได้ / จำนวนสั่ง), `actual_quantity` (เบิกสุทธิ), `variance_quantity`, `actual_cost`, `variance_value` (ผลต่าง × ต้นทุนเฉลี่ยที่เบิก) |
| `wip` | `end_date` (บังคับ), `page, page_size` | ใบที่ปล่อยผลิตแล้วแต่ยังไม่ปิดงาน ณ วันที่: `order, date, item, quantity, materials` (เบิก − คืนถึงวันที่) + `total` |

## 4.12 Endpoint — petty cash (เงินสดย่อย, ระบบ imprest)

เงินสดย่อยบันทึกเอกสารเท่านั้น — เงินออกจากบริษัทจริงเมื่อ **การเงิน** จ่ายคำขอ (`POST /payments` kind `petty_cash`) และการเงินเป็นผู้ลงบัญชี

| Method | Path | สิทธิ์ | หมายเหตุ |
|---|---|---|---|
| GET / PUT | `/petty-cash-settings` | `petty_cash.fund:read` / `manage` | `input_vat_account_id` (ตั้งให้อัตโนมัติ = 1107090000 ตอนสร้างบริษัท) |
| GET / POST | `/petty-cash-funds` | `petty_cash.fund:read` / `manage` | `?q=&status=&sort=code\|name`; body `code, name, custodian_user_id` (ต้องเป็นสมาชิกบริษัท), `gl_account_id` (ไม่ส่ง = 1101010200), `imprest_amount, status` |
| GET / PUT | `/petty-cash-funds/{fundID}` | เหมือนบน | response มี `funded, cash_on_hand, pending_replenishment, requested`; แก้วงเงินได้จนกว่าจะตั้งวงเงิน (หลังจากนั้นใช้คำขอ `increase`) |
| GET / POST | `/petty-cash-vouchers` | `petty_cash.voucher:read` / `manage` | ใบสำคัญจ่าย (PCV) `?q=&fund_id=&status=&from=&to=&unrequested=true`; body `fund_id, date, payee{name,tax_id,branch_no}, description, lines[{account_id,description,amount}], vat_rate, price_includes_vat, tax_invoice{no,date}` — มี VAT ต้องมีใบกำกับภาษี |
| GET / PUT | `/petty-cash-vouchers/{voucherID}` | เหมือนบน | แก้ได้เฉพาะร่าง |
| POST | `/petty-cash-vouchers/{voucherID}/approve` | `petty_cash.voucher:approve` | กองทุนต้องตั้งวงเงินแล้วและเงินคงเหลือพอ |
| POST | `/petty-cash-vouchers/{voucherID}/cancel` | `petty_cash.voucher:manage` | ร่าง หรืออนุมัติแล้วที่ยังไม่อยู่ในคำขอ |
| GET / POST | `/petty-cash-replenishments` | `petty_cash.replenishment:read` / `post` | คำขอ (PCR) `kind`: `establish` (ยอด = วงเงิน), `increase` (+`amount`), `replenish` (+`voucher_ids`, ยอด = ผลรวมใบสำคัญจ่าย) |
| GET | `/petty-cash-replenishments/{requestID}` | `read` | มี `vouchers[]` และ `payment{document_id,document_no,date}` เมื่อการเงินจ่ายแล้ว |
| POST | `/petty-cash-replenishments/{requestID}/cancel` | `post` | เฉพาะที่ยังไม่จ่าย — ปลดใบสำคัญจ่ายกลับเป็นอนุมัติแล้ว |
| GET | `/payments/petty-cash-requests` | `finance.payment:manage` | (finance) คำขอที่รอจ่าย สำหรับฟอร์มจ่ายเงิน |
| POST | `/payments` kind `petty_cash` | `finance.payment:manage` | body `petty_cash_request_id, date, methods[]` — บรรทัดบัญชีมาจากคำขอในทรานแซกชันเดียวกัน: ตั้ง / เพิ่มวงเงิน = Dr บัญชีเงินสดย่อย; เบิกชดเชย = Dr ค่าใช้จ่ายรวมตามบัญชี (ก่อน VAT) + Dr ภาษีซื้อ; วันที่จ่ายต้องไม่ก่อนวันที่คำขอ; ยกเลิกใบจ่าย = คำขอกลับเป็นรอจ่าย |

### รายงาน (`petty_cash.report:read`) — ผ่าน `GET /reports?book=`

| `book` | parameter | ผลลัพธ์ |
|---|---|---|
| `petty-cash-voucher-register` | `start_date, end_date` (บังคับ), `fund_id, status, page, page_size` | `rows` = แถวใบสำคัญจ่าย + `totals{net,vat,gross}` |
| `petty-cash-fund-balance` | `end_date` (ไม่ส่ง = วันนี้) | ต่อกองทุน: `fund, custodian, imprest_amount, funded, cash_on_hand, pending_replenishment, requested` |
| `petty-cash-input-vat` | `start_date, end_date` (บังคับ), `page, page_size` | ภาษีซื้อจากใบสำคัญจ่ายที่ชดเชยแล้ว (ตามวันที่การเงินจ่าย) — ยอดนี้รวมใน `vat-summary` (ภ.พ.30) ด้วย |

## Dashboard (ภาพรวมของแต่ละแอป)

`GET /companies/{companyID}/dashboards/{app}?start_date=&end_date=` — ต้องส่งทั้งสองวัน (ผิด = 400 `invalid_query_parameter`), อ่านอย่างเดียวไม่ใช้ transaction, แต่ละ module ใช้ aggregate query ไม่เกิน ~5 ครั้ง

| app | สิทธิ์ | เนื้อหา |
|---|---|---|
| `sales` | `sales.report:read` | ยอดขายก่อน VAT ช่วงนี้ / ช่วงก่อน, จำนวนใบกำกับภาษี, รายเดือนตามปีบัญชีที่ครอบคลุม end_date (`fiscal_year {year, start_date, end_date}`) + `previous_year` ปีบัญชีก่อนหน้าเดือนต่อเดือน (null เมื่อไม่มียอด; ไม่มีปีบัญชี = 12 เดือนล่าสุด และ `fiscal_year` = null), ใบเสนอราคารอตอบรับ, ใบสั่งขายร่าง / ค้างส่ง / ค้างออกบิล, ลูกหนี้ค้างรับ + เกินกำหนด + อายุหนี้ ณ end_date, ลูกค้า top 5 |
| `purchasing` | `purchasing.report:read` | ยอดซื้อ (ตาม posting_date), 12 เดือน, ใบสั่งซื้อร่าง / ค้างรับ / ค้างรับใบแจ้งหนี้, เจ้าหนี้ค้างจ่าย + เกินกำหนด + ครบกำหนดใน 7 วัน + อายุหนี้, ผู้ขาย top 5 |
| `inventory` | `inventory.report:read` | มูลค่าคงเหลือ ณ end_date (รวม / ตามคลัง), จำนวนสินค้านับสต็อก, ต่ำกว่าจุดสั่งซื้อ (10 อันดับ + จำนวนทั้งหมด), เอกสารรับ / เบิกในช่วง, มูลค่ารับเข้า / จ่ายออกรายเดือน (ไม่รวมโอนคลัง) |
| `finance` | `finance.report:read` | ยอดบัญชีเงินสด / ธนาคารตามบัญชีแยกประเภท ณ end_date (รวมยอดยกมาจากสมุดรายวัน), เงินรับ / จ่ายจริง, กระแสสุทธิ, จำนวนโอน, 12 เดือน (ไม่รวมโอน), ภาษีหัก ณ ที่จ่าย (หักไว้ / ถูกหัก), รายการล่าสุด 6 รายการ |
| `accounting` | `ledger.report:read` | รายได้ / ค่าใช้จ่าย / กำไรสุทธิช่วงนี้และช่วงก่อน (ไม่รวมบัญชีพัก), 12 เดือน, ยอดบัญชีพัก ณ end_date, จำนวนรายการบัญชี, รายการล่าสุด 5 รายการ, งวดบัญชีที่ครอบคลุม end_date |

- "ช่วงก่อน" = จำนวนวันเท่ากันก่อน start_date; รายเดือน = 12 เดือนถึงเดือนของ end_date (เติม 0 ครบ 12 เดือน เรียงเก่า → ใหม่); จำนวนเงินเป็น string
