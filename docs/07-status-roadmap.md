# 7. Status & Roadmap

ข้อมูล ณ 2026-11-11 — มี commit เดียว (`first commit`) งานทั้งหมดด้านล่างยังไม่ได้ commit

## 7.1 สถานะตามขั้นของ SDLC

| ขั้น | สถานะ | หลักฐาน |
|---|---|---|
| Requirements | 🟡 บางส่วน — ครอบคลุม identity + organization | [01-requirements.md](01-requirements.md) |
| Design | ✅ สถาปัตยกรรม, DB, API ของสอง module | [02](02-architecture.md), [03](03-database.md), [04](04-api.md), [CLAUDE.md](../CLAUDE.md) |
| Implementation | 🟡 backend 3 module เสร็จ, frontend มีหน้า login | `apps/backend`, `apps/frontend` |
| Testing | 🟡 unit + integration ผ่าน, ยังไม่มี CI | [05-testing.md](05-testing.md) |
| Deployment | ⬜ รันในเครื่องได้, ยังไม่มี pipeline / image | [06-deployment.md](06-deployment.md) |
| Maintenance | ⬜ ยังไม่เริ่ม | — |

## 7.2 ประวัติการเปลี่ยนแปลง (ยังไม่ commit)

1. **ปรับ identity ให้ตรงสเปก CLAUDE.md**
   - ย้ายโครงสร้างให้ตรงสเปก (`application/usecases`, `application/ports/repositories`, `domain/value_objects`, `domain/errors`, `infrastructure/persistence/gorm/{mapper,transaction}.go`)
   - เปลี่ยน route ให้เป็น REST: `POST /users` (สมัคร), `POST /auth/tokens` (ล็อกอิน), `PATCH /members/{userID}` แทน route suspend / activate
   - list ทั้งหมดรองรับการค้นหา / เรียง / แบ่งหน้าผ่าน URL query
   - ตรวจสิทธิ์ด้วยคิวรี JOIN เดียว; `GET /members` ลดจาก ~9–10 คิวรีเหลือ 3
2. **เปลี่ยนชื่อ Go module** เป็น `apexis/backend`
3. **เพิ่ม platform**: `httpx` (envelope / error mapper), `listing` (URL query), `database/query.go` (`OrderBy`, `FetchPage`)
4. **สร้าง module organization** — 7 resource ตาม [01-requirements.md §1.4](01-requirements.md)
5. **ย้ายบริษัทจาก identity มา organization** (ตัดสินใจ 2026-11-11)
   - ตาราง `identity_companies` + `organization_company_profiles` รวมเป็น `organization_companies`
   - ตารางอื่นของ organization มี FK มาที่ตารางนี้
   - `POST /companies`, `GET/PUT /companies/{id}`, `GET /me/companies` ย้ายมา organization
   - `/companies/{id}/profile` ถูกยกเลิก
   - identity เหลือแค่ role / สมาชิก / สิทธิ์ และมี facade `ProvisionCompanyOwner`, `MemberCompanyIDs`
   - permission `identity.company:*` ถูกยกเลิก → ใช้ `organization.company:read|manage`
6. **งวดบัญชี 12 งวดต่อปี** (ตัดสินใจ 2026-11-11) — ไม่มีงวดที่ 13
7. **สร้าง module ledger**
   - ผังบัญชี, สมุดรายวัน, รายการบัญชี (draft → posted → reversed)
   - รายงาน 4 ชนิดผ่าน `GET /reports?book=` endpoint เดียว
   - organization เพิ่ม public API ให้ module อื่นใช้: `CompanyActive`, `FiscalPeriodAt` (lock งวด FOR SHARE), `MissingDimensions`
8. **เริ่ม frontend** (`apps/frontend`)
   - React 19 + Vite + Tailwind CSS v4 + React Router 7
   - ข้อความทั้งหมดอยู่ที่ `src/config/locales/th.ts`, design token อยู่ที่ `src/styles/app.css`
   - หน้า login + หน้าหลักชั่วคราว

## 7.3 ข้อเบี่ยงเบนจากสเปก / การตัดสินใจที่ควรรู้

| เรื่อง | สิ่งที่ทำ | เหตุผล |
|---|---|---|
| `domain/events`, `infrastructure/services` | มีโฟลเดอร์ที่ไม่อยู่ในผังของสเปก | event ของ identity และ adapter (bcrypt, JWT, identity) ไม่เข้ากับโฟลเดอร์อื่น |
| ชื่อ package `domain/errors` | `domainerrors` (import เป็น `domain`) | กันชนกับ `errors` ของ standard library |
| FK ข้าม module | `identity_roles` / `identity_memberships` เก็บ `company_id` โดยไม่มี FK ไป `organization_companies` | ให้ module แยก schema ได้; บริษัทกับ owner ถูกสร้างใน transaction เดียวกันเสมอ |
| `GET /me/companies` ใช้ 2 คิวรี | identity คืน id → organization กรอง / แบ่งหน้า | JOIN ข้าม module ไม่ได้ (สเปก §13 อนุญาตเมื่อมีเหตุผล) |
| บริษัท inactive | อ่านได้, เขียนข้อมูลของบริษัทไม่ได้ (422), แก้บริษัทเพื่อเปิดกลับได้; จัดการ role / สมาชิกยังได้ | **รอยืนยัน** — เดิมใน identity บริษัท inactive = ห้ามเข้าทุกอย่าง แต่ไม่มีทางเปิดกลับ |
| ปีบัญชีซ้อนกัน | ตรวจใน application + UNIQUE `(company_id, year)` | ถ้าต้องการกันที่ DB 100% เพิ่ม exclusion constraint (`btree_gist`) ได้ |
| ลบบริษัท / ลบปีบัญชี | ไม่มี endpoint | ปิดบริษัทด้วย status แทน; ข้อมูลบัญชีไม่ควรหาย |
| Audit | ยังไม่ทำ | สเปกระบุ "ถ้าจำเป็น" — ledger มี created_by / posted_by / posted_at แต่ยังไม่มีประวัติการแก้ draft |
| ledger: ปิดบัญชีสิ้นปี | ยังไม่มี — รายงานแยก "กำไรสะสมปีก่อนที่ยังไม่ปิดบัญชี" ให้งบสมดุลแทน | **รอยืนยัน** ว่าต้องการรายการปิดบัญชีจริงหรือไม่ |
| ledger: สกุลเงิน | สกุลเดียว (สกุลเงินหลักของบริษัท) | ยังไม่รองรับรายการสกุลเงินต่างประเทศ / อัตราแลกเปลี่ยน |
| ledger: รายงาน | endpoint เดียว `GET /reports?book=…` (ตัดสินใจ 2026-11-11) | หน้ารายงานของ frontend เป็นหน้าเดียว เลือกฟอร์มด้วย `book`; ทุกรายงานใช้ `start_date` / `end_date` ชุดเดียวกัน |
| ledger: ผังบัญชีเริ่มต้น | ยังไม่มี template ผังบัญชีไทย / สมุดเริ่มต้น (ต้องสร้างเอง) | **รอยืนยัน** ว่าต้องการ template หรือไม่ |
| ledger: ผ่านรายการ | ~8 query เล็กใน transaction เดียว (สิทธิ์, บริษัท, lock รายการ, สมุด, บัญชี, มิติ, งวด, เลขที่) | แต่ละ query ใช้ index / primary key — ไม่ขึ้นกับจำนวนบรรทัด |
| ledger: มิติวิเคราะห์ใน response | ส่งเป็น id (ไม่มีชื่อ) | ชื่ออยู่ใน organization — JOIN ข้าม module ไม่ได้; frontend ใช้ master list ที่โหลดไว้ |
| สิทธิ์ role อื่นนอกจาก owner | ต้องให้ admin กำหนด `organization.*` เอง | ยังไม่มี role template |

## 7.4 งานถัดไป (แนะนำ)

| ลำดับ | งาน |
|---|---|
| 1 | commit งานปัจจุบัน (แยก commit: refactor identity / platform / organization / ย้ายบริษัท / docs) |
| 2 | ยืนยันกฎ "บริษัท inactive = อ่านอย่างเดียว" |
| 3 | CI (vet + unit + integration กับ Postgres) และ Dockerfile |
| 4 | Versioned migration แทน AutoMigrate |
| 5 | Audit log ร่วม (platform) — ใช้ transaction เดียวกับการเขียน |
| 6 | frontend ต่อ: layout หลัก + เมนู, เลือกบริษัท (`/me/companies`), หน้ารายงานเดียว `?book=` |
| 7 | ledger ต่อ: template ผังบัญชีไทย, การปิดบัญชีสิ้นปี, ตารางยอดสะสมรายงวด (ถ้ารายงานช้าเมื่อข้อมูลโต) |
| 8 | module ถัดไป (เช่น AR / AP) ที่ลงบัญชีผ่าน ledger use case ใน transaction เดียวกัน |
