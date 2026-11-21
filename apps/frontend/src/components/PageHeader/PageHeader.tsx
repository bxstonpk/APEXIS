import type { ReactNode } from 'react'

export type PageHeaderProps = {
  title: string
  description?: string
  /** ข้อความเล็กเหนือชื่อหน้า เช่น ชื่อหมวด */
  eyebrow?: string
  /** ปุ่มด้านขวา เช่น "สร้าง" */
  actions?: ReactNode
}

/** หัวหน้าเนื้อหา: (หมวด) ชื่อหน้า + คำอธิบาย + ปุ่ม */
export function PageHeader({ title, description, eyebrow, actions }: PageHeaderProps) {
  return (
    <header className="mb-5 flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
      <div className="min-w-0">
        {eyebrow && <p className="mb-1 text-[13px] font-medium text-ink-muted">{eyebrow}</p>}
        <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{title}</h1>
        {description && <p className="mt-1 text-sm text-ink-muted">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  )
}
