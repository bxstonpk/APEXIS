import type { ReactNode } from 'react'

export type CardProps = {
  title?: string
  description?: string
  children: ReactNode
}

/** กรอบเนื้อหาพร้อมหัวข้อ / คำอธิบาย (ข้อความมาจาก config ของผู้ใช้ component) */
export function Card({ title, description, children }: CardProps) {
  return (
    <section className="w-full rounded-2xl border border-line bg-surface p-6 shadow-card sm:p-8">
      {(title || description) && (
        <header className="mb-6">
          {title && <h1 className="mb-1 text-2xl leading-tight font-semibold">{title}</h1>}
          {description && <p className="text-ink-muted">{description}</p>}
        </header>
      )}
      {children}
    </section>
  )
}
