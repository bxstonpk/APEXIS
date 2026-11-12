# APEXIS — เอกสาร SDLC

APEXIS คือระบบ ERP แบบ multi-company (หนึ่งผู้ใช้อยู่ได้หลายบริษัท แต่ละบริษัทคือ tenant แยกกัน)
เอกสารชุดนี้สรุปสิ่งที่ **มีอยู่จริงในโค้ด ณ วันที่ 2026-11-11** และสิ่งที่ยังขาด

> มาตรฐานการพัฒนา (บังคับ) อยู่ที่ [CLAUDE.md](../CLAUDE.md) — เอกสารใน `docs/` อธิบายว่าระบบตอนนี้ทำอะไร
> ส่วน CLAUDE.md กำหนดว่าโค้ด *ต้อง* เขียนอย่างไร ถ้าสองที่ขัดกัน ให้ยึด CLAUDE.md

## สารบัญ

| # | เอกสาร | เนื้อหา |
|---|---|---|
| 1 | [Requirements](01-requirements.md) | ขอบเขต, ความต้องการเชิงฟังก์ชันของแต่ละ module, ความต้องการที่ไม่ใช่ฟังก์ชัน |
| 2 | [Architecture](02-architecture.md) | Modular monolith + Hexagonal, layer, transaction, การคุยข้าม module |
| 3 | [Database](03-database.md) | ตาราง, constraint, index, soft delete, migration |
| 4 | [API](04-api.md) | ข้อตกลง REST, การค้นหาผ่าน URL query, รูปแบบ response/error, endpoint ทั้งหมด |
| 5 | [Testing](05-testing.md) | กลยุทธ์การทดสอบ, วิธีรัน, รายการ test ที่มี |
| 6 | [Deployment & Operations](06-deployment.md) | config / env, การรัน, migration, ข้อควรทำก่อนขึ้น production |
| 7 | [Status & Roadmap](07-status-roadmap.md) | สถานะแต่ละส่วน, ข้อเบี่ยงเบนจากสเปก, งานค้าง |
| 8 | [Frontend](08-frontend.md) | React component, ข้อความจาก config, Tailwind, หน้า login |

## ภาพรวมสถานะ

| ส่วน | สถานะ |
|---|---|
| Backend — `identity` (ผู้ใช้, ล็อกอิน, role, สิทธิ์, สมาชิกของบริษัท) | ✅ ใช้งานได้, มี unit + integration test |
| Backend — `organization` (บริษัท / นิติบุคคล, สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน, โครงการ/ไซต์งาน, ปีบัญชี/งวด 12 งวด) | ✅ ใช้งานได้, มี unit + integration test |
| Backend — `ledger` (ผังบัญชี, สมุดรายวัน, รายการบัญชี draft/posted/reversed, งบทดลอง, บัญชีแยกประเภท, งบแสดงฐานะการเงิน, งบกำไรขาดทุน) | ✅ ใช้งานได้, มี unit + integration test |
| Backend — `inventory` (หน่วยนับ, หมวด, สินค้า, คลัง, เอกสารรับ / เบิก / โอน / ปรับปรุง, ต้นทุน FIFO, ยอดคงเหลือ, stock card) | ✅ ใช้งานได้, มี unit + integration test |
| Backend — platform (config, database, httpx, listing, numeric, reports) | ✅ ใช้ร่วมกันทุก module |
| Frontend (`apps/frontend`) — React + Tailwind | 🟡 หน้า login + หน้าหลักชั่วคราว, มี test |
| `services/` | ⬜ ยังว่าง |
| CI/CD, Dockerfile, versioned migration, audit log | ⬜ ยังไม่มี |

## โครงสร้าง repo

```text
APEXIS/
├── CLAUDE.md                 # มาตรฐานการพัฒนา backend (บังคับ)
├── .env.example              # ตัวอย่าง config
├── docs/                     # เอกสารชุดนี้
├── apps/
│   ├── backend/              # Go module: apexis/backend
│   │   ├── cmd/api/main.go   # entry point ของ HTTP API
│   │   └── internal/
│   │       ├── platform/     # config, database, httpx, listing (ใช้ร่วมทุก module)
│   │       └── modules/
│   │           ├── identity/
│   │           ├── organization/
│   │           ├── ledger/
│   │           └── inventory/
│   └── frontend/             # React + Vite + Tailwind (ดู 08-frontend.md)
└── services/                 # (ยังว่าง)
```
