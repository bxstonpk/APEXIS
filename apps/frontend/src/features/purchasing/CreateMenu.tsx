'use client'

import { ChevronDown, Plus } from 'lucide-react'
import Link from 'next/link'
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { buttonClass } from '@/components/Button/Button'
import { cx } from '@/lib/cx'

export type CreateMenuGroup = { label: string; items: { label: string; href: string }[] }

/**
 * ปุ่มเมนู "สร้าง" (menu button ตาม WAI-ARIA): Enter / Space / ↓ เปิดแล้วโฟกัสรายการแรก, ↑ รายการสุดท้าย,
 * ↑ ↓ Home End เลื่อน, Esc / Tab ปิด (Esc คืนโฟกัสให้ปุ่ม), คลิกนอกเมนูปิด — กลุ่มแยกด้วยหัวกลุ่ม (role=group)
 */
export function CreateMenu({ label, groups }: { label: string; groups: readonly CreateMenuGroup[] }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const button = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const items = () => [...(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]

  const show = (focus: 'first' | 'last') => {
    setOpen(true)
    requestAnimationFrame(() => {
      const all = items()
      all[focus === 'first' ? 0 : all.length - 1]?.focus()
    })
  }
  const close = (refocus: boolean) => {
    setOpen(false)
    if (refocus) button.current?.focus()
  }

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (!menu.current?.contains(t) && !button.current?.contains(t)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const onButtonKey = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      show('first')
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      show('last')
    }
  }
  const onMenuKey = (e: KeyboardEvent) => {
    const all = items()
    const i = all.indexOf(document.activeElement as HTMLElement)
    const go = (n: number) => {
      e.preventDefault()
      all[(n + all.length) % all.length]?.focus()
    }
    if (e.key === 'ArrowDown') go(i + 1)
    else if (e.key === 'ArrowUp') go(i - 1)
    else if (e.key === 'Home') go(0)
    else if (e.key === 'End') go(all.length - 1)
    else if (e.key === 'Escape') {
      e.preventDefault()
      close(true)
    } else if (e.key === 'Tab') setOpen(false)
  }

  return (
    <div className="relative">
      <button
        ref={button}
        type="button"
        id={`${id}-button`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={`${id}-menu`}
        className={buttonClass('primary')}
        onClick={() => (open ? close(false) : show('first'))}
        onKeyDown={onButtonKey}
      >
        <Plus aria-hidden />
        {label}
        <ChevronDown aria-hidden className={cx('transition-transform motion-reduce:transition-none', open && 'rotate-180')} />
      </button>
      <div
        ref={menu}
        id={`${id}-menu`}
        role="menu"
        aria-labelledby={`${id}-button`}
        hidden={!open}
        onKeyDown={onMenuKey}
        className="absolute right-0 z-30 mt-1.5 w-56 rounded-(--radius-panel) border border-line bg-surface py-1.5 shadow-card"
      >
        {groups.map((g, gi) => (
          <div key={g.label} role="group" aria-labelledby={`${id}-g${gi}`} className={cx(gi > 0 && 'mt-1 border-t border-line pt-1')}>
            <div id={`${id}-g${gi}`} role="presentation" className="px-3 pt-1.5 pb-1 text-[12px] font-medium text-ink-muted">
              {g.label}
            </div>
            {g.items.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                role="menuitem"
                tabIndex={-1}
                onClick={() => setOpen(false)}
                className="flex min-h-10 items-center px-3 text-sm text-ink outline-none hover:bg-subtle focus-visible:bg-primary-soft focus-visible:text-primary"
              >
                {item.label}
              </Link>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
