# 6. Deployment & Operations

## 6.1 ความต้องการของระบบ

- Go 1.26.x
- PostgreSQL (ทดสอบกับ 16)
- Docker (สำหรับ integration test)

## 6.2 Configuration

อ่านจาก environment ก่อน แล้วเติมจาก `.env` (หาที่ `./.env` และ `../../.env` จึงรันได้ทั้งจาก `apps/backend` และ root) — ค่าใน env จริงไม่ถูกทับ
ตัวอย่างอยู่ที่ [.env.example](../.env.example), ไฟล์ `.env` ถูก ignore จาก git

| ตัวแปร | ค่าเริ่มต้น | หมายเหตุ |
|---|---|---|
| `HTTP_ADDR` | `:8080` | |
| `DATABASE_CONNECTION` | `postgres` | รองรับเฉพาะ postgres |
| `DATABASE_HOST` / `DATABASE_PORT` | `localhost` / `5432` | |
| `DATABASE_USER` / `DATABASE_PASSWORD` | — | `USER` บังคับ |
| `DATABASE_NAME` | — | บังคับ |
| `DATABASE_SSLMODE` | `disable` | `disable` \| `require` \| `verify-full` |
| `JWT_SECRET` | — | บังคับ, อย่างน้อย 32 ตัวอักษร (`openssl rand -base64 48`) |
| `JWT_TTL` | `8h` | รูปแบบ Go duration |
| `JWT_ISSUER` | `erp-api` | |
| `VAT_REGISTRY_URL` | (endpoint ของกรมสรรพากร) | ค้นหาผู้จด VAT — ว่าง = `https://vsinter.rd.go.th/.../getVatInfo`; server ต้องออก internet ได้ (ไม่ได้ = ผู้ใช้กรอกเอง) |
| `INITIAL_ADMIN_USERNAME` | — | อีเมลของผู้ดูแลระบบที่สร้างตอนเริ่มโปรแกรม (ถ้ายังไม่มี) — ต้องตั้งคู่กับ `INITIAL_ADMIN_PASSWORD`; ไม่ตั้งทั้งคู่ = ไม่ seed |
| `INITIAL_ADMIN_PASSWORD` | — | 8–72 ตัวอักษร — ใช้ตอนสร้างครั้งแรกเท่านั้น (เริ่มโปรแกรมใหม่ไม่รีเซ็ตรหัสผ่าน) |

## 6.3 รันในเครื่อง

```bash
cp .env.example .env          # แล้วแก้ค่า DB / JWT_SECRET
cd apps/backend
go run ./cmd/api
curl http://localhost:8080/healthz   # 200
```

ลำดับการเริ่มโปรแกรม (`cmd/api/main.go`): โหลด config → เปิด DB → `identity.Migrate` → `organization.Migrate`
→ สร้าง identity module → ผูก route ของ identity และ organization → `ListenAndServe`
ปิดโปรแกรมด้วย SIGINT / SIGTERM แบบ graceful (รอ request ค้างได้ 10 วินาที)

HTTP server timeout: read header 10s, read 30s, write 30s, idle 120s
DB pool: max open 25, max idle 5, conn lifetime 30 นาที; log คิวรีที่ช้ากว่า 200ms

Frontend: Next.js (ดู [08-frontend.md §8.6](08-frontend.md)) — server ของ Next ส่งต่อ `/api/v1/*` ไป backend เอง
- dev: `npm run dev` (http://localhost:5173)
- production: `npm run build` ได้ `.next/standalone/` → รัน `node server.js`
  - คัดลอก `public/` ไปที่ `.next/standalone/public` และ `.next/static/` ไปที่ `.next/standalone/.next/static`
  - env: `APEXIS_API_URL` (ที่อยู่ backend จากมุมมองของ server Next — ไม่ต้องเปิด backend สู่ภายนอก), `PORT`, `HOSTNAME`
  - ต้องใช้ HTTPS — cookie `apexis_session` ตั้ง `Secure` เมื่อ `NODE_ENV=production`

> ถ้าเครื่องมีโปรแกรมอื่นใช้พอร์ต 8080 อยู่ (เช่น Docker / WSL) ให้ตั้ง `HTTP_ADDR=:18080` ที่ backend
> และ `APEXIS_API_URL=http://127.0.0.1:18080` ใน `apps/frontend/.env.local`

## 6.4 Migration

ตอนนี้ใช้ GORM `AutoMigrate` ทุกครั้งที่เริ่มโปรแกรม — เพิ่มตาราง / คอลัมน์ / index ได้ แต่ **ไม่ลบหรือเปลี่ยนชนิดคอลัมน์** และไม่มีประวัติ version

ก่อนขึ้น production ควรเปลี่ยนเป็น versioned migration (เช่น goose หรือ golang-migrate) และแยกขั้นตอน migrate ออกจากการเริ่ม server

> **ฐานข้อมูล dev ที่สร้างก่อนย้ายบริษัทมา organization (2026-11-11)** ยังมีตาราง `identity_companies`,
> `organization_company_profiles` และ FK จาก `identity_roles` / `identity_memberships` ไป `identity_companies`
> ซึ่ง AutoMigrate ไม่ลบให้ — FK เดิมจะทำให้สร้างบริษัทใหม่ไม่ได้ ให้ลบฐานข้อมูล dev แล้วสร้างใหม่
> (ยังไม่มีข้อมูล production จึงไม่ได้เขียน data migration)

## 6.5 Logging

- log ของแอปเป็น JSON (`log/slog`) ออก stdout
- GORM log ระดับ Warn (คิวรีช้า / error)
- domain event ของ identity ถูกเขียนลง log

## 6.6 Checklist ก่อนขึ้น production (ยังไม่ได้ทำ)

- [ ] Dockerfile / image สำหรับ backend
- [ ] CI: `go vet`, `go test`, integration test กับ Postgres
- [ ] Versioned migration
- [ ] `DATABASE_SSLMODE=verify-full`, เก็บ `JWT_SECRET` ใน secret manager
- [ ] CORS (เมื่อมี frontend), rate limit ที่ `/auth/tokens`
- [ ] Audit log ของการแก้ข้อมูล
- [ ] Monitoring / metrics / tracing
- [ ] Backup & restore ของฐานข้อมูล
