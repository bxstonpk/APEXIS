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
    │   │           ├── branches/           #   รายการ / new / [branchId] — แม่แบบของหน้ารายการ
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
    │   └── PageHeader  DataTable (เรียงด้วยลิงก์ ?sort=)  Pagination (ลิงก์ ?page=)
    ├── features/
    │   ├── auth/               # login / logout API, LoginForm, LoginPanel, LogoutButton (client)
    │   ├── shell/              # TopBar, AppTile, AppShell, AppNav, CloseAppButton, MessagePanel
    │   ├── companies/          # CompanyForm
    │   └── branches/           # types, BranchForm
    ├── layouts/AuthLayout.tsx  # โครงหน้าก่อนเข้าสู่ระบบ
    └── lib/
        ├── http.ts             # เรียก API (same-origin) + แปลง error.code -> ข้อความ
        ├── permissions.ts      # can / covers — กติกาเดียวกับ backend
        ├── url.ts              # param, pickQuery, withQuery (เงื่อนไขค้นหาใน URL)
        ├── validation.ts       # รูปแบบ code / เลขที่สาขา / รหัสไปรษณีย์ ... (ตรงกับ backend)
        ├── cx.ts
        └── server/
            ├── session.ts      # ★ อ่าน / เขียน cookie, backendUrl, safeRedirect, ตรวจ origin
            ├── backend.ts      # ★ backendGet — server component อ่านข้อมูลจาก backend พร้อม token
            └── company.ts      # ★ getCompanyContext — บริษัท + สิทธิ์ (cache ต่อ request)
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
@theme { --color-primary: #1f4fd1; }
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
- บริษัท + สิทธิ์โหลดด้วย `getCompanyContext(companyId)`: 2 คำขอพร้อมกัน (`/companies/{id}/access`, `/companies/{id}`) ครั้งเดียวต่อ request (React `cache`) แม้ layout / page เรียกซ้ำ
  - สมาชิกที่ไม่มีสิทธิ์ `organization.company:read` เข้าได้แต่ไม่เห็นชื่อบริษัท (ถ้าต้องการ ควรให้ `/access` ส่งชื่อบริษัทมาด้วย)
- เมนูที่ยังไม่มีหน้าจริงแสดง "หน้านี้กำลังพัฒนา"

### หน้ารายการ (แม่แบบ: สาขา `organization/branches`)

| ส่วน | ทำอย่างไร |
|---|---|
| อ่านข้อมูล | server component: `pickQuery(searchParams, KEYS)` → `backendGet(path, query)` — ส่งเงื่อนไขจาก URL ให้ API ตรงๆ คิวรี 1 ครั้ง |
| ค้นหา / กรอง | `<Form action="">` ของ `next/form` (GET) — เขียนลง URL, กด Back ได้เงื่อนไขเดิม; ส่ง `sort` / `page_size` เดิมเป็น hidden |
| เรียง | `DataTable` หัวคอลัมน์เป็นลิงก์ `?sort=key` / `?sort=-key` (กลับหน้า 1) |
| แบ่งหน้า | `Pagination` ลิงก์ `?page=` จาก `meta` ของ API |
| เพิ่ม / แก้ไข | หน้าแยก `/new`, `/{id}` — ฟอร์มเป็น client component เรียก `/api/v1/*` (POST / PUT / DELETE) แล้ว `router.push(listHref)` + `router.refresh()` |
| กลับหน้ารายการเดิม | ลิงก์แถวพา `?back=<query ของรายการ>`; หน้าแก้ไขกรองด้วย `pickQuery` อีกชั้นก่อนใช้ |
| สิทธิ์ | ปุ่ม "สร้าง" / บันทึก / ลบ แสดงเมื่อ `can(permissions, '...:manage')`; ไม่มีสิทธิ์ = ฟอร์มอ่านอย่างเดียว |

เพิ่มหน้าจริงให้แอปใหม่: สร้างโฟลเดอร์ `c/[companyId]/<app>/` ที่มี `layout.tsx` (AppShell), `page.tsx` (`redirectToFirstMenu`), `[...rest]/page.tsx` (`ComingSoon`) แบบเดียวกับ `organization/` แล้วเพิ่มหน้าของเมนูนั้น

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

## 8.7 เวอร์ชัน

ล็อกเวอร์ชันตรง (ไม่มี `^`):
- Next.js 16.4, React 19.3, TypeScript 5.9, Tailwind 4.3, Vitest 3.2, lucide-react 1.69 (ไอคอน)
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

