'use client'

import { Check } from 'lucide-react'
import { useId } from 'react'
import { texts } from '@/config'
import { BUSINESS_TYPES, businessTypeIcons, type BusinessType } from '@/config/businessTypes'
import { cx } from '@/lib/cx'

/**
 * เลือกประเภทธุรกิจ (radio cards) — fieldset + legend + radio จริง จึงใช้คีย์บอร์ดได้ตามปกติ (Tab เข้ากลุ่ม, ลูกศรเลื่อนตัวเลือก)
 * ข้อผิดพลาดอยู่ใต้กลุ่มและผูกด้วย aria-describedby; radio แรกได้ aria-invalid เพื่อให้ focusFirstInvalid โฟกัสถูกที่
 */
export function BusinessTypePicker({
  value,
  onChange,
  error,
  label,
}: {
  value: BusinessType | ''
  onChange: (value: BusinessType) => void
  error?: string
  label: string
}) {
  const id = useId()
  const errorId = `${id}-error`
  const t = texts.businessTypes
  return (
    <fieldset aria-describedby={error ? errorId : undefined} className="flex flex-col gap-2">
      <legend className="mb-1.5 text-[13px] font-medium text-ink">
        {label}
        <span aria-hidden className="ml-0.5 text-danger">
          *
        </span>
      </legend>
      <div className="grid gap-2 sm:grid-cols-2">
        {BUSINESS_TYPES.map((type, i) => {
          const Icon = businessTypeIcons[type]
          const checked = value === type
          return (
            <label
              key={type}
              className={cx(
                'relative flex cursor-pointer items-center gap-3 rounded-(--radius-control) border p-3 transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-primary has-[:focus-visible]:ring-offset-2',
                checked ? 'border-primary bg-primary-soft' : error ? 'border-danger' : 'border-line hover:border-ink-subtle',
              )}
            >
              <input
                type="radio"
                name={`${id}-business-type`}
                value={type}
                checked={checked}
                required
                aria-invalid={i === 0 && error ? true : undefined}
                onChange={() => onChange(type)}
                className="sr-only"
              />
              <span className={cx('flex size-9 shrink-0 items-center justify-center rounded-md', checked ? 'bg-primary text-white' : 'bg-neutral-soft text-ink-muted')}>
                <Icon aria-hidden className="size-4.5" />
              </span>
              <span className="min-w-0 pr-5 text-sm font-medium">{t.names[type]}</span>
              {checked && <Check aria-hidden className="absolute top-1/2 right-3 size-4 -translate-y-1/2 text-primary" />}
            </label>
          )
        })}
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-[13px] font-medium text-danger">
          {error}
        </p>
      )}
    </fieldset>
  )
}
