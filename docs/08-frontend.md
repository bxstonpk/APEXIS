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
├── public/                     # favicon, brand/apexis-logo.webp (ทำจาก docs/brand/ — ดู §8.7)
└── src/
    ├── proxy.ts                # ★ ด่านหน้า: ไม่มี session -> /login?redirect=<หน้าเดิม + query>
    ├── app/
    │   ├── layout.tsx          # <html lang="th">, ชื่อแท็บ "%s | APEXIS", favicon, โหลด styles/app.css
    │   ├── (auth)/login/page.tsx   # หน้า login (เข้าสู่ระบบแล้ว = ไปหน้าเดิม)
    │   ├── (app)/page.tsx      # หน้าหลักชั่วคราว
    │   └── api/
    │       ├── session/route.ts       # ★ POST เข้าสู่ระบบ / GET ผู้ใช้ปัจจุบัน / DELETE ออกจากระบบ
    │       └── v1/[...path]/route.ts  # ★ ส่งต่อไป backend + เติม Bearer จาก cookie
    ├── config/
    │   ├── locales/th.ts       # ★ ข้อความทั้งหมดของแอป (ภาษาไทย)
    │   ├── texts.ts            # เลือกภาษา + format('{name}') แทนค่าตัวแปร
    │   ├── app.ts              # ★ ค่าตั้ง: API path, routes, ชื่อ cookie
    │   └── brand.ts            # ★ โลโก้ (ไฟล์ + ขนาด), favicon
    ├── styles/app.css          # ★ Tailwind + design tokens (@theme)
    ├── components/             # UI พื้นฐาน ไม่มีข้อความในตัว (รับผ่าน props)
    │   ├── Button  TextField  PasswordField  Alert  Card  Spinner
    ├── features/auth/          # login / logout API, LoginForm, LoginPanel, LogoutButton (client)
    ├── layouts/AuthLayout.tsx  # โครงหน้าก่อนเข้าสู่ระบบ
    └── lib/
        ├── http.ts             # เรียก API (same-origin) + แปลง error.code -> ข้อความ
        ├── cx.ts
        └── server/session.ts   # ★ อ่าน / เขียน cookie, backendUrl, safeRedirect, ตรวจ origin
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

## 8.5 รัน / ทดสอบ

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

## 8.6 เวอร์ชัน

ล็อกเวอร์ชันตรง (ไม่มี `^`):
- Next.js 16.4, React 19.3, TypeScript 5.9, Tailwind 4.3, Vitest 3.2
- Next 16 ใช้ `proxy.ts` แทน `middleware.ts` เดิม
- เลือก major ที่ตรวจสอบ API ได้แล้ว (มี TS 7 ออกแล้ว) — การอัปเกรด major ควรทำเป็นงานแยก

## 8.7 โลโก้

- ต้นฉบับ: [`docs/brand/Apexis_Enterprise_Growth_Logo.png`](brand/Apexis_Enterprise_Growth_Logo.png)
  - 1536×1024, พื้นหลังโปร่งใส, 1.2 MB — เก็บไว้เป็นต้นฉบับ ไม่ได้ใช้บนเว็บโดยตรง
- ไฟล์บนเว็บ: `apps/frontend/public/brand/apexis-logo.webp`
  - ตัดขอบโปร่งใสแล้วย่อเป็น WebP 640×358, ~45 KB
  - แสดงที่ความกว้าง 224–288px (2× สำหรับจอความละเอียดสูง)
- favicon: `public/favicon-32.png` และ `public/apple-touch-icon.png` (180×180 พื้นขาว — iOS ไม่รองรับพื้นโปร่งใส) ทำจากสัญลักษณ์ "A"
- โลโก้ออกแบบมาสำหรับพื้นสว่าง — ตัวอักษร "APEXIS" สีกรมท่าจะจมบนพื้นมืด ถ้าจะทำ dark mode ต้องมีโลโก้อีกเวอร์ชัน
- เปลี่ยนโลโก้: สร้างไฟล์ใหม่จากต้นฉบับ แล้วแก้ `src/config/brand.ts` ที่เดียว (หน้าที่ใช้ `AuthLayout` เปลี่ยนตาม)

