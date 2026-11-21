'use client'

import { useId, type InputHTMLAttributes, type ReactNode, type Ref } from 'react'
import { cx } from '@/lib/cx'

export type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: string
  /** ข้อความ error ของช่องนี้ (มาจาก config) — มีค่า = แสดงสถานะผิด */
  error?: string
  hint?: string
  /** ไอคอนหน้าช่อง เช่น แว่นขยายของช่องค้นหา */
  startAdornment?: ReactNode
  /** ปุ่ม / ไอคอนท้ายช่อง เช่น ปุ่มแสดงรหัสผ่าน */
  endAdornment?: ReactNode
  /** ซ่อน label (ยังอ่านได้ด้วย screen reader) */
  hideLabel?: boolean
  ref?: Ref<HTMLInputElement>
}

/** กรอบของช่องกรอกทุกชนิด (ใช้ร่วม: TextField, SelectField, ReferenceSelect) */
export function fieldFrameClass(error?: string, disabled?: boolean): string {
  return cx(
    'flex h-10 items-center rounded-(--radius-control) border bg-surface shadow-card transition-[border-color,box-shadow]',
    'focus-within:border-primary focus-within:ring-3 focus-within:ring-primary/15',
    error ? 'border-danger focus-within:border-danger focus-within:ring-danger/15' : 'border-line-strong hover:border-ink-subtle',
    disabled && 'bg-subtle shadow-none hover:border-line-strong',
  )
}

export const labelClass = 'text-[13px] font-medium text-ink'

/**
 * label ของช่อง + เครื่องหมาย * สำหรับช่องบังคับ
 * * อยู่นอก <label> และซ่อนจาก screen reader — ชื่อของช่องคงเป็นข้อความ label พอดี (ความบังคับบอกด้วย aria-required)
 */
export function FieldLabel({ htmlFor, label, required, hidden }: { htmlFor: string; label: string; required?: boolean; hidden?: boolean }) {
  return (
    <span className={cx('flex items-baseline gap-0.5', hidden && 'sr-only')}>
      <label htmlFor={htmlFor} className={labelClass}>
        {label}
      </label>
      {required && (
        <span aria-hidden className="text-[13px] text-danger">
          *
        </span>
      )}
    </span>
  )
}

/** ขนาดตัวอักษรในช่อง: 16px บนมือถือ (iOS ไม่ซูมหน้าเอง), 14px บนจอกว้าง */
export const inputTextClass = 'text-base text-ink sm:text-sm'
export const hintClass = 'text-[13px] leading-snug text-ink-muted'
export const errorClass = 'text-[13px] leading-snug text-danger'

/** ช่องกรอกข้อความพร้อม label / hint / error ที่ผูก aria ให้ครบ */
export function TextField({ label, error, hint, startAdornment, endAdornment, hideLabel, className, ref, disabled, required, ...input }: TextFieldProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      <FieldLabel htmlFor={id} label={label} required={required} hidden={hideLabel} />
      <div className={fieldFrameClass(error, disabled)}>
        {startAdornment && <span className="flex pl-3 text-ink-subtle [&_svg]:size-4">{startAdornment}</span>}
        <input
          {...input}
          ref={ref}
          id={id}
          disabled={disabled}
          aria-required={required || undefined}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cx(inputTextClass, 'h-full min-w-0 flex-1 bg-transparent px-3 outline-none placeholder:text-ink-subtle disabled:cursor-not-allowed disabled:text-ink-muted')}
        />
        {endAdornment && <span className="flex pr-1">{endAdornment}</span>}
      </div>
      {hint && !error && (
        <p id={hintId} className={hintClass}>
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className={errorClass}>
          {error}
        </p>
      )}
    </div>
  )
}
