import type { ReactNode } from 'react'

export type PageHeaderProps = {
  title: string
  description?: string
  /** ปุ่มด้านขวา เช่น "สร้าง" */
  actions?: ReactNode
}

/** หัวหน้าเนื้อหา: ชื่อหน้า + คำอธิบาย + ปุ่ม */
export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-2xl leading-tight font-semibold">{title}</h1>
        {description && <p className="mt-1 text-ink-muted">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  )
}
