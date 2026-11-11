# 8. Frontend

`apps/frontend` — React 19 + TypeScript + Vite + Tailwind CSS v4 + React Router 7

## 8.1 กติกา

| กติกา | ทำอย่างไร |
|---|---|
| **ทุกอย่างเป็น component** | หน้า (page) ประกอบจาก component; feature component (เช่น `LoginForm`) ไม่ผูกกับ router / layout จึง embed ที่อื่นได้ |
| **ข้อความทั้งหมดอยู่ใน config** | ปุ่ม, label, placeholder, คำอธิบาย, ข้อความ error, ชื่อแท็บ — อยู่ที่ `src/config/locales/th.ts` ที่เดียว ห้ามเขียนข้อความที่ผู้ใช้เห็นลงใน component |
| **CSS ใช้ Tailwind เท่านั้น** | component ใช้ utility class; ไม่มีไฟล์ CSS ของ component |
| **สี / ฟอนต์ / เงา เป็น token** | อยู่ใน `@theme` ของ `src/styles/app.css` ที่เดียว — Tailwind สร้าง class ให้ เช่น `bg-primary`, `text-ink-muted`, `shadow-card` |
| **ค่าตั้งของแอป** | path ของ API / หน้า, ชื่อ query parameter, key ของ storage อยู่ที่ `src/config/app.ts` |
| **โลโก้ / ภาพแบรนด์** | `src/config/brand.ts` ที่เดียว (alt text อยู่ใน `texts.app.logoAlt`) |
| **การค้นหาใช้ URL query** | หน้าใดที่มีการค้นหาเก็บเงื่อนไขไว้ใน URL แล้วส่งต่อให้ API ตรงๆ (ดู [04-api.md §4.2](04-api.md)) — login พากลับหน้าเดิมพร้อม query ผ่าน `?redirect=` |

## 8.2 โครงสร้าง

```text
apps/frontend/
├── index.html
├── vite.config.ts              # Tailwind plugin, alias @ -> src, proxy /api -> backend, ค่าตั้ง test
├── .env.example                # VITE_API_PROXY_TARGET, VITE_API_BASE_URL
└── src/
    ├── main.tsx                # จุดเริ่ม: render <App/> + โหลด styles/app.css
    ├── app/App.tsx             # AuthProvider + router (path จาก appConfig.routes)
    ├── config/
    │   ├── locales/th.ts       # ★ ข้อความทั้งหมดของแอป (ภาษาไทย)
    │   ├── texts.ts            # เลือกภาษา + format('{name}') แทนค่าตัวแปร
    │   ├── app.ts              # ★ ค่าตั้ง: API base / endpoints, routes, storage key
    │   └── brand.ts            # ★ โลโก้ (ไฟล์ + ขนาด)
    ├── assets/brand/           # apexis-logo.webp (ทำจาก docs/brand/ — ดู §8.7)
    ├── styles/app.css          # ★ Tailwind + design tokens (@theme)
    ├── components/             # UI พื้นฐาน ไม่มีข้อความในตัว (รับผ่าน props)
    │   ├── Button  TextField  PasswordField  Alert  Card  Spinner
    ├── features/auth/          # login API, session (localStorage), AuthProvider, RequireAuth, LoginForm
    ├── layouts/AuthLayout.tsx  # โครงหน้าก่อนเข้าสู่ระบบ
    ├── pages/                  # LoginPage, HomePage (หน้าหลักชั่วคราว)
    └── lib/                    # http (เรียก API + แปลง error.code -> ข้อความ), cx, useDocumentTitle
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
- เรียก `POST /api/v1/auth/tokens`
  - สำเร็จ: เก็บ session ใน localStorage แล้วไปหน้าเดิมจาก `?redirect=` (รับเฉพาะ path ภายในแอป กัน open redirect) หรือหน้าหลัก
  - ผิดพลาด: แสดงข้อความจาก `texts.errors[error.code]` (ไม่รู้จัก = `unknown`), ล้างช่องรหัสผ่าน
- หน้าที่ต้องเข้าสู่ระบบครอบด้วย `<RequireAuth>`; token หมดอายุ = ออกจากระบบอัตโนมัติ
- เข้าถึงได้ (a11y): label ผูก input, `aria-invalid` / `aria-describedby`, error แจ้งด้วย `role="alert"`

ข้อควรรู้: token เก็บใน localStorage (อยู่ต่อหลัง refresh) — ถ้าต้องการกันการขโมย token จาก XSS เต็มรูปแบบ ต้องเปลี่ยน backend ให้ออก httpOnly cookie

## 8.5 รัน / ทดสอบ

```bash
cd apps/frontend
npm install
cp .env.example .env.local      # ตั้ง VITE_API_PROXY_TARGET ถ้า backend ไม่ได้อยู่ที่ 127.0.0.1:8080
npm run dev                     # http://localhost:5173
npm test                        # vitest + Testing Library (jsdom)
npm run typecheck
npm run build                   # typecheck + build ไป dist/
```

test ที่มี: `features/auth/LoginForm.test.tsx` (ข้อความจาก config, override ข้อความ, validation, สำเร็จ, รหัสผ่านผิด, network error, error code ไม่รู้จัก, แสดง / ซ่อนรหัสผ่าน) และ `app/App.test.tsx` (redirect ไป login แล้วกลับหน้าเดิมพร้อม query, logout, session หมดอายุ, กัน open redirect, `format`)

## 8.6 เวอร์ชัน

ล็อกเวอร์ชันตรง (ไม่มี `^`):
- React 19.3, React Router 7.18, Vite 7.3, TypeScript 5.9, Tailwind 4.3, Vitest 3.2
- เลือก major ที่ตรวจสอบ API ได้แล้ว (มี Vite 8 / TS 7 / React Router 8 ออกแล้ว) — การอัปเกรด major ควรทำเป็นงานแยก

## 8.7 โลโก้

- ต้นฉบับ: [`docs/brand/Apexis_Enterprise_Growth_Logo.png`](brand/Apexis_Enterprise_Growth_Logo.png)
  - 1536×1024, พื้นหลังโปร่งใส, 1.2 MB — เก็บไว้เป็นต้นฉบับ ไม่ได้ใช้บนเว็บโดยตรง
- ไฟล์บนเว็บ: `apps/frontend/src/assets/brand/apexis-logo.webp`
  - ตัดขอบโปร่งใสแล้วย่อเป็น WebP 640×358, ~45 KB
  - แสดงที่ความกว้าง 224–288px (2× สำหรับจอความละเอียดสูง)
- favicon: `public/favicon-32.png` และ `public/apple-touch-icon.png` (180×180 พื้นขาว — iOS ไม่รองรับพื้นโปร่งใส) ทำจากสัญลักษณ์ "A"
- โลโก้ออกแบบมาสำหรับพื้นสว่าง — ตัวอักษร "APEXIS" สีกรมท่าจะจมบนพื้นมืด ถ้าจะทำ dark mode ต้องมีโลโก้อีกเวอร์ชัน
- เปลี่ยนโลโก้: สร้างไฟล์ใหม่จากต้นฉบับ แล้วแก้ `src/config/brand.ts` ที่เดียว (หน้าที่ใช้ `AuthLayout` เปลี่ยนตาม)

