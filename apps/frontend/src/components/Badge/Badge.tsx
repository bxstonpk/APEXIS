import type { ReactNode } from 'react'
import { cx } from '@/lib/cx'

const tones = {
  success: 'bg-success-soft text-success',
  neutral: 'bg-neutral-soft text-ink-muted',
  info: 'bg-info-soft text-primary',
} as const

export type BadgeProps = { tone?: keyof typeof tones; children: ReactNode }

/** ป้ายสถานะสั้นๆ เช่น "ใช้งาน" / "สำนักงานใหญ่" */
export function Badge({ tone = 'neutral', children }: BadgeProps) {
  return (
    <span className={cx('inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap', tones[tone])}>
      {children}
    </span>
  )
}
