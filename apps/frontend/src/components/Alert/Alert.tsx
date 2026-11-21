import { CircleAlert, Info } from 'lucide-react'
import type { ReactNode } from 'react'
import { cx } from '@/lib/cx'

export type AlertProps = {
  tone?: 'error' | 'info'
  children: ReactNode
}

const tones = {
  error: { box: 'border-danger-line bg-danger-soft text-danger', Icon: CircleAlert },
  info: { box: 'border-info-line bg-info-soft text-ink', Icon: Info },
} as const

/** กล่องข้อความแจ้งเตือน — tone error ประกาศให้ screen reader ทันที (role="alert") */
export function Alert({ tone = 'info', children }: AlertProps) {
  const { box, Icon } = tones[tone]
  return (
    <div role={tone === 'error' ? 'alert' : 'status'} className={cx('flex gap-2.5 rounded-(--radius-control) border px-3.5 py-2.5 text-sm', box)}>
      <Icon aria-hidden className={cx('mt-0.5 size-4 shrink-0', tone === 'info' && 'text-primary')} />
      <div className="min-w-0 whitespace-pre-line">{children}</div>
    </div>
  )
}
