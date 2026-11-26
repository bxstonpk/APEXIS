'use client'

import { X } from 'lucide-react'
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { apiRequest } from '@/lib/http'
import { cx } from '@/lib/cx'
import { errorClass, FieldLabel, fieldFrameClass, inputTextClass } from '../TextField/TextField'

export type RefOption = { id: string; code: string; name: string }

export type ReferenceSelectProps = {
  label: string
  /** API ที่ค้นหา (ต่อจาก /api/v1) เช่น /companies/{id}/departments — ส่ง ?q=&page_size= ให้เอง */
  source: string
  /** query เพิ่มเติม เช่น { status: 'active' } */
  params?: Record<string, string>
  /** ค่าเริ่มต้น (ไม่ควบคุม) หรือค่าปัจจุบัน (ควบคุม — ใช้คู่กับ onChange) */
  defaultValue?: RefOption | null
  value?: RefOption | null
  onChange?: (value: RefOption | null) => void
  /** มีค่า = ส่งค่า id ในฟอร์ม (เช่น ตัวกรองใน URL ?branch_id=) */
  name?: string
  /** ไม่แสดง id นี้ในตัวเลือก (เช่น เลือกหน่วยแม่ห้ามเลือกตัวเอง) */
  excludeId?: string
  disabled?: boolean
  error?: string
  hideLabel?: boolean
  required?: boolean
  className?: string
  /** ข้อความจาก config */
  texts: { placeholder: string; searching: string; noResults: string; clear: string }
}

const PAGE_SIZE = 20
const DEBOUNCE_MS = 250

export function refLabel(r: RefOption): string {
  return `${r.code} · ${r.name}`
}

/**
 * ช่องเลือกข้อมูลอ้างอิงแบบพิมพ์ค้นหา (combobox ตาม WAI-ARIA)
 * ค้นหาจาก API ทีละ PAGE_SIZE รายการ (กรองใน DB ด้วย ?q=) จึงใช้กับข้อมูลจำนวนมากได้
 * คีย์บอร์ด: ↑ ↓ เลือก, Enter ยืนยัน, Esc ปิด
 */
export function ReferenceSelect({
  label,
  source,
  params,
  defaultValue = null,
  value,
  onChange,
  name,
  excludeId,
  disabled,
  error,
  hideLabel,
  required,
  className,
  texts,
}: ReferenceSelectProps) {
  const id = useId()
  const listId = `${id}-list`
  const [internal, setInternal] = useState<RefOption | null>(defaultValue)
  const selected = value !== undefined ? value : internal
  const [text, setText] = useState(selected ? refLabel(selected) : '')
  const [open, setOpen] = useState(false)
  const [options, setOptions] = useState<RefOption[]>([])
  const [loading, setLoading] = useState(false)
  const [active, setActive] = useState(-1)
  const abort = useRef<AbortController | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const paramsKey = JSON.stringify(params ?? {})

  // ค่าจากภายนอกเปลี่ยน (โหมดควบคุม) → แสดงข้อความตามค่าใหม่
  useEffect(() => {
    if (!open) setText(selected ? refLabel(selected) : '')
  }, [selected, open])

  useEffect(
    () => () => {
      abort.current?.abort()
      if (timer.current) clearTimeout(timer.current)
    },
    [],
  )

  function search(q: string) {
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(async () => {
      abort.current?.abort()
      const ctrl = new AbortController()
      abort.current = ctrl
      setLoading(true)
      const query = new URLSearchParams({ ...(JSON.parse(paramsKey) as Record<string, string>), page_size: String(PAGE_SIZE) })
      if (q.trim()) query.set('q', q.trim())
      try {
        const rows = await apiRequest<RefOption[]>(`${source}?${query}`, { signal: ctrl.signal })
        setOptions(rows.filter((r) => r.id !== excludeId).map((r) => ({ id: r.id, code: r.code, name: r.name })))
        setActive(-1)
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') return
        setOptions([])
      } finally {
        if (abort.current === ctrl) setLoading(false)
      }
    }, DEBOUNCE_MS)
  }

  function choose(r: RefOption | null) {
    if (value === undefined) setInternal(r)
    onChange?.(r)
    setText(r ? refLabel(r) : '')
    setOpen(false)
  }

  function openList() {
    if (disabled) return
    setOpen(true)
    search(selected && text === refLabel(selected) ? '' : text)
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (!open) return openList()
      const n = options.length
      if (n === 0) return
      setActive((i) => (e.key === 'ArrowDown' ? (i + 1) % n : (i - 1 + n) % n))
    } else if (e.key === 'Enter' && open) {
      e.preventDefault()
      const r = options[active]
      if (r) choose(r)
    } else if (e.key === 'Escape' && open) {
      e.preventDefault()
      setOpen(false)
      setText(selected ? refLabel(selected) : '')
    }
  }

  const describedBy = error ? `${id}-error` : undefined
  const activeId = open && active >= 0 ? `${id}-opt-${active}` : undefined

  return (
    <div className={cx('relative flex flex-col gap-1.5', className)}>
      <FieldLabel htmlFor={id} label={label} required={required} hidden={hideLabel} />
      {name && <input type="hidden" name={name} value={selected?.id ?? ''} />}
      <div className={fieldFrameClass(error, disabled)}>
        <input
          id={id}
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={activeId}
          aria-invalid={error ? true : undefined}
          aria-required={required || undefined}
          aria-describedby={describedBy}
          autoComplete="off"
          disabled={disabled}
          placeholder={texts.placeholder}
          value={text}
          onChange={(e) => {
            setText(e.target.value)
            setOpen(true)
            search(e.target.value)
          }}
          onFocus={openList}
          onBlur={() => {
            setOpen(false)
            setText(selected ? refLabel(selected) : '')
          }}
          onKeyDown={handleKeyDown}
          className={cx(inputTextClass, 'h-full min-w-0 flex-1 bg-transparent px-3 outline-none placeholder:text-ink-subtle disabled:cursor-not-allowed disabled:text-ink-muted')}
        />
        {selected && !disabled && (
          <button
            type="button"
            aria-label={texts.clear.replace('{label}', label)}
            title={texts.clear.replace('{label}', label)}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => choose(null)}
            className="mr-1 inline-flex size-7 cursor-pointer items-center justify-center rounded-md text-ink-subtle hover:bg-neutral-soft hover:text-ink"
          >
            <X aria-hidden className="size-4" />
          </button>
        )}
      </div>
      {open && (
        <ul
          id={listId}
          role="listbox"
          aria-label={label}
          className="absolute top-full right-0 left-0 z-20 mt-1 max-h-72 overflow-y-auto rounded-(--radius-control) border border-line bg-surface p-1 shadow-pop"
        >
          {loading && options.length === 0 ? (
            <li className="px-2.5 py-1.5 text-sm text-ink-muted">{texts.searching}</li>
          ) : options.length === 0 ? (
            <li className="px-2.5 py-1.5 text-sm text-ink-muted">{texts.noResults}</li>
          ) : (
            options.map((r, i) => (
              <li
                key={r.id}
                id={`${id}-opt-${i}`}
                role="option"
                aria-selected={selected?.id === r.id}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => choose(r)}
                onMouseEnter={() => setActive(i)}
                className={cx('flex cursor-pointer items-baseline gap-2 rounded px-2.5 py-1.5 text-sm', i === active && 'bg-primary-soft', selected?.id === r.id && 'font-medium')}
              >
                <span className="shrink-0 text-[13px] text-ink-subtle">{r.code}</span>
                <span className="truncate">{r.name}</span>
              </li>
            ))
          )}
        </ul>
      )}
      {error && (
        <p id={`${id}-error`} className={errorClass}>
          {error}
        </p>
      )}
    </div>
  )
}
