import type { ReactNode } from 'react'

export type CardProps = {
  title?: string
  children: ReactNode
}

/** กรอบเนื้อหาพร้อมหัวข้อ (ข้อความมาจาก config ของผู้ใช้ component) — ไม่มีคำอธิบาย (หน้าจอไม่อธิบาย) */
export function Card({ title, children }: CardProps) {
  return (
    <section className="w-full rounded-(--radius-panel) border border-line bg-surface p-6 shadow-card sm:p-7">
      {title && (
        <header className="mb-6">
          <h1 className="text-xl leading-tight font-semibold tracking-tight">{title}</h1>
        </header>
      )}
      {children}
    </section>
  )
}
