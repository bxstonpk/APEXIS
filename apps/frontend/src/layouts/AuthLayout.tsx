import type { ReactNode } from 'react'
import { brand, format, texts } from '@/config'

/** โครงหน้าสำหรับหน้าที่ยังไม่เข้าสู่ระบบ: โลโก้ด้านบน, เนื้อหากลางจอ, footer */
export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-full flex-col items-center justify-center gap-8 px-4 py-12">
      <header className="flex flex-col items-center text-center">
        {/* eslint-disable-next-line @next/next/no-img-element -- โลโก้เป็น WebP ที่ย่อไว้แล้ว ไม่ต้องผ่าน image optimizer */}
        <img
          src={brand.logo.src}
          width={brand.logo.width}
          height={brand.logo.height}
          alt={texts.app.logoAlt}
          className="h-auto w-56 sm:w-72"
          decoding="async"
          fetchPriority="high"
        />
        <p className="mt-2 text-sm text-ink-muted">{texts.app.tagline}</p>
      </header>
      <main className="w-full max-w-md">{children}</main>
      <footer className="text-sm text-ink-muted">{format(texts.footer.copyright, { year: new Date().getFullYear() })}</footer>
    </div>
  )
}
