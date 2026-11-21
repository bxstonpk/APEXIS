import { Construction, Info, ShieldAlert } from 'lucide-react'
import type { ReactNode } from 'react'

const icons = { info: Info, forbidden: ShieldAlert, soon: Construction } as const

/** ข้อความกลางหน้า เช่น ไม่มีสิทธิ์ / กำลังพัฒนา / เปิดบริษัทไม่ได้ */
export function MessagePanel({
  title,
  description,
  action,
  icon = 'info',
}: {
  title: string
  description?: string
  action?: ReactNode
  icon?: keyof typeof icons
}) {
  const Icon = icons[icon]
  return (
    <section className="mx-auto mt-16 flex max-w-md flex-col items-center px-4 text-center">
      <span className="flex size-12 items-center justify-center rounded-full border border-line bg-surface text-ink-muted shadow-card">
        <Icon aria-hidden className="size-5" />
      </span>
      <h1 className="mt-4 text-lg font-semibold">{title}</h1>
      {description && <p className="mt-1.5 text-sm text-ink-muted">{description}</p>}
      {action && <div className="mt-6 flex justify-center">{action}</div>}
    </section>
  )
}
