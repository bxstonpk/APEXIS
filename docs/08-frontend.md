# 8. Frontend

`apps/frontend` — Next.js 16 (App Router) + React 19 + TypeScript + Tailwind CSS v4

## 8.1 กติกา

| กติกา | ทำอย่างไร |
|---|---|
| **ทุกอย่างเป็น component** | หน้า (page) ประกอบจาก component; feature component (เช่น `LoginForm`) ไม่ผูกกับ router / layout จึง embed ที่อื่นได้ |
| **server / client แยกชัด** | หน้า (`page.tsx`) เป็น server component เป็นค่าเริ่มต้น; component ที่มี state / event ใส่ `'use client'`; โค้ดที่แตะ cookie / token อยู่ใน `src/lib/server/` เท่านั้น |
| **ข้อความทั้งหมดอยู่ใน config** | ปุ่ม, label, placeholder, คำอธิบาย, ข้อความ error, ชื่อแท็บ — อยู่ที่ `src/config/locales/th.ts` ที่เดียว ห้ามเขียนข้อความที่ผู้ใช้เห็นลงใน component |
| **CSS ใช้ Tailwind เท่านั้น** | component ใช้ utility class; ไม่มีไฟล์ CSS ของ component |
| **สี / ฟอนต์ / เงา เป็น token** | อยู่ใน `@theme` ของ `src/styles/app.css` ที่เดียว — Tailwind สร้าง class ให้ เช่น `bg-primary`, `text-ink-muted`, `shadow-card` |
| **แนวทางหน้าตา** | สีจากโลโก้ (กรมท่า `primary` + ส้ม `accent`), พื้นเรียบ เส้นบาง เงาน้อย, ขนาดกระชับแบบโปรแกรมทำงาน (ช่องกรอก / ปุ่มสูง 40px, แถวตาราง 48px), มุมโค้ง `--radius-control` 6px / `--radius-panel` 10px; แท็บของแอปใช้แถบบนสีเข้ม `nav` |
| **การเข้าถึง (a11y)** | ช่องบังคับมี `*` (อยู่นอก `<label>` — ชื่อช่องไม่เปลี่ยน) + `aria-required`; ส่งฟอร์มไม่ผ่านโฟกัสช่องแรกที่ผิด (`focusFirstInvalid`); ลิงก์ "ข้ามไปเนื้อหาหลัก" ไป `#main`; ข้อความทุกสี contrast ≥ 4.5:1; รองรับ `prefers-reduced-motion`; ช่องกรอก 16px บนมือถือ (iOS ไม่ซูม); `scroll-padding` กันแถบ sticky บังช่องที่โฟกัส |
| **ฟอนต์** | IBM Plex Sans Thai (ไทย + ละติน) self-hosted ผ่าน `@fontsource` — โหลดใน `app/layout.tsx` เฉพาะน้ำหนัก 400 / 500 / 600 |
| **ค่าตั้งของแอป** | path ของ API / หน้า, ชื่อ query parameter, ชื่อ cookie อยู่ที่ `src/config/app.ts`; ที่อยู่ backend อยู่ใน env `APEXIS_API_URL` (ฝั่ง server เท่านั้น) |
| **โลโก้ / ภาพแบรนด์** | `src/config/brand.ts` ที่เดียว (alt text อยู่ใน `texts.app.logoAlt`) |
| **การค้นหาใช้ URL query** | หน้าใดที่มีการค้นหาเก็บเงื่อนไขไว้ใน URL แล้วส่งต่อให้ API ตรงๆ (ดู [04-api.md §4.2](04-api.md)) — login พากลับหน้าเดิมพร้อม query ผ่าน `?redirect=` |

## 8.2 โครงสร้าง

```text
apps/frontend/
├── next.config.ts              # output: 'standalone'
├── postcss.config.mjs          # Tailwind (@tailwindcss/postcss)
├── vitest.config.ts            # alias @ -> src, jsdom, setup
├── .env.example                # APEXIS_API_URL (ที่อยู่ backend — ใช้ฝั่ง server เท่านั้น)
├── public/                     # favicon, brand/apexis-logo.webp (ทำจาก docs/brand/ — ดู §8.8)
└── src/
    ├── proxy.ts                # ★ ด่านหน้า: ไม่มี session -> /login?redirect=<หน้าเดิม + query>
    ├── app/
    │   ├── layout.tsx          # <html lang="th">, ชื่อแท็บ "%s | APEXIS", favicon, โหลด styles/app.css
    │   ├── (auth)/login/page.tsx   # หน้า login (เข้าสู่ระบบแล้ว = ไปหน้าเดิม)
    │   ├── (app)/
    │   │   ├── page.tsx                    # / -> บริษัทล่าสุด / บริษัทเดียว / หน้าเลือกบริษัท
    │   │   ├── companies/page.tsx, new/    # เลือกบริษัท (?q=&page=), สร้างบริษัท
    │   │   └── c/[companyId]/
    │   │       ├── page.tsx                # ★ หน้ารวมแอป (ไอคอนกลางจอ)
    │   │       ├── [app]/                  # แอปที่ยังไม่มีหน้าจริง: layout (AppShell), หน้า "กำลังพัฒนา"
    │   │       └── organization/           # ★ แอปที่มีหน้าจริงแล้วมีโฟลเดอร์ของตัวเอง (Next เลือกก่อน [app])
    │   │           ├── layout.tsx          #   AppShell appKey="organization"
    │   │           ├── branches/  business-units/  departments/  cost-centers/  project-sites/
    │   │           │                       #   ข้อมูลหลัก: รายการ / new / [itemId]
    │   │           ├── fiscal-years/       #   ปีบัญชี: รายการ / new / [itemId] (12 งวด + ปิด / เปิดงวด)
    │   │           ├── numbering/          #   เลขที่เอกสาร (แก้ทีละแถว + ตัวอย่างทันที)
    │   │           ├── company/            #   ข้อมูลบริษัท
    │   │           └── [...rest]/          #   เมนูที่ยังไม่ทำ = "กำลังพัฒนา"
    │   └── api/
    │       ├── session/route.ts       # ★ POST เข้าสู่ระบบ / GET ผู้ใช้ปัจจุบัน / DELETE ออกจากระบบ
    │       ├── session/expired/       # backend ตอบ 401 ระหว่าง render -> ลบ cookie แล้วไป login
    │       └── v1/[...path]/route.ts  # ★ ส่งต่อไป backend + เติม Bearer จาก cookie
    ├── config/
    │   ├── locales/th.ts       # ★ ข้อความทั้งหมดของแอป (ภาษาไทย)
    │   ├── texts.ts            # เลือกภาษา + format('{name}') แทนค่าตัวแปร
    │   ├── app.ts              # ★ ค่าตั้ง: API path, routes, ชื่อ cookie, page size
    │   ├── apps.ts             # ★ แอป + เมนู + สิทธิ์ที่ต้องมี + ไอคอน / สี
    │   └── brand.ts            # ★ โลโก้ (ไฟล์ + ขนาด), favicon
    ├── styles/app.css          # ★ Tailwind + design tokens (@theme)
    ├── components/             # UI พื้นฐาน ไม่มีข้อความในตัว (รับผ่าน props)
    │   ├── Button  ButtonLink  TextField  PasswordField  SelectField  Alert  Badge  Card  Spinner
    │   ├── PageHeader  DataTable (เรียงด้วยลิงก์ ?sort=)  Pagination (ลิงก์ ?page=)
    │   └── ReferenceSelect     # ช่องเลือกข้อมูลอ้างอิงแบบพิมพ์ค้นหา (combobox)
    ├── features/
    │   ├── auth/               # login / logout API, LoginForm, LoginPanel, LogoutButton (client)
    │   ├── shell/              # TopBar, AppTile, AppShell, AppNav, CloseAppButton, MessagePanel
    │   ├── lists/ListParts.tsx # ListHeader, ListToolbar, ListView, StatusBadge, RefText, rowHref
    │   ├── forms/              # FormSection, FormActions, AddressFields, StatusSelect, useResourceForm
    │   ├── organization/       # resources (path / สิทธิ์ / query keys), ฟอร์มข้อมูลหลัก, ปีบัญชี, เลขที่เอกสาร, บริษัท
    │   ├── companies/          # CompanyForm (สร้างบริษัท)
    │   └── branches/           # types, BranchForm
    ├── layouts/AuthLayout.tsx  # โครงหน้าก่อนเข้าสู่ระบบ
    └── lib/
        ├── http.ts             # เรียก API (same-origin) + แปลง error.code -> ข้อความ
        ├── permissions.ts      # can / covers — กติกาเดียวกับ backend
        ├── url.ts              # param, pickQuery, withQuery (เงื่อนไขค้นหาใน URL)
        ├── validation.ts       # รูปแบบ code / เลขที่สาขา / รหัสไปรษณีย์ ... (ตรงกับ backend)
        ├── numbering.ts        # ตรวจ / แสดงตัวอย่างเลขที่เอกสาร (กติกาเดียวกับ backend platform/numbering)
        ├── format.ts           # formatDate (วันที่แบบไทย พ.ศ.)
        ├── cx.ts
        └── server/
            ├── session.ts      # ★ อ่าน / เขียน cookie, backendUrl, safeRedirect, ตรวจ origin
            ├── backend.ts      # ★ backendGet — server component อ่านข้อมูลจาก backend พร้อม token
            ├── company.ts      # ★ getCompanyContext — บริษัท + สิทธิ์ (cache ต่อ request)
            └── list.ts         # ★ loadList, loadRef, listHrefFromBack — หน้ารายการ
```

## 8.3 แก้ข้อความ / สี

```ts
// src/config/locales/th.ts — เปลี่ยนที่นี่ ทุกปุ่ม "เข้าสู่ระบบ" ในแอปเปลี่ยนตาม
auth: { login: { submit: 'เข้าสู่ระบบ', ... } }

// ข้อความ error จาก API: key = error.code ที่ backend ส่งมา
errors: { invalid_email_or_password: 'อีเมลหรือรหัสผ่านไม่ถูกต้อง', ... }
```

```css
/* src/styles/app.css — เปลี่ยนสีหลักของทั้งแอป */
@theme { --color-primary: #1d3f8f; }
```

ใช้ component ซ้ำในที่อื่นด้วยข้อความต่างไป (ยังมาจาก config):

```tsx
<LoginForm onSuccess={handleSuccess} texts={{ ...texts.auth.login, submit: texts.auth.reauth.submit }} />
```

## 8.4 หน้า login

- `/login` — ฟอร์มอีเมล + รหัสผ่าน (ปุ่มแสดง / ซ่อนรหัสผ่าน)
- ตรวจช่องว่าง / รูปแบบอีเมลก่อนส่ง (ไม่เรียก API) แล้ว focus ช่องที่ผิด
- เรียก `POST /api/session` (route handler ของ Next) → ส่งต่อไป backend `POST /api/v1/auth/tokens`
  - สำเร็จ: Next เก็บ token ใน cookie `apexis_session` แล้วตอบเฉพาะข้อมูลผู้ใช้ — browser ไม่เห็น token
    แล้วไปหน้าเดิมจาก `?redirect=` (รับเฉพาะ path ภายในแอป กัน open redirect) หรือหน้าหลัก
  - ผิดพลาด: แสดงข้อความจาก `texts.errors[error.code]` (ไม่รู้จัก = `unknown`), ล้างช่องรหัสผ่าน
- ออกจากระบบ: `DELETE /api/session` (ลบ cookie)
- เข้าถึงได้ (a11y): label ผูก input, `aria-invalid` / `aria-describedby`, error แจ้งด้วย `role="alert"`

### session / การเรียก API

```text
browser --(cookie apexis_session)--> Next server --(Authorization: Bearer <token>)--> backend Go
        /api/session                                /api/v1/auth/tokens
        /api/v1/*  -------------------------------> /api/v1/*
```

- cookie: `HttpOnly` (JavaScript อ่านไม่ได้ — กันขโมย token ผ่าน XSS), `SameSite=Lax`, `Secure` เมื่อ production, หมดอายุพร้อม token
  - ค่า = base64url ของ JSON `{token, expiresAt, user}` (ไม่ได้เข้ารหัส — ความปลอดภัยมาจาก HttpOnly และ backend ตรวจ token เอง)
- `proxy.ts`: ไม่มี session / หมดอายุ / cookie เสีย → `/login?redirect=<หน้าเดิม + query>`
- `/api/v1/*`: ส่งต่อ method, query, body, header `accept` / `content-type` / `accept-language` — ไม่ส่ง cookie ของ Next ไป backend
  - backend ตอบ 401 → ลบ cookie (ครั้งถัดไป proxy พาไป login)
  - backend ล่ม → 502 `network_error`
- กัน CSRF: คำขอที่เปลี่ยนข้อมูล (POST / PUT / PATCH / DELETE) ต้องมี `Origin` ตรงกับแอป (ไม่มี Origin ใช้ `Sec-Fetch-Site`) ไม่ตรง = 403 `permission_denied`
- component ฝั่ง client เรียก `apiRequest('/companies?...')` → `/api/v1/companies?...` ตามปกติ ไม่ต้องจัดการ token

## 8.5 หน้ารวมแอป (แบบ app launcher)

```text
/  ──>  บริษัทที่เปิดล่าสุด (cookie apexis_company) หรือบริษัทเดียว ──> /c/{companyId}
   └─>  นอกนั้น ──> /companies (เลือก / ค้นหา / สร้างบริษัท)

/c/{companyId}                 หน้ารวมแอป: ไอคอนแอปกลางจอ (เฉพาะแอปที่มีสิทธิ์)
  │  กดไอคอน = window.open(url, "apexis-{companyId}-{app}") — แท็บใหม่ ชื่อแท็บต่อแอป
  ▼                              (กดแอปเดิมซ้ำ = กลับไปแท็บเดิม ไม่เปิดซ้ำ)
/c/{companyId}/{app}/{menu}    แท็บของแอป: [✕] [ไอคอน ชื่อแอป] [เมนูของแอป ...] [ชื่อบริษัท]
     ✕ = window.close() กลับไปแท็บหน้ารวมแอป
         (แท็บที่เปิดเอง เช่น พิมพ์ URL / bookmark ปิดด้วย script ไม่ได้ -> ไปหน้ารวมแอปในแท็บนั้นแทน)
```

- แอป / เมนู / สิทธิ์อยู่ที่ `src/config/apps.ts` ที่เดียว, ชื่ออยู่ที่ `texts.apps`, สีไอคอนเป็น token `--color-app-*`
  - เมนูแสดงเมื่อมีสิทธิ์ `permission` ของเมนู (ตรวจแบบเดียวกับ backend: `*`, `sales.*:read`, `sales.order:*`)
  - แอปแสดงเมื่อเหลืออย่างน้อยหนึ่งเมนู; หน้าแรกของแอป = เมนูแรกที่มีสิทธิ์
  - ซ่อนเมนูเพื่อความสะดวกเท่านั้น — backend ตรวจสิทธิ์ทุกคำขอเสมอ
- บริษัท + สิทธิ์โหลดด้วย `getCompanyContext(companyId)`: 2 คำขอพร้อมกัน ครั้งเดียวต่อ request (React `cache`) แม้ layout / page เรียกซ้ำ
  - `/me/companies/{id}` (organization) ชื่อบริษัท — สมาชิกทุกคนอ่านได้ ไม่ต้องมีสิทธิ์อ่านบริษัท
  - `/companies/{id}/access` (identity) สิทธิ์ของผู้ใช้
  - แยก 2 คำขอเพราะข้อมูลอยู่คนละ module (JOIN ข้าม module ไม่ได้ และ identity ไม่พึ่ง organization) — เรียกพร้อมกันจึงไม่เพิ่มเวลารอ
- เมนูที่ยังไม่มีหน้าจริงแสดง "หน้านี้กำลังพัฒนา"

### หน้ารายการ (แม่แบบ: `organization/branches`, `departments` ...)

| ส่วน | ทำอย่างไร |
|---|---|
| อ่านข้อมูล | server component: `loadList(api, searchParams, KEYS)` — เลือก key ที่อนุญาตจาก URL แล้วส่งให้ API ตรงๆ คิวรี 1 ครั้ง (KEYS อยู่ใน `features/organization/resources.ts` ตรงกับ backend) |
| ค้นหา / กรอง | `ListToolbar` = `<Form action="">` ของ `next/form` (GET) — เขียนลง URL, กด Back ได้เงื่อนไขเดิม; ส่ง `sort` / `page_size` เดิมเป็น hidden |
| กรองด้วยข้อมูลอ้างอิง | `RefFilter` (ReferenceSelect ที่มี `name`) ส่ง id ลง URL เช่น `?branch_id=`; ตอนเปิดหน้า `loadRef` โหลดชื่อมาแสดง — คิวรีเพิ่มเฉพาะตัวกรองที่ใช้ และเรียกพร้อมกับรายการ (`Promise.all`) |
| เรียง | `DataTable` หัวคอลัมน์เป็นลิงก์ `?sort=key` / `?sort=-key` (กลับหน้า 1) |
| แบ่งหน้า | `Pagination` ลิงก์ `?page=` จาก `meta` ของ API |
| เพิ่ม / แก้ไข | หน้าแยก `/new`, `/{id}` — ฟอร์มเป็น client component เรียก `/api/v1/*` (POST / PUT / DELETE) แล้ว `router.push(listHref)` + `router.refresh()` |
| กลับหน้ารายการเดิม | ลิงก์แถวพา `?back=<query ของรายการ>`; หน้าแก้ไขกรองด้วย `pickQuery` อีกชั้นก่อนใช้ |
| สิทธิ์ | ปุ่ม "สร้าง" / บันทึก / ลบ แสดงเมื่อ `can(permissions, '...:manage')`; ไม่มีสิทธิ์ = ฟอร์มอ่านอย่างเดียว |
| ฟอร์ม | `useResourceForm` (POST / PUT / DELETE + กลับหน้ารายการ + error จาก config), `FormSection`, `FormActions`, `AddressFields` |
| แจ้งผล | `showToast()` (`lib/toast.ts`) → `Toaster` ใน AppShell มุมขวาล่าง หายเองใน 4 วินาที ไม่แย่งโฟกัส — `useResourceForm` เรียกให้เองหลังสร้าง / บันทึก / ลบ (เก็บใน sessionStorage ข้ามการเปลี่ยนหน้า) |
| หน้าว่าง | `ListView` แยก 2 แบบ: ยังไม่มีข้อมูล (+ ปุ่มสร้างเมื่อมีสิทธิ์) / มีตัวกรองแต่ไม่พบ (+ ล้างตัวกรอง) |
| หน้าฟอร์ม | `FormPage` มีเส้นทาง องค์กร › รายการ › หน้านี้; `FormSection` แบบ 2 คอลัมน์ (หัวข้อ + คำอธิบาย | ช่องกรอก) เหมือนหน้าข้อมูลบริษัท |

### ช่องเลือกข้อมูลอ้างอิง (`ReferenceSelect`)

- พิมพ์แล้วค้นหาจาก API (`?q=&page_size=20` + `params` เช่น `status=active`) หน่วง 250ms และยกเลิกคำขอเก่า — ใช้กับข้อมูลจำนวนมากได้
- แสดง "รหัส · ชื่อ", ปุ่ม ✕ ล้างค่า, `excludeId` (หน่วยแม่ห้ามเลือกตัวเอง)
- คีย์บอร์ด ↑ ↓ Enter Esc ตาม WAI-ARIA combobox
- ใช้ได้ทั้งในฟอร์ม (`value` + `onChange`) และตัวกรองใน URL (`name` + `defaultValue`)

### แอปผู้ใช้และสิทธิ์ (`users/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| สมาชิก | รายการ (ค้นหา, กรองสถานะ / บทบาท, ป้าย "รอเปลี่ยนรหัสผ่าน"), เพิ่ม, จัดการ | เพิ่มด้วยอีเมล — ไม่พบบัญชีจึงเปิดส่วน "สร้างบัญชีใหม่" (ชื่อ + รหัสผ่านชั่วคราว ปุ่มสุ่ม / คัดลอก); หน้าจัดการเปลี่ยนบทบาท และพัก / เปิดใช้งาน (ยืนยันก่อน) — แก้บัญชีตัวเองไม่ได้ |
| บทบาท | รายการ, สร้าง / แก้ไข / ลบ | ตารางสิทธิ์ (`PermissionMatrix`) จาก `GET /permissions`: กลุ่มตาม module, แถว = ข้อมูล, คอลัมน์ = การกระทำ, เลือกทั้งแอป (checkbox แบบเลือกบางส่วน), เลือก action อื่น = เลือก "ดู" ให้ด้วย; owner อ่านอย่างเดียว; wildcard ของบทบาทเดิมแสดงเป็นช่องที่ครอบคลุม |

- ชื่อสิทธิ์ภาษาไทยอยู่ที่ `texts.permissions` (module / resource / action) — ไม่มีชื่อ = แสดงรหัส จึงเพิ่มสิทธิ์ใหม่ใน backend ได้โดยตารางไม่พัง
- ตรรกะตาราง (`features/users/matrix.ts`) เป็น pure function มีเทสต์แยก

### บังคับเปลี่ยนรหัสผ่าน

```text
login (รหัสชั่วคราว) → cookie มี user.must_change_password → proxy.ts พาไป /change-password?redirect=<หน้าเดิม>
  → PUT /api/session/password (Next) → backend PUT /me/password → ล้างค่าใน cookie → โหลดหน้าเดิมใหม่ทั้งหน้า
```

- ใช้ `window.location.assign` (ไม่ใช่ router) เพราะ router ของ Next จำ redirect เดิมไว้ — ต้องล้างแคช
- ช่องรหัสผ่านตั้ง `autocomplete` (current-password / new-password) และมีช่อง username ซ่อน ให้ password manager ทำงานได้
- `Toaster` อยู่ใน root layout — แจ้งผลได้ทุกหน้า (ข้ามการโหลดหน้าใหม่ด้วย sessionStorage)

### แอปบัญชี (`accounting/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| รายการบัญชี | รายการ (กรองสถานะ / สมุด / บัญชี / ช่วงวันที่), สร้าง, ร่าง = ฟอร์ม, ผ่านแล้ว = หน้าแสดง + กลับรายการ | ตารางบรรทัด: บัญชี, คำอธิบาย, เดบิต, เครดิต (กรอกด้านหนึ่งอีกด้านถูกล้าง); ยอดรวม / ผลต่างคำนวณสด; มิติวิเคราะห์ซ่อนในแต่ละบรรทัด (ปุ่มไอคอนพร้อมจำนวนที่ระบุ); "บันทึกร่าง" ไม่ต้องสมดุล, "บันทึกและผ่านรายการ" ต้องสมดุล (ยืนยันก่อน) |
| ผังบัญชี | ต้นไม้ (`GET /accounts/tree`), ค้นหา / กรอง = รายการแบบแบน (`GET /accounts` กรองใน DB), สร้าง / แก้ไข | แบบเดียวกับโปรเจกต์ต้นแบบ: ขยาย / ยุบทีละกลุ่มหรือทั้งหมด, ระดับบนสุดขยายไว้, บัญชีคุมตัวหนา + ป้าย "กลุ่มบัญชี"; ยังไม่มีบัญชี = ปุ่ม "สร้างผังบัญชีมาตรฐาน"; จอเล็กซ่อนคอลัมน์ชื่ออังกฤษ / หมวด และลดย่อหน้า |
| สมุดรายวัน | รายการ, สร้าง / แก้ไข | ตัวอย่างเลขที่รายการทันทีขณะพิมพ์รูปแบบ (`lib/numbering`) |
| รายงาน | หน้าเดียว `?book=&start_date=&end_date=&account_id=` | งบทดลอง (กดชื่อบัญชี = บัญชีแยกประเภท), งบแสดงฐานะการเงิน (+ ส่วนบัญชีพัก / คำเตือน), งบกำไรขาดทุน, บัญชีแยกประเภท (100 บรรทัดต่อหน้า); ปุ่ม**พิมพ์** (`print:` ซ่อนเมนู / ตัวกรอง) และ**ส่งออก CSV** (UTF-8 + BOM เปิดใน Excel ได้, ตัวเลขไม่คั่นหลัก, กันสูตร) — บัญชีแยกประเภทส่งออกเฉพาะหน้าที่แสดง |

- จำนวนเงินคำนวณด้วย BigInt สเกล 10^6 (`features/accounting/money.ts`) — ไม่ใช้ float; แสดงคั่นหลัก ทศนิยม 2 ตำแหน่ง ติดลบในวงเล็บ
- ตรรกะต้นไม้ (`tree.ts`), CSV (`csv.ts`, `reportCsv.ts`) เป็น pure function มีเทสต์

### แอปการเงิน (`finance/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| รับเงิน / จ่ายเงิน | รายการ (กรองประเภท / สถานะ / บัญชีเงิน / ช่วงวันที่), บันทึก, ดู + ยกเลิก | ฟอร์มเดียวกัน (`MoneyDocForm`): เลือกประเภท → ชำระหนี้ = เลือกลูกค้า / ผู้ขายแล้วแสดงใบแจ้งหนี้ค้าง (ติ๊กเลือก + ยอดที่ตัด), อื่นๆ = กรอกผู้รับ / ผู้จ่าย + บรรทัดบัญชี; ภาษีหัก ณ ที่จ่ายคำนวณตัวอย่างสูตรเดียวกับ backend; ช่องทางหลายบัญชี + กล่องสรุปบอก "ตรง / ต่าง" (ไอคอน + ข้อความ) และปุ่มใส่ยอดที่เหลือ |
| โอนเงิน | รายการ, บันทึก, ดู + ยกเลิก | |
| บัญชีเงินสด / ธนาคาร, ประเภทภาษี | รายการ + ฟอร์ม | |
| รายงานภาษี | ภ.ง.ด.3 / 53 และภาษีถูกหัก | พิมพ์ + ส่งออก CSV (ใช้ `ReportActions` ของแอปบัญชี) |
| ตั้งค่า | สมุดรายวัน + บัญชีภาษีหัก ณ ที่จ่าย | |

- ผู้ใช้หน้ารับ / จ่ายเงินต้องมีสิทธิ์ดูลูกค้า / ผู้ขาย (ช่องค้นหาใช้ API ของขาย / จัดซื้อ)
- ใบเสร็จรับเงินที่บันทึกแล้วพิมพ์ A4 ได้ (`receipts/{id}/print`) — ใช้ `PrintDocument` ของแอปขาย

### ลูกค้า / ผู้ขาย: ดึงข้อมูลจากกรมสรรพากร (`features/partners/`)

- `VatLookup`: ช่องเลขผู้เสียภาษี + ปุ่ม "ดึงข้อมูลจากกรมสรรพากร" — ฟอร์มใหม่ดึงเองเมื่อครบ 13 หลัก (ฟอร์มแก้ไขต้องกดปุ่ม เพื่อไม่ทับข้อมูลเดิม); พบ 1 สาขาเติมทันที, หลายสาขาให้เลือก, ไม่พบ / ระบบล่มแจ้งใน `role="status"` ให้กรอกเอง
- `ThaiAddressFields`: ช่องที่อยู่ตามแบบสรรพากร (ป้ายเปลี่ยนเป็น แขวง / เขต เมื่อเป็นกรุงเทพฯ) + ตัวอย่างที่อยู่ที่จะพิมพ์ (`formatThaiAddress` กฎเดียวกับ backend)
- ใช้ใน `CustomerForm` (แอปขาย) และ `SupplierForm` (แอปจัดซื้อ → ผู้ขาย)
- `VatLookup` ตรวจก่อนว่าเลขนี้มีรหัสอยู่แล้วหรือไม่ (ค้น `?q=` ของรายการ) — มี = แจ้ง + ลิงก์ไปรหัสเดิม; พบหลายสาขาในกรมสรรพากร = สำนักงานใหญ่เป็นสาขาหลัก และมีปุ่ม "เพิ่มเป็นสาขา"
- `BranchesEditor`: สาขาอื่น (เลขสาขา / ชื่อ / ที่อยู่) — ตรวจเลข 5 หลักและไม่ซ้ำสาขาหลักก่อนส่ง
- `SalesDocForm` โหลดสาขาของลูกค้าที่เลือก — มีมากกว่า 1 สาขาแสดงช่อง "สาขาลูกค้า"

### หน้าภาพรวม (`{app}/dashboard`, `features/dashboard/`)

- เมนูแรกของแอปขาย / จัดซื้อ / คลัง / บัญชี / การเงิน (สิทธิ์ = `*.report:read`) — เปิดแอปแล้วเจอภาพรวมก่อน
- **shadcn/ui** (ใช้เฉพาะ dashboard ตอนนี้): `components.json`, `src/lib/utils.ts` (`cn`), `src/components/ui/{card,chart,skeleton}.tsx`; token ของ shadcn (`card`, `muted`, `border`, `chart-1/2` ...) ผูกกับสี APEXIS ใน `app.css` — ไม่ใช้ `accent` ของ shadcn (APEXIS accent = ส้มโลโก้)
- ช่วงเวลา: `?period=month|quarter|year|custom&start_date=&end_date=` (`resolvePeriod`) — ค่าเริ่มต้นเดือนนี้ถึงวันนี้, กด Back แล้วได้ช่วงเดิม
- `KpiCard`: ตัวเลขหลัก + เทียบช่วงก่อน (ลูกศร + ข้อความ ไม่ใช้สีอย่างเดียว; `upIsGood=false` สำหรับค่าใช้จ่าย / ยอดซื้อ) ทั้งการ์ดลิงก์ไปรายการ / รายงานที่กรองช่วงเดียวกัน
- ภาพรวมการขาย: กราฟรายเดือนตามปีบัญชี (เดือนเริ่มงวด → สิ้นงวด) เทียบปีบัญชีก่อนหน้าเมื่อปีก่อนมียอด (ชื่อ series "ปีบัญชี 2569 / 2568")
- `TrendChart` (Recharts ผ่าน shadcn Chart): 12 เดือน 1–2 series สี `chart-1` / `chart-2` (ผ่านตัวตรวจ palette ของ dataviz: CVD / ความต่างสี / contrast), ไม่มี animation, tooltip, legend เรียงตาม series, ปุ่ม "ดูเป็นตาราง"; ไม่มีข้อมูล = ข้อความแทนกรอบกราฟว่าง
- `BarList` / `AgingList` / `RowList`: แถบเทียบขนาด (ตัวเลขเป็นข้อความเสมอ), อายุหนี้ 5 ช่วง, รายการล่าสุด
- มือถือ: การ์ดเรียง 1 คอลัมน์, กราฟเต็มความกว้าง

### แอปคลังสินค้า (`inventory/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| ภาพรวม | dashboard | การ์ดลิงก์ไปรายงานสินค้าคงเหลือ / สินค้านับสต็อก / เอกสารในช่วง |
| เอกสารสต็อก | รายการ (กรองประเภท / สถานะ / คลัง / วันที่), สร้าง, ร่าง (แก้ไข / ลบ), ผ่านแล้ว (ดู + ยกเลิก) | `StockDocForm`: รับเข้า / เบิกออก / โอนระหว่างคลัง (จาก → ไป) / ปรับปรุงยอด (จำนวนลบได้); ต้นทุนต่อหน่วยแสดงเฉพาะที่ต้องใช้ (รับเข้า, ปรับปรุงเพิ่ม) อื่นๆ = FIFO ตอนผ่าน; "บันทึกร่าง" หรือ "บันทึกและผ่านเอกสาร" (ยืนยันก่อน) |
| สินค้า / หมวด / หน่วยนับ / คลัง | รายการ + ฟอร์ม | สินค้า: ประเภท, หน่วยนับ, หมวด, จุดสั่งซื้อ (เฉพาะนับสต็อก), บัญชี; หมวด: บัญชีซื้อ / สินค้า / รายได้; คลัง: ผูกสาขา |
| ปิดงวดสต็อก | รายการการปิดงวด + ปิดงวด ณ วันสิ้นงวด / ยกเลิก | ใช้กับระบบ periodic |
| รายงาน | สินค้าคงเหลือ (ณ วันที่ / คลัง / หมวด) → คลิกสินค้าไปบัตรสินค้า (ยกมา, เข้า / ออก, คงเหลือ) | พิมพ์ + CSV |
| ตั้งค่า | บัญชีค่าเริ่มต้น (ซื้อ / สินค้า / ต้นทุนขาย / สมุด), ระบบบัญชีสินค้าต่อปีบัญชี | |

- ตารางรายการในฟอร์ม: กล่องเลื่อนตารางต้อง `relative` (label ซ่อนแบบ `sr-only` เป็น absolute — ไม่งั้นทำให้ทั้งหน้าเลื่อนแนวนอนบนมือถือ)
- ฟอร์มกรองรายงาน `key={query}` — เปลี่ยนรายงาน / วันที่ผ่านลิงก์แล้วช่องกรองตรงกับ URL

### แอปจัดซื้อ (`purchasing/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| ภาพรวม | dashboard | การ์ดลิงก์ไปใบแจ้งหนี้ซื้อ / อายุเจ้าหนี้ / ใบสั่งซื้อค้างรับ |
| ใบสั่งซื้อ | รายการ, สร้าง, ร่าง (แก้ไข + อนุมัติ / ลบ), ดู (รับสินค้า / บันทึกใบแจ้งหนี้ / ปิด / ยกเลิก + ความคืบหน้ารายบรรทัด), พิมพ์ A4 | ใช้ฟอร์มเดียวกับใบเสนอราคา / ใบสั่งขาย (`SalesDocForm` kind `purchase-order`, prop `partiesApi`) — เลือกผู้ขายแล้วโหลดสาขาและการจด VAT; VAT = อัตราในค่าตั้งจัดซื้อเมื่อผู้ขายจด VAT |
| ใบรับสินค้า | รายการ, สร้างจากใบสั่งซื้อ (`new?order_id=`), ดู + ยกเลิก | `PurchaseFulfilForm` ตั้งต้นจำนวนคงเหลือ (สั่ง − รับแล้ว) + คลังที่รับเข้า |
| ใบแจ้งหนี้ | รายการ, บันทึกจากใบสั่งซื้อ, ดู + ยกเลิก + "จ่ายเงิน" ไปแอปการเงิน | เลขที่ / วันที่ใบกำกับภาษีของผู้ขาย + วันที่บันทึกบัญชี; จำนวน = รับแล้ว − แจ้งหนี้แล้ว; ราคาแก้ได้ (ผลต่างราคาแสดงรายบรรทัด); ผู้ขายตามใบกำกับ ณ วันออก (`bill_from`) |
| ผู้ขาย | รายการ + ฟอร์ม (สาขา, ดึงข้อมูลกรมสรรพากร) | |
| รายงาน | ใบสั่งซื้อค้างรับ / ค้างแจ้งหนี้, อายุเจ้าหนี้ | พิมพ์ + CSV (`ReportTable` ใช้ร่วมกับรายงานขาย) |
| ตั้งค่า | อัตรา VAT, สมุดรายวันซื้อ, พักรับสินค้า (GR/IR), เจ้าหนี้, ภาษีซื้อ, ผลต่างราคา | |

### แอปขาย (`sales/`)

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| ใบเสนอราคา | รายการ, สร้าง, ร่าง (แก้ไข + ส่ง / ลบ), ดู (ตอบรับ / ไม่ตอบรับ → สร้างใบสั่งขาย), พิมพ์ | ฟอร์มเดียวกับใบสั่งขาย (`SalesDocForm`): ตารางสินค้า + ยอดก่อนภาษี / VAT / รวม แสดงสด (สูตรเดียวกับ backend ใน `calc.ts`); ยืนราคาเริ่มต้น 30 วัน |
| ใบสั่งขาย | รายการ, สร้าง, ร่าง (แก้ไข + อนุมัติ / ลบ), ดู (ส่งของ / ออกใบกำกับภาษี / ปิด / ยกเลิก) | อนุมัติแล้วแสดงคำเตือนวงเงินเครดิตจาก backend (แปลเป็นไทยใน `warningText`); แต่ละบรรทัดแสดง ส่งแล้ว / ออกบิลแล้ว |
| ใบส่งของ / ใบกำกับภาษี | รายการ, สร้างจากใบสั่งขาย (`new?order_id=`), ดู + ยกเลิก | `FulfilForm` ตั้งต้นจำนวนคงเหลือ (ส่งของ = สั่ง − ส่งแล้ว, ออกบิล = ส่งแล้ว − ออกบิลแล้ว, ส่งพร้อมออกบิล = สั่ง − ส่งแล้ว); ยังไม่เคยส่ง = ตั้งต้นเป็นส่งพร้อมออกบิล; ใบกำกับภาษีมีปุ่ม "รับเงิน" ไปแอปการเงิน |
| ลูกค้า | รายการ + ฟอร์ม | เลขผู้เสียภาษี 13 หลัก, สาขา 5 หลัก (00000 = สำนักงานใหญ่), วงเงินว่าง = ไม่จำกัด |
| รายงาน | ค้างส่ง / ค้างออกบิล, อายุลูกหนี้ (ณ วันที่), ภาษีขาย | พิมพ์ + CSV |
| ตั้งค่า | จด VAT + อัตรา, สมุดรายวัน, บัญชีลูกหนี้ / ภาษีขาย / รายได้ / ต้นทุนขาย | |

- **พิมพ์ A4** (`.../print`): แผ่น `.a4-sheet` ขนาด 210×297mm บนจอเท่ากับตอนพิมพ์ (`@page a4doc` ขอบ 12mm, ช่องลงนามชิดล่าง, จอแคบย่อทั้งแผ่นด้วย zoom); ใบกำกับภาษีใช้ `bill_to` ที่เก็บในใบ; `PrintDocument` (หัวกระดาษจาก `GET /me/companies/{id}/letterhead`, ผู้ซื้อพร้อมเลขผู้เสียภาษี / สาขา, ตาราง, ยอดรวม, จำนวนเงินตัวอักษร `bahtText`, ช่องลงนาม) + `@page A4` ใน `app.css`; ชื่อใบกำกับภาษี: ไม่มี VAT = ใบแจ้งหนี้, ส่งพร้อมออกบิล = ใบส่งของ / ใบกำกับภาษี, อื่นๆ = ใบแจ้งหนี้ / ใบกำกับภาษี
- จำนวนเงินรายบรรทัดแสดงก่อน VAT (ราคารวม VAT = แสดงตามราคาที่กรอก) ให้ตรงกับ "รวมเงิน"

### แอปองค์กร

| เมนู | หน้า | หมายเหตุ |
|---|---|---|
| สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน, โครงการ / ไซต์งาน | รายการ / สร้าง / แก้ไข / ลบ | ลำดับชั้นแสดงเป็นตาราง มีคอลัมน์ "แม่" และกรอง `?parent_id=`; backend ตรวจการวน, ข้อมูลที่ถูกอ้างอิงลบไม่ได้ |
| ปีบัญชี / งวด | รายการปี, สร้างปี (backend สร้าง 12 งวด), หน้าปีพร้อมงวด | ปิด / เปิดทีละงวด (ยืนยันก่อน) — ปุ่มแสดงเฉพาะงวดแรกที่เปิด / งวดล่าสุดที่ปิด; ต้องมีสิทธิ์ `fiscal_period:close` |
| เลขที่เอกสาร | ตารางทุกชนิดเอกสาร | แก้ทีละแถว, ตัวอย่างเลขที่เปลี่ยนทันทีที่พิมพ์ (`lib/numbering.ts`), "ใช้ค่าเริ่มต้น" |
| ข้อมูลบริษัท | ฟอร์มเดียว | ทศนิยมเลือกได้ตั้งแต่ค่าปัจจุบัน (ลดไม่ได้), บันทึกแล้วอยู่หน้าเดิม |

เพิ่มหน้าจริงให้แอปใหม่: สร้างโฟลเดอร์ `c/[companyId]/<app>/` ที่มี `layout.tsx` (AppShell), `page.tsx` (`redirectToFirstMenu`), `[...rest]/page.tsx` (`ComingSoon`) แบบเดียวกับ `organization/` แล้วเพิ่มหน้าของเมนูนั้น

### การโหลดหน้า (skeleton, `features/loading/`)
- ทุกการโหลดหน้าแสดง skeleton เสมอ (ไม่ว่าจะเร็วหรือช้า) — โครงเดียวกับหน้าจริงเพื่อไม่ให้เนื้อหากระโดด; `role="status"` + `aria-busy` + ข้อความ "กำลังโหลด..." สำหรับ screen reader; ปิด animation เมื่อตั้ง reduced motion
- `PageSkeleton kind=` `list` / `dashboard` / `report` / `form` / `doc` ใช้ใน `loading.tsx`:
  - `{app}/loading.tsx` = list (หน้ารายการทั้งหมดของแอป); `dashboard/`, `reports/` ตามชื่อ; `new/`, `settings/`, `organization/company/`, `[id]` ของทะเบียน = form; `[docId]`, `[entryId]` = doc
  - หน้าใหม่: ถ้าไม่ใช่หน้ารายการ ให้เพิ่ม `loading.tsx` ตามชนิดหน้า
- เปิดแอปในแท็บใหม่ (hard load): `AppShell` แสดงแถบแอป (ชื่อ / ไอคอนจาก appKey) + skeleton ทันทีระหว่างรอสิทธิ์ (Suspense ภายใน layout)
- หน้ารวมแอป / หน้าเลือกบริษัท: แถบบนแสดงทันที ส่วนที่รอ backend อยู่ใน Suspense (`LauncherSkeleton`, `CompaniesSkeleton`)
- ผลวัด (2026-11-23, 20 หน้า): production build กดเมนู → หน้าขึ้น 70–360 ms (backend 5–50 ms ต่อคำขอ); `next dev` ช้ากว่า 10–30 เท่า (compile ตอนเปิดหน้าครั้งแรก, ไม่ prefetch) — วัดความเร็วด้วย `npm run build && npm start` เท่านั้น
- ฟอร์มที่มีรหัสผ่าน (`LoginForm`, `ChangePasswordForm`, `MemberAddForm`) ตั้ง `method="post"` — ถ้า submit ก่อน JS โหลดเสร็จ รหัสผ่านจะไม่ไปอยู่ใน URL

## 8.6 รัน / ทดสอบ

```bash
cd apps/frontend
npm install
cp .env.example .env.local      # ตั้ง APEXIS_API_URL ถ้า backend ไม่ได้อยู่ที่ 127.0.0.1:8080
npm run dev                     # http://localhost:5173
npm test                        # vitest + Testing Library
npm run typecheck
npm run build                   # typecheck + next build -> .next/standalone (ดู 06-deployment.md)
npm start                       # รันผล build (port 5173)
```

test ที่มี:
- `features/auth/LoginForm.test.tsx` (jsdom) — ข้อความจาก config, override ข้อความ, validation, สำเร็จ (เรียก `/api/session`), รหัสผ่านผิด, network error, error code ไม่รู้จัก, แสดง / ซ่อนรหัสผ่าน
- `*.server.test.ts` (node — ระบุ `// @vitest-environment node` ที่หัวไฟล์)
  - `lib/server/session.server.test.ts` — encode / decode cookie, หมดอายุ, กัน open redirect, ตรวจ origin
  - `proxy.server.test.ts` — redirect ไป login พร้อม query เดิม, cookie เสีย, ผ่านเมื่อมี session
  - `app/api/session/route.server.test.ts` — ตั้ง HttpOnly cookie โดยไม่คืน token, ส่งต่อ error, backend ล่ม, ต่าง origin = 403, GET / DELETE
  - `proxy.server.test.ts` ยังตรวจการจำบริษัทล่าสุด (cookie `apexis_company`)
- `lib/permissions.test.ts` — `covers` ตรงกับ backend, `visibleApps` กรองแอป / เมนูตามสิทธิ์
- `lib/url.test.ts` — `param`, `pickQuery`, `withQuery`
- `components/DataTable/DataTable.test.tsx` — ลิงก์เรียง (ขึ้น / ลง), `aria-sort`, ตารางว่าง, `Pagination`
- `features/shell/shell.test.tsx` — ไอคอนเปิดแท็บใหม่ด้วยชื่อแท็บของแอป, ปุ่มปิด (ปิดไม่ได้ → กลับหน้ารวมแอป)
- `features/branches/BranchForm.test.tsx` — validation, สร้าง (POST + รหัสพิมพ์ใหญ่), สำนักงานใหญ่แก้ / ลบไม่ได้, ลบแล้ว backend ปฏิเสธ, อ่านอย่างเดียว
- `components/ReferenceSelect/ReferenceSelect.test.tsx` — ค้นหาจาก API พร้อม params, เลือกด้วยคีย์บอร์ด, hidden input, ล้างค่า, ไม่พบรายการ
- `features/organization/organization.test.tsx` — แผนก (PUT + ล้างค่าอ้างอิง = null), ศูนย์ต้นทุน (ช่วงวันที่), โครงการ (ปิดต้องมีวันสิ้นสุด, อ่านอย่างเดียว), งวด (ปุ่มตามลำดับ + PATCH), เลขที่เอกสาร (ตัวอย่างทันที / ผิด = บันทึกไม่ได้), ข้อมูลบริษัท (ทศนิยมลดไม่ได้)
- `features/loading/skeleton.test.tsx` — skeleton ทุกชนิดบอก screen reader ว่ากำลังโหลด และซ่อนชิ้น skeleton จาก a11y tree
- `lib/numbering.test.ts` — ตัวอย่างเลขที่ตรงกับ backend และข้อความ error ทุกกรณี

## 8.7 เวอร์ชัน

ล็อกเวอร์ชันตรง (ไม่มี `^`):
- Next.js 16.4, React 19.3, TypeScript 5.9, Tailwind 4.3, Vitest 3.2, lucide-react 1.69 (ไอคอน), @fontsource/ibm-plex-sans-thai 5.3
- Next 16 ใช้ `proxy.ts` แทน `middleware.ts` เดิม
- เลือก major ที่ตรวจสอบ API ได้แล้ว (มี TS 7 ออกแล้ว) — การอัปเกรด major ควรทำเป็นงานแยก

## 8.8 โลโก้

- ต้นฉบับ: [`docs/brand/Apexis_Enterprise_Growth_Logo.png`](brand/Apexis_Enterprise_Growth_Logo.png)
  - 1536×1024, พื้นหลังโปร่งใส, 1.2 MB — เก็บไว้เป็นต้นฉบับ ไม่ได้ใช้บนเว็บโดยตรง
- ไฟล์บนเว็บ: `apps/frontend/public/brand/apexis-logo.webp`
  - ตัดขอบโปร่งใสแล้วย่อเป็น WebP 640×358, ~45 KB
  - แสดงที่ความกว้าง 224–288px (2× สำหรับจอความละเอียดสูง)
- favicon: `public/favicon-32.png` และ `public/apple-touch-icon.png` (180×180 พื้นขาว — iOS ไม่รองรับพื้นโปร่งใส) ทำจากสัญลักษณ์ "A"
- โลโก้ออกแบบมาสำหรับพื้นสว่าง — ตัวอักษร "APEXIS" สีกรมท่าจะจมบนพื้นมืด ถ้าจะทำ dark mode ต้องมีโลโก้อีกเวอร์ชัน
- เปลี่ยนโลโก้: สร้างไฟล์ใหม่จากต้นฉบับ แล้วแก้ `src/config/brand.ts` ที่เดียว (หน้าที่ใช้ `AuthLayout` เปลี่ยนตาม)

