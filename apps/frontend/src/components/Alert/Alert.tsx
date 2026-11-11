import type { ReactNode } from 'react'
import { cx } from '@/lib/cx'

export type AlertProps = {
  tone?: 'error' | 'info'
  children: ReactNode
}

const tones = {
  error: 'border-danger-line bg-danger-soft text-danger',
  info: 'border-info-line bg-info-soft text-ink',
} as const

/** กล่องข้อความแจ้งเตือน — tone error ประกาศให้ screen reader ทันที (role="alert") */
export function Alert({ tone = 'info', children }: AlertProps) {
  return (
    <div role={tone === 'error' ? 'alert' : 'status'} className={cx('rounded-lg border px-4 py-3 text-sm', tones[tone])}>
      {children}
    </div>
  )
}
