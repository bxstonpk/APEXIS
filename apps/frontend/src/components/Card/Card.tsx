import type { ReactNode } from 'react'

export type CardProps = {
  title?: string
  description?: string
  children: ReactNode
}

/** กรอบเนื้อหาพร้อมหัวข้อ / คำอธิบาย (ข้อความมาจาก config ของผู้ใช้ component) */
export function Card({ title, description, children }: CardProps) {
  return (
    <section className="w-full rounded-(--radius-panel) border border-line bg-surface p-6 shadow-card sm:p-7">
      {(title || description) && (
        <header className="mb-6">
          {title && <h1 className="text-xl leading-tight font-semibold tracking-tight">{title}</h1>}
          {description && <p className="mt-1.5 text-sm text-ink-muted">{description}</p>}
        </header>
      )}
      {children}
    </section>
  )
}
