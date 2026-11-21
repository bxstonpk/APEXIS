'use client'

import { useId } from 'react'
import { Badge } from '@/components'
import { format, texts } from '@/config'
import { cx } from '@/lib/cx'
import type { Role } from './types'

/** เลือกบทบาทหลายรายการ — checkbox พร้อมชื่อ คำอธิบาย และจำนวนสิทธิ์ (กลุ่มมีชื่อสำหรับ screen reader) */
export function RoleChecklist({
  roles,
  selected,
  onChange,
  disabled,
  error,
}: {
  roles: Role[]
  selected: ReadonlySet<string>
  onChange: (next: Set<string>) => void
  disabled?: boolean
  error?: string
}) {
  const id = useId()
  const t = texts.roles
  if (roles.length === 0) return <p className="text-sm text-ink-muted sm:col-span-2">{texts.members.fields.noRoles}</p>
  return (
    <div role="group" aria-labelledby={`${id}-label`} aria-describedby={error ? `${id}-error` : undefined} className="flex flex-col gap-2 sm:col-span-2">
      <span id={`${id}-label`} className="text-[13px] font-medium">
        {texts.members.fields.roles}
        <span aria-hidden className="ml-0.5 text-danger">
          *
        </span>
      </span>
      <ul className="grid gap-2 sm:grid-cols-2">
        {roles.map((r) => {
          const on = selected.has(r.id)
          return (
            <li key={r.id}>
              <label
                className={cx(
                  'flex h-full cursor-pointer gap-3 rounded-(--radius-control) border px-3.5 py-3 transition-colors',
                  on ? 'border-primary/50 bg-primary-soft/60' : 'border-line hover:bg-subtle',
                  disabled && 'cursor-not-allowed opacity-70',
                )}
              >
                <input
                  type="checkbox"
                  checked={on}
                  disabled={disabled}
                  aria-invalid={error ? true : undefined}
                  onChange={(e) => {
                    const next = new Set(selected)
                    if (e.target.checked) next.add(r.id)
                    else next.delete(r.id)
                    onChange(next)
                  }}
                  className="mt-0.5 size-4 shrink-0 cursor-pointer accent-primary"
                />
                <span className="min-w-0">
                  <span className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    {r.name}
                    {r.is_system && <Badge tone="info">{t.system}</Badge>}
                  </span>
                  <span className="mt-0.5 block text-[13px] text-ink-muted">
                    {r.is_system ? t.systemDescription : r.description || format(t.permissionCount, { n: r.permissions.length })}
                  </span>
                </span>
              </label>
            </li>
          )
        })}
      </ul>
      {error && (
        <p id={`${id}-error`} role="alert" className="text-[13px] text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
