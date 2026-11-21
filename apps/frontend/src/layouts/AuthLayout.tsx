import type { ReactNode } from 'react'
import { brand, format, texts } from '@/config'
import { appTexts, apps } from '@/config/apps'

/**
 * โครงหน้าก่อนเข้าสู่ระบบ
 * - จอกว้าง: ซ้าย = แผงแบรนด์สีกรมท่า (คำโปรย + โมดูลในระบบ + ลวดลายเส้นโค้งสีส้มและกราฟแท่งจากโลโก้), ขวา = โลโก้ + ฟอร์ม
 * - จอแคบ: โลโก้ + ฟอร์ม (ไม่มีแผงแบรนด์)
 * รายชื่อโมดูลมาจาก config/apps.ts (ชุดเดียวกับหน้ารวมแอป) — ไม่มีข้อความเขียนตายใน component
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  const year = new Date().getFullYear()
  const modules = apps.filter((a) => a.key !== 'users')

  return (
    <div className="grid min-h-full bg-surface lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
      {/* แผงแบรนด์ */}
      <aside className="relative hidden overflow-hidden bg-nav text-white lg:flex lg:flex-col lg:justify-between lg:p-12 xl:p-16">
        <BrandArt />

        <p className="relative text-[13px] font-medium tracking-[0.2em] text-white/55">APEXIS · ERP</p>

        <div className="relative max-w-md">
          <span aria-hidden className="mb-6 block h-1 w-12 rounded-full bg-accent" />
          <h2 className="text-[34px] leading-[1.25] font-semibold tracking-tight text-balance">{texts.app.tagline}</h2>
          <p className="mt-4 text-[15px] leading-relaxed text-white/70">{texts.app.pitch}</p>

          <p className="mt-10 mb-3 text-[13px] font-medium text-white/50">{texts.app.modulesLabel}</p>
          <ul className="grid grid-cols-2 gap-2">
            {modules.map((app) => {
              const Icon = app.icon
              return (
                <li key={app.key} className="flex items-center gap-2.5 rounded-lg border border-white/10 bg-nav/70 px-3 py-2.5 backdrop-blur-sm">
                  <span className={`flex size-7 shrink-0 items-center justify-center rounded-md ${app.tone}`}>
                    <Icon aria-hidden className="size-4" strokeWidth={2} />
                  </span>
                  <span className="text-sm text-white/85">{appTexts(app.key).name}</span>
                </li>
              )
            })}
          </ul>
        </div>

        <p className="relative text-[13px] text-white/45">{format(texts.footer.copyright, { year })}</p>
      </aside>

      {/* ฟอร์ม */}
      <div className="flex min-h-full flex-col px-6 py-10 sm:px-10">
        <div className="flex flex-1 flex-col justify-center">
          <div className="mx-auto w-full max-w-[380px]">
            {/* eslint-disable-next-line @next/next/no-img-element -- โลโก้เป็น WebP ที่ย่อไว้แล้ว ไม่ต้องผ่าน image optimizer */}
            <img
              src={brand.logo.src}
              width={brand.logo.width}
              height={brand.logo.height}
              alt={texts.app.logoAlt}
              className="mb-10 h-auto w-40"
              decoding="async"
              fetchPriority="high"
            />
            <main id="main" tabIndex={-1} className="outline-none">
              {children}
            </main>
          </div>
        </div>
        <p className="mx-auto w-full max-w-[380px] pt-8 text-[13px] text-ink-muted lg:hidden">{format(texts.footer.copyright, { year })}</p>
      </div>
    </div>
  )
}

/** ลวดลายตกแต่งของแผงแบรนด์: แสงฟุ้ง + ตารางจุด + เส้นโค้งสีส้ม (ล้อเส้นวงโคจรในโลโก้) + กราฟแท่ง (ล้อกราฟในโลโก้) */
function BrandArt() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0">
      <div className="absolute -top-40 -left-40 size-[36rem] rounded-full bg-primary/50 blur-3xl" />
      <div className="absolute -right-32 -bottom-48 size-[28rem] rounded-full bg-accent/15 blur-3xl" />
      <div className="absolute inset-0 [background-image:radial-gradient(rgb(255_255_255/0.09)_1px,transparent_1px)] [background-size:22px_22px] [mask-image:linear-gradient(to_bottom,transparent,black_30%,black_70%,transparent)]" />
      <svg className="absolute right-0 bottom-0 h-[48%] w-full" viewBox="0 0 600 500" preserveAspectRatio="xMaxYMax meet" fill="none">
        {/* กราฟแท่ง */}
        {[0, 1, 2, 3, 4].map((i) => (
          <rect key={i} x={360 + i * 44} y={420 - (i + 1) * 46} width="28" height={(i + 1) * 46 + 80} rx="3" fill="white" fillOpacity={0.035 + i * 0.012} />
        ))}
        {/* เส้นโค้งสีส้ม */}
        <path d="M-20 470 C 160 330, 380 250, 640 210" stroke="var(--color-accent)" strokeWidth="2.5" strokeLinecap="round" strokeOpacity="0.9" />
        <path d="M-20 495 C 170 365, 390 290, 640 255" stroke="var(--color-accent)" strokeWidth="1" strokeLinecap="round" strokeOpacity="0.35" />
      </svg>
    </div>
  )
}
