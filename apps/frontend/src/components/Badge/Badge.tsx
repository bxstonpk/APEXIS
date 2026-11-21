import type { ReactNode } from 'react'
import { cx } from '@/lib/cx'

const tones = {
  success: { box: 'bg-success-soft text-success', dot: 'bg-success' },
  neutral: { box: 'bg-neutral-soft text-ink-muted', dot: 'bg-ink-subtle' },
  info: { box: 'bg-info-soft text-primary', dot: 'bg-primary' },
  warning: { box: 'bg-warning-soft text-warning', dot: 'bg-warning' },
} as const

export type BadgeProps = { tone?: keyof typeof tones; children: ReactNode; /** แสดงจุดสีหน้าข้อความ (ใช้กับสถานะ) */ dot?: boolean }

/** ป้ายสั้นๆ เช่น สถานะ "ใช้งาน" / "สำนักงานใหญ่" */
export function Badge({ tone = 'neutral', dot = false, children }: BadgeProps) {
  const t = tones[tone]
  return (
    <span className={cx('inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap', t.box)}>
      {dot && <span aria-hidden className={cx('size-1.5 rounded-full', t.dot)} />}
      {children}
    </span>
  )
}
