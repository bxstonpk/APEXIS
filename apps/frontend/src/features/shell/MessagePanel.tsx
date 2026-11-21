import type { ReactNode } from 'react'

/** กล่องข้อความกลางหน้า เช่น ไม่มีสิทธิ์ / กำลังพัฒนา / เปิดบริษัทไม่ได้ */
export function MessagePanel({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <section className="mx-auto mt-12 max-w-lg rounded-2xl border border-line bg-surface p-8 text-center shadow-card">
      <h1 className="text-xl font-semibold">{title}</h1>
      {description && <p className="mt-2 text-ink-muted">{description}</p>}
      {action && <div className="mt-6 flex justify-center">{action}</div>}
    </section>
  )
}
