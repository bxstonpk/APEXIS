'use client'

import { useId, type InputHTMLAttributes, type ReactNode, type Ref } from 'react'
import { cx } from '@/lib/cx'

export type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: string
  /** ข้อความ error ของช่องนี้ (มาจาก config) — มีค่า = แสดงสถานะผิด */
  error?: string
  hint?: string
  /** ปุ่ม / ไอคอนท้ายช่อง เช่น ปุ่มแสดงรหัสผ่าน */
  endAdornment?: ReactNode
  ref?: Ref<HTMLInputElement>
}

/** ช่องกรอกข้อความพร้อม label / hint / error ที่ผูก aria ให้ครบ */
export function TextField({ label, error, hint, endAdornment, className, ref, ...input }: TextFieldProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className={cx('flex flex-col gap-1', className)}>
      <label htmlFor={id} className="text-sm font-semibold">
        {label}
      </label>
      <div
        className={cx(
          'flex min-h-11 items-center rounded-lg border bg-surface transition-[border-color,box-shadow]',
          'focus-within:border-primary focus-within:ring-3 focus-within:ring-primary/35',
          error ? 'border-danger' : 'border-line-strong',
        )}
      >
        <input
          {...input}
          ref={ref}
          id={id}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className="h-11 min-w-0 flex-1 bg-transparent px-3 text-ink outline-none placeholder:text-ink-muted"
        />
        {endAdornment && <span className="flex pr-1">{endAdornment}</span>}
      </div>
      {hint && (
        <p id={hintId} className="text-sm text-ink-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
