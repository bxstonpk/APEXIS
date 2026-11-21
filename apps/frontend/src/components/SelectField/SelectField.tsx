'use client'

import { ChevronDown } from 'lucide-react'
import { useId, type SelectHTMLAttributes } from 'react'
import { cx } from '@/lib/cx'
import { errorClass, FieldLabel, fieldFrameClass, inputTextClass } from '../TextField/TextField'

export type SelectOption = { value: string; label: string }

export type SelectFieldProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> & {
  label: string
  options: readonly SelectOption[]
  error?: string
  /** ซ่อน label (ยังอ่านได้ด้วย screen reader) — ใช้ในแถบค้นหา */
  hideLabel?: boolean
}

/** ช่องเลือก (select) พร้อม label / error — ตัวเลือกและข้อความมาจาก config ของผู้ใช้ component */
export function SelectField({ label, options, error, hideLabel, className, disabled, required, ...select }: SelectFieldProps) {
  const id = useId()
  const errorId = error ? `${id}-error` : undefined
  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      <FieldLabel htmlFor={id} label={label} required={required} hidden={hideLabel} />
      <div className={cx(fieldFrameClass(error, disabled), 'relative')}>
        <select
          {...select}
          id={id}
          disabled={disabled}
          aria-required={required || undefined}
          aria-invalid={error ? true : undefined}
          aria-describedby={errorId}
          className={cx(inputTextClass, 'h-full w-full cursor-pointer appearance-none bg-transparent pr-9 pl-3 outline-none disabled:cursor-not-allowed disabled:text-ink-muted')}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <ChevronDown aria-hidden className="pointer-events-none absolute right-3 size-4 text-ink-subtle" />
      </div>
      {error && (
        <p id={errorId} className={errorClass}>
          {error}
        </p>
      )}
    </div>
  )
}
