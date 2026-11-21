'use client'

import { useId, type SelectHTMLAttributes } from 'react'
import { cx } from '@/lib/cx'

export type SelectOption = { value: string; label: string }

export type SelectFieldProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> & {
  label: string
  options: readonly SelectOption[]
  error?: string
  /** ซ่อน label (ยังอ่านได้ด้วย screen reader) — ใช้ในแถบค้นหา */
  hideLabel?: boolean
}

/** ช่องเลือก (select) พร้อม label / error — ตัวเลือกและข้อความมาจาก config ของผู้ใช้ component */
export function SelectField({ label, options, error, hideLabel, className, ...select }: SelectFieldProps) {
  const id = useId()
  const errorId = error ? `${id}-error` : undefined
  return (
    <div className={cx('flex flex-col gap-1', className)}>
      <label htmlFor={id} className={cx('text-sm font-semibold', hideLabel && 'sr-only')}>
        {label}
      </label>
      <select
        {...select}
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={errorId}
        className={cx(
          'h-11 rounded-lg border bg-surface px-3 text-ink outline-none transition-[border-color,box-shadow]',
          'focus:border-primary focus:ring-3 focus:ring-primary/35 disabled:bg-canvas disabled:text-ink-muted',
          error ? 'border-danger' : 'border-line-strong',
        )}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      {error && (
        <p id={errorId} className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
