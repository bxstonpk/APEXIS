# APEXIS

ระบบ ERP แบบหลายบริษัท (multi-company) — ขาย จัดซื้อ คลังสินค้า บัญชี และการเงินในระบบเดียว ลงบัญชีอัตโนมัติจากเอกสาร

| ส่วน | เทคโนโลยี |
|---|---|
| Backend (`apps/backend`) | Go, Hexagonal Architecture แบบ Modular Monolith, GORM, PostgreSQL |
| Frontend (`apps/frontend`) | Next.js 16 (App Router), Tailwind CSS, session แบบ HttpOnly cookie |

## Module

| Module | ความสามารถ |
|---|---|
| `identity` | ผู้ใช้, ล็อกอิน, บทบาท + ตารางสิทธิ์, สมาชิกของบริษัท, รหัสผ่านชั่วคราว |
| `organization` | บริษัท, สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน, โครงการ, ปีบัญชี / งวด, เลขที่เอกสาร |
| `ledger` | ผังบัญชี (มีผังมาตรฐาน 245 บัญชี), สมุดรายวัน, รายการบัญชี, งบทดลอง, งบแสดงฐานะการเงิน, งบกำไรขาดทุน, บัญชีแยกประเภท |
| `inventory` | สินค้า, คลัง, รับ / เบิก / โอน / ปรับปรุง, ต้นทุน FIFO, stock card |
| `purchasing` | ผู้ขาย, ใบสั่งซื้อ, รับสินค้า, ใบแจ้งหนี้ |
| `sales` | ลูกค้า, ใบเสนอราคา, ใบสั่งขาย, ใบกำกับภาษี |
| `finance` | การเงิน: ทะเบียนเงินสด / ธนาคาร, รับเงิน, จ่ายเงิน + ภาษีหัก ณ ที่จ่าย, โอนเงิน |

หน้าเว็บเป็นแบบ app launcher: เลือกบริษัท → กดไอคอนแอป → เปิดแอปในแท็บใหม่ (ตอนนี้มีหน้าจริงของแอปองค์กร ผู้ใช้และสิทธิ์ บัญชี และการเงิน)

## เริ่มต้นใช้งาน (development)

ต้องมี Go, Node.js และ PostgreSQL

```bash
# 1) ตั้งค่า — คัดลอกแล้วกรอกค่าฐานข้อมูล / JWT_SECRET / ผู้ดูแลเริ่มต้น
cp .env.example .env

# 2) backend (สร้างตารางอัตโนมัติตอนเริ่ม)
cd apps/backend
go run ./cmd/api

# 3) frontend
cd apps/frontend
npm install
npm run dev        # http://localhost:5173
```

## ทดสอบ

```bash
# backend — integration test ต้องชี้ไป DB สำหรับทดสอบเท่านั้น (test จะ DROP ตาราง)
IDENTITY_TEST_DSN="postgres://user:pass@localhost:5432/apexis_test?sslmode=disable" go test -p 1 ./...

# frontend
npm test
npm run build
```

## เอกสาร

- [docs/](docs/README.md) — requirements, architecture, database, API, testing, deployment, roadmap, frontend
- [CLAUDE.md](CLAUDE.md) — มาตรฐานการพัฒนา backend (บังคับ)
