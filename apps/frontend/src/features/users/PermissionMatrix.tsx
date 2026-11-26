'use client'

import { useEffect, useRef } from 'react'
import { format, texts } from '@/config'
import { cx } from '@/lib/cx'
import { toggle, toggleGroup, type MatrixGroup } from './matrix'

const t = texts.permissions
const label = (map: Record<string, string>, key: string) => map[key] ?? key

/**
 * ตารางสิทธิ์: กลุ่มละ module — แถว = ข้อมูล, คอลัมน์ = การกระทำ
 * ทุกช่องเป็น checkbox จริง (ใช้คีย์บอร์ด / screen reader ได้) ชื่อช่องบอกครบ "การกระทำ + ข้อมูล"
 * ช่องที่ไม่มีสิทธิ์นั้นแสดง "–" (ไม่ใช่ช่องที่ถูกปิด)
 */
export function PermissionMatrix({
  groups,
  selected,
  onChange,
  disabled,
}: {
  groups: MatrixGroup[]
  selected: ReadonlySet<string>
  onChange: (next: Set<string>) => void
  disabled?: boolean
}) {
  return (
    <div className="flex flex-col gap-4">
      {groups.map((g) => {
        const count = g.codes.filter((c) => selected.has(c)).length
        const moduleName = label(t.modules, g.module)
        return (
          <section key={g.module} aria-label={moduleName} className="overflow-hidden rounded-(--radius-panel) border border-line bg-surface shadow-card">
            <header className="flex flex-wrap items-center justify-between gap-2 border-b border-line bg-subtle px-4 py-2.5">
              <GroupCheckbox
                label={format(texts.roles.matrix.selectApp, { app: moduleName })}
                text={moduleName}
                checked={count === g.codes.length}
                indeterminate={count > 0 && count < g.codes.length}
                disabled={disabled}
                onChange={(on) => onChange(toggleGroup(selected, g, on))}
              />
              <span className="text-[13px] text-ink-muted tabular-nums">{format(texts.roles.matrix.selected, { n: count, total: g.codes.length })}</span>
            </header>
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-sm">
                <thead className="text-[13px] text-ink-muted">
                  <tr>
                    <th scope="col" className="h-9 border-b border-line px-4 text-left font-medium">
                      {texts.roles.matrix.resource}
                    </th>
                    {g.actions.map((a) => (
                      <th key={a} scope="col" className="h-9 w-28 border-b border-line px-2 text-center font-medium">
                        {label(t.actions, a)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {g.rows.map((row) => {
                    const resourceName = label(t.resources, row.key)
                    return (
                      <tr key={row.key} className="border-b border-line last:border-b-0 hover:bg-subtle">
                        <th scope="row" className="h-11 px-4 text-left font-normal">
                          {resourceName}
                        </th>
                        {g.actions.map((a) => {
                          const code = row.cells[a]
                          return (
                            <td key={a} className="text-center">
                              {code ? (
                                <input
                                  type="checkbox"
                                  aria-label={`${label(t.actions, a)} ${resourceName}`}
                                  checked={selected.has(code)}
                                  disabled={disabled}
                                  onChange={(e) => onChange(toggle(selected, row, a, e.target.checked))}
                                  className={cx('size-4 cursor-pointer accent-primary disabled:cursor-not-allowed')}
                                />
                              ) : (
                                <span aria-hidden className="text-ink-subtle">
                                  {texts.roles.matrix.none}
                                </span>
                              )}
                            </td>
                          )
                        })}
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </section>
        )
      })}
    </div>
  )
}

/** checkbox "เลือกทั้งหมด" ที่บอกสถานะ "เลือกบางส่วน" (indeterminate) ได้ */
function GroupCheckbox({
  label: aria,
  text,
  checked,
  indeterminate,
  disabled,
  onChange,
}: {
  label: string
  text: string
  checked: boolean
  indeterminate: boolean
  disabled?: boolean
  onChange: (on: boolean) => void
}) {
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate
  }, [indeterminate])
  return (
    <label className="flex cursor-pointer items-center gap-2.5 text-[15px] font-semibold">
      <input
        ref={ref}
        type="checkbox"
        aria-label={aria}
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="size-4 cursor-pointer accent-primary disabled:cursor-not-allowed"
      />
      {text}
    </label>
  )
}
