import type { ReactNode } from 'react'
import { brand, format, texts } from '@/config'

/**
 * โครงหน้าก่อนเข้าสู่ระบบ
 * - จอกว้าง: แบ่งครึ่ง — ซ้าย = แผงสีกรมท่า (สีเดียวกับพื้นภาพ) มีแบนเนอร์จัตุรัส (docs/brand/banners/login-hero/glass-1080x1080) อยู่กลางแผง, ขวา = โลโก้ + ฟอร์ม
 *   ขนาด = ครึ่งทางระหว่าง min(72%, 72vh, 640px) กับด้านที่สั้นกว่าของแผง; ขอบภาพจางเข้าพื้นทั้ง 4 ด้าน (EDGE_FADE) จึงกลืนเป็นชิ้นเดียวกับแผง
 * - จอแคบ: โลโก้ + ฟอร์ม (ไม่โหลดภาพ — ซ่อนด้วย CSS และภาพเป็น loading="lazy")
 */
export function AuthLayout({ children }: { children: ReactNode }) {
  const year = new Date().getFullYear()
  const hero = brand.loginHero

  return (
    <div className="grid min-h-full bg-surface lg:grid-cols-2">
      {/* แผงแบรนด์ */}
      <aside className="relative hidden items-center justify-center overflow-hidden lg:flex" style={{ backgroundColor: hero.background }}>
        {/* แสงฟุ้งรอบแบนเนอร์ ให้แผงไม่เรียบเกินไป */}
        <div aria-hidden className="pointer-events-none absolute inset-0">
          <div className="absolute -top-32 -left-32 size-[30rem] rounded-full bg-primary/40 blur-3xl" />
          <div className="absolute -right-24 -bottom-32 size-[26rem] rounded-full bg-accent/20 blur-3xl" />
        </div>
        {/* eslint-disable-next-line @next/next/no-img-element -- ภาพ WebP ที่ย่อไว้แล้ว ไม่ต้องผ่าน image optimizer */}
        <img
          src={hero.src}
          width={hero.width}
          height={hero.height}
          alt={texts.app.loginHeroAlt}
          loading="lazy"
          decoding="async"
          className="relative aspect-square w-[calc((min(72%,72vh,640px)+min(100%,100vh))/2)]"
          style={{ maskImage: EDGE_FADE, WebkitMaskImage: EDGE_FADE, maskComposite: 'intersect', WebkitMaskComposite: 'source-in' }}
        />
        <p className="absolute bottom-8 left-12 text-[13px] text-white/45 xl:left-16">{format(texts.footer.copyright, { year })}</p>
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

/** จางขอบภาพทั้ง 4 ด้าน (6%) ให้กลืนกับพื้นแผง — ข้อความในภาพเริ่มที่ ~9% จากขอบ จึงไม่ถูกจาง */
const EDGE_FADE =
  'linear-gradient(to right, transparent, #000 6%, #000 94%, transparent), linear-gradient(to bottom, transparent, #000 6%, #000 94%, transparent)'
