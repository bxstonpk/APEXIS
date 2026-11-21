import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { brand, texts } from '@/config'
import '@fontsource/ibm-plex-sans-thai/thai-400.css'
import '@fontsource/ibm-plex-sans-thai/thai-500.css'
import '@fontsource/ibm-plex-sans-thai/thai-600.css'
import '@fontsource/ibm-plex-sans-thai/latin-400.css'
import '@fontsource/ibm-plex-sans-thai/latin-500.css'
import '@fontsource/ibm-plex-sans-thai/latin-600.css'
import '@/styles/app.css'
import { Toaster } from '@/features/shell/Toaster'

/** ชื่อแท็บ: "<ชื่อหน้า> | <ชื่อแอป>" — แต่ละหน้าตั้ง title ของตัวเองจาก config */
export const metadata: Metadata = {
  title: { template: `%s | ${texts.app.name}`, default: texts.app.name },
  description: texts.app.tagline,
  icons: {
    icon: [{ url: brand.icons.favicon, sizes: '32x32', type: 'image/png' }],
    apple: brand.icons.appleTouch,
  },
}

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="th">
      <body>
        {/* ผู้ใช้คีย์บอร์ด: กด Tab ครั้งแรกเห็นลิงก์นี้ ข้ามแถบเมนูไปเนื้อหาได้ทันที */}
        <a
          href="#main"
          className="sr-only z-50 rounded-(--radius-control) bg-primary px-4 py-2 text-sm font-medium text-on-primary focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
        >
          {texts.common.skipToContent}
        </a>
        {children}
        <Toaster />
      </body>
    </html>
  )
}
