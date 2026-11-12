# 2. Architecture

## 2.1 รูปแบบโดยรวม

- **Modular monolith** — โปรแกรมเดียว (`cmd/api`), แบ่งเป็น module ตาม bounded context
- **Hexagonal architecture** ภายในแต่ละ module (ตาม [CLAUDE.md](../CLAUDE.md) §2)
- Go 1.26 + `net/http` ServeMux (pattern แบบ `"GET /path/{id}"`) — ไม่ใช้ web framework
- GORM + PostgreSQL (driver pgx)

```text
cmd/api/main.go
   │  โหลด config → เปิด DB → Migrate ทุก module → สร้าง module → ผูก route
   ▼
┌──────────────────────────── internal/modules ─────────────────────────────┐
│  identity              organization                   ledger              │
│  (users, roles,  ◄──── (companies = tenant, branch, ◄── (accounts,        │
│   memberships,  facade  BU, dept, cost center,  facade journals,         │
│   JWT)          ◄────── project site, fiscal year)      entries, reports) │
│                 └──────────────── facade ──────────────────┘              │
└────────────────────────────────────┬──────────────────────────────────────┘
                               ▼
            internal/platform (config, database, httpx, listing)
```

## 2.2 Layer ภายใน module

```text
internal/modules/{module}/
├── domain/                  # กฎธุรกิจล้วน — ไม่ import GORM / HTTP
│   ├── entities/            # aggregate + invariant
│   ├── value_objects/       # ID แบบมี type, Code, Email, Status, Permission ...
│   ├── services/            # domain service (เช่น ResolveAccess, EnsureNoCycle)
│   └── errors/              # package domainerrors (import ด้วย alias "domain")
├── application/
│   ├── ports/               # TransactionManager, Authorizer, TokenService ...
│   │   └── repositories/    # interface ของ repository (สิ่งที่ use case ต้องการ)
│   ├── dto/                 # input / output / filter ของ use case
│   └── usecases/            # workflow + ขอบเขต transaction + ตรวจสิทธิ์
├── infrastructure/
│   ├── persistence/gorm/
│   │   ├── models/          # รูปร่างตาราง (แยกจาก entity)
│   │   ├── repositories/    # implement port ด้วย GORM
│   │   ├── mapper.go        # model ⇄ entity / read model
│   │   └── transaction.go   # TransactionManager
│   └── services/            # adapter อื่น (bcrypt, JWT, event publisher, adapter ไป identity)
├── presentation/http/
│   ├── handlers/            # แปลง HTTP ⇄ use case, map error → status code
│   ├── requests/            # JSON body + URL query parser (validation ระดับ presentation)
│   ├── responses/           # response DTO
│   └── routes.go
└── module.go                # ประกอบ dependency + public API ของ module
```

ทิศทาง dependency: `presentation → application → domain`, `infrastructure → application ports`
GORM มีอยู่เฉพาะใน `infrastructure/` และ `platform/database`

## 2.3 Platform (ใช้ร่วมทุก module)

| Package | หน้าที่ |
|---|---|
| `platform/config` | อ่าน `.env` (ไม่ทับ env ที่ตั้งไว้แล้ว) และ env → `Config` |
| `platform/database` | เปิด Postgres (pool 25/5, UTC, `TranslateError`), transaction ผ่าน `ctx`, `LikeContains`, ตรวจ unique/FK violation, `OrderBy`, `FetchPage` |
| `platform/httpx` | decode JSON, envelope `{data}` / `{data, meta}` / `{error}`, `ErrorMapper` (domain error → HTTP status) |
| `platform/listing` | parse URL query: `page`, `page_size`, `sort`, `q`, list, optional, bool, date, date range |

## 2.4 Transaction

```text
Handler ─► UseCase ─► TransactionManager.WithinTransaction(ctx, fn)
                          │  BEGIN — เก็บ *gorm.DB (tx) ไว้ใน ctx
                          ▼
              fn(txCtx): ตรวจสิทธิ์ → load → domain rule → repo.Create/Update ...
                          │  ทุก repository ดึง tx จาก ctx (database.Conn / TxConn)
                          ▼
                     COMMIT  หรือ ROLLBACK เมื่อมี error
```

- เปิด transaction ที่ use case เท่านั้น — repository ไม่เปิดเอง
- key ของ transaction อยู่ใน `platform/database` ⇒ repository ต่าง module เข้าร่วม transaction เดียวกันได้
- `WithinTransaction` ซ้อนกันได้ (เข้าร่วม transaction เดิม)
- repository ที่เขียนหลายตาราง เรียก `database.TxConn(ctx)` ซึ่ง error ถ้าไม่มี transaction (กันลืม)
- งานที่ใช้ CPU นาน (bcrypt hash) ทำ **ก่อน** เปิด transaction
- GET ไม่เปิด transaction ยกเว้นจำเป็น
- การปิด/เปิดงวดบัญชีใช้ `SELECT ... FOR UPDATE` ล็อกปีบัญชี เพื่อให้ลำดับการปิดงวดถูกต้องเมื่อมีคำขอพร้อมกัน

## 2.5 Authentication & Authorization

1. `POST /auth/tokens` ออก JWT (HS256, `JWT_SECRET` ≥ 32 ตัว, อายุ `JWT_TTL`)
2. Middleware `Authenticate` ตรวจ `Authorization: Bearer <token>` แล้วใส่ user ID ใน `ctx`
3. Use case ตรวจสิทธิ์ทุกครั้ง:
   - `AccessRepository.FindAccessFacts` ดึง user / membership / role / permission ใน **คิวรี JOIN เดียว**
   - `services.ResolveAccess` (pure function) ตัดสินผล
   - ฝั่งเขียน ตรวจสิทธิ์ **ใน transaction เดียวกับการแก้ข้อมูล**

## 2.6 การคุยข้าม module

แบ่งหน้าที่:
- **organization** เป็นเจ้าของบริษัท (tenant): รหัส, ชื่อ, ข้อมูลนิติบุคคล, สถานะ
- **identity** เป็นเจ้าของ "ใครเข้าบริษัทไหนได้ด้วยสิทธิ์อะไร" โดยรู้จักแค่ `company_id`

organization ไม่ import โค้ดภายในของ identity — พึ่งพาผ่าน interface ที่ประกาศเอง (`organization.Identity`)
ซึ่ง `*identity.Module` implement:

```go
type Identity interface {
    Authenticate(next http.Handler) http.Handler
    CurrentUserID(ctx context.Context) (uuid.UUID, bool)
    Authorize(ctx context.Context, userID, companyID uuid.UUID, permission string) (bool, error)
    ProvisionCompanyOwner(ctx context.Context, companyID, ownerID uuid.UUID) (bool, error)
    MemberCompanyIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}
```

- `Authorize` / `ProvisionCompanyOwner` คืน `(false, nil)` เมื่อถูกปฏิเสธ (ไม่มีสิทธิ์ / ผู้ใช้หรือสมาชิกไม่ active) และคืน error เฉพาะเมื่อระบบผิดพลาด
- organization แปลงผลเป็น type / error ของตัวเองใน `infrastructure/services/identity.go`
- ไม่มี FK ข้าม module (ดู [03-database.md](03-database.md))

การสร้างบริษัท — transaction เดียวข้าม module:

```text
POST /companies
  organization.CompanyUseCases.Create
    BEGIN
      INSERT organization_companies
      identity.ProvisionCompanyOwner  → INSERT identity_roles (owner, "*") + identity_memberships
    COMMIT   (ขั้นใดล้มเหลว → ROLLBACK ทั้งหมด — มี integration test ยืนยัน)
```

### ledger → organization

ledger พึ่งพา organization ผ่าน interface `ledger.Organization` (`*organization.Module` implement) — organization ไม่ตรวจสิทธิ์ในส่วนนี้ (ledger ตรวจของตัวเองแล้ว):

```go
type Organization interface {
    CompanySettings(ctx, companyID uuid.UUID) (active bool, amountDecimals, quantityDecimals int, err error)
    FiscalPeriodAt(ctx, companyID uuid.UUID, date time.Time) (periodID uuid.UUID, fiscalYear int, yearStart time.Time, open bool, err error)
    MissingDimensions(ctx, companyID uuid.UUID, branches, departments, costCenters, projectSites []uuid.UUID) ([]string, error)
}
```

การผ่านรายการ — transaction เดียว:

```text
PATCH /journal-entries/{id} {"status":"posted"}
  BEGIN
    identity.Authorize(ledger.entry:post)          ← สิทธิ์
    organization.CompanySettings                    ← บริษัทต้อง active + จำนวนทศนิยมของเงิน
    SELECT entry ... FOR UPDATE                      ← กันผ่านรายการซ้อน
    journal / accounts (IN) / MissingDimensions      ← query ละ 1 ครั้ง ไม่ว่ากี่บรรทัด
    organization.FiscalPeriodAt ... FOR SHARE OF p   ← งวดต้องเปิด; การปิดงวดพร้อมกันต้องรอจน commit
    INSERT ledger_journal_sequences ... ON CONFLICT DO UPDATE ... RETURNING   ← เลขที่ถัดไป (lock แถวลำดับ)
    UPDATE entry SET status = 'posted', entry_no, ...
  COMMIT  (rollback = เลขที่ไม่ถูกใช้ จึงไม่ขาดตอน — มี test ผ่านรายการพร้อมกัน 10 ใบ)
```

### inventory → organization (และ FIFO)

inventory ใช้ public API เดียวกับ ledger (`CompanySettings`, `FiscalPeriodAt`, `MissingDimensions` สำหรับสาขาของคลัง)

การผ่านเอกสารสต็อก — transaction เดียว:

```text
BEGIN
  identity.Authorize / organization.CompanySettings   ← สิทธิ์, บริษัท active, จำนวนทศนิยม
  SELECT document ... FOR UPDATE
  คลัง (IN) / สินค้า (IN)                               ← query ละ 1 ครั้ง
  organization.FiscalPeriodAt ... FOR SHARE            ← งวดต้องเปิด
  SELECT inventory_layers WHERE (item, warehouse) IN (...) AND layer_date <= วันที่เอกสาร
         ORDER BY item, warehouse, layer_date, id FOR UPDATE      ← ลำดับ lock คงที่ กัน deadlock
  services.PlanPosting (pure function)                  ← FIFO / ห้ามติดลบ คำนวณใน domain
  UPDATE layers FROM (VALUES ...) ; INSERT layers / movements / allocations (batch)
  INSERT inventory_sequences ... ON CONFLICT DO UPDATE ... RETURNING   ← เลขที่
COMMIT
```

- FIFO และการยกเลิก (`PlanPosting`, `PlanCancel`) เป็น pure function ใน `domain/services` จึงทดสอบได้โดยไม่ต้องมีฐานข้อมูล
- `inventory_movements` คือที่มาของยอดคงเหลือ / stock card; `inventory_layers` คือชั้นต้นทุนที่ยังเหลือ; `inventory_allocations` บันทึกว่าการเคลื่อนไหวขาออกตัดชั้นไหน (ใช้คืนชั้นตอนยกเลิก)

## 2.6.1 รายงานรวม (`platform/reports`)

- endpoint เดียวของทั้งระบบ: `GET /api/v1/companies/{companyID}/reports?book=...`
- แต่ละ module ลงทะเบียน book ของตัวเอง (`RegisterReports`):
  - ledger: `trial-balance`, `balance-sheet`, `income-statement`, `general-ledger`
  - inventory: `stock-on-hand`, `stock-card`
- registry ส่งต่อตาม `book`; book ไม่รู้จัก = 400 พร้อมรายชื่อ book ทั้งหมด; ชื่อซ้ำข้าม module = panic ตอนเริ่มโปรแกรม
- `main.go` mount registry พร้อม `Authenticate`; แต่ละ handler ตรวจสิทธิ์ของ module ตัวเอง

## 2.6.2 ตัวเลข (`platform/numeric`)

- จำนวนเงิน / จำนวนสินค้าใช้ `shopspring/decimal` (ฐาน 10) — ไม่ใช้ float
- ฐานข้อมูลเก็บ `NUMERIC(24,6)` (จำนวนเต็ม 18 หลัก, ทศนิยม 6 ตำแหน่ง)
- จำนวนทศนิยมที่ยอมรับมาจากบริษัท (`amount_decimals`, `quantity_decimals`) — อ่านพร้อมสถานะบริษัทใน query เดียวตอนเขียน
- JSON ส่งเป็นข้อความทศนิยมตามค่าจริง เช่น `"1200"`, `"30000.5"` — frontend จัดรูปแบบตามทศนิยมของบริษัท

## 2.7 บริษัท inactive

- identity ไม่ตรวจสถานะบริษัท
- organization บังคับใน `base.write`: ทุกการเขียนข้อมูลของบริษัท อ่าน `status` ของบริษัทใน transaction เดียวกัน ถ้า `inactive` → `422 company is inactive (read-only)`
- การอ่านไม่ถูกกระทบ; `PUT /companies/{id}` ไม่ผ่านกฎนี้เพื่อให้เปิดใช้งานกลับได้
- ledger และ inventory บังคับกฎเดียวกันผ่าน `organization.CompanySettings` ใน transaction ของการเขียน — module ที่จะสร้างต่อไปต้องทำแบบเดียวกัน

## 2.8 Domain events

identity ประกาศ event (`UserRegistered`, `MemberAdded`, `MemberRolesChanged` ฯลฯ) ใน `domain/events` และ publish **หลัง commit**
ตอนนี้ adapter คือ `LogEventPublisher` (เขียน log อย่างเดียว) — ยังไม่มี consumer
organization ยังไม่มี event (เช่น "บริษัทถูกสร้าง")
