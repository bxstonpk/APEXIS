import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { brand, texts } from '@/config'
import '@/styles/app.css'

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
      <body>{children}</body>
    </html>
  )
}
