'use client'

import { CircleAlert, CircleCheck, Plus, Trash2 } from 'lucide-react'
import { Button, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toString, toUnits } from '@/features/accounting/money'
import { cx } from '@/lib/cx'
import { requiredMessage } from '@/lib/validation'
import type { Ref } from './types'

/**
 * ช่องทางรับ / จ่ายเงิน (หลายบัญชีเงินสด / ธนาคาร) — ใช้ร่วมระหว่างรับเงิน / จ่ายเงิน (การเงิน)
 * ยอดรวมของช่องทางต้องเท่ากับยอดเป้าหมาย (target) — ปุ่ม "ใส่ยอดที่เหลือ" เติมผลต่างในแถวสุดท้าย
 */
export type MethodLine = { key: number; cash: Ref | null; amount: string; reference: string }

let nextKey = 1
export const methodLine = (p: Partial<MethodLine> = {}): MethodLine => ({ key: nextKey++, cash: null, amount: '', reference: '', ...p })

export const methodsTotal = (methods: MethodLine[]) => methods.reduce((a, m) => a + toUnits(m.amount), 0n)

/** ตรวจแต่ละแถว — คืน error ตาม key `cash-{key}` / `mamt-{key}` */
export function validateMethods(methods: MethodLine[]): Record<string, string> {
  const t = texts.finance
  const e: Record<string, string> = {}
  for (const m of methods) {
    if (!m.cash) e[`cash-${m.key}`] = requiredMessage(t.fields.cashAccount)
    const n = normalizeAmount(m.amount)
    if (!n || toUnits(n) <= 0n) e[`mamt-${m.key}`] = t.errors.amountRequired
  }
  return e
}

export const methodsBody = (methods: MethodLine[]) =>
  methods.map((m) => ({ cash_account_id: m.cash!.id, amount: normalizeAmount(m.amount), reference: m.reference.trim() }))

export function MethodsEditor({
  methods,
  onChange,
  cashAccountsApi,
  target,
  errors,
}: {
  methods: MethodLine[]
  onChange: (next: MethodLine[]) => void
  cashAccountsApi: string
  /** ยอดที่ต้องรับ / จ่าย (หน่วยย่อย) — ใช้กับปุ่ม "ใส่ยอดที่เหลือ" */
  target: bigint
  errors: Record<string, string>
}) {
  const t = texts.finance
  const diff = target - methodsTotal(methods)
  /** ป้ายของช่องในแถว: จอกว้างซ่อน (มีหัวคอลัมน์แล้ว) แต่ screen reader ยังอ่านชื่อเต็ม */
  const methodLabel = 'sm:[&>span:first-child]:sr-only'

  function fillRemaining() {
    if (diff === 0n || methods.length === 0) return
    onChange(methods.map((m, i) => (i === methods.length - 1 ? { ...m, amount: toString(toUnits(m.amount) + diff) } : m)))
  }

  return (
    <div className="flex flex-col gap-3 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card">
      <div aria-hidden className="hidden gap-3 text-[13px] font-medium text-ink-muted sm:grid sm:grid-cols-[minmax(0,1fr)_9rem_minmax(0,10rem)_2.5rem]">
        <span>{t.fields.cashAccount}</span>
        <span className="text-right">{t.fields.amount}</span>
        <span>{t.fields.reference}</span>
      </div>
      {methods.map((m, i) => {
        const n = i + 1
        const up = (patch: Partial<MethodLine>) => onChange(methods.map((x) => (x.key === m.key ? { ...x, ...patch } : x)))
        return (
          <div key={m.key} className="grid items-start gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_minmax(0,10rem)_2.5rem]">
            <ReferenceSelect
              label={`${t.fields.cashAccount} ${format(t.lines.method, { n })}`}
              className={methodLabel}
              source={cashAccountsApi}
              params={{ status: 'active' }}
              value={m.cash}
              onChange={(c) => up({ cash: c })}
              error={errors[`cash-${m.key}`]}
              texts={texts.reference}
            />
            <TextField
              label={`${t.fields.amount} ${format(t.lines.method, { n })}`}
              inputMode="decimal"
              value={m.amount}
              error={errors[`mamt-${m.key}`]}
              className={`${methodLabel} [&_input]:text-right [&_input]:tabular-nums`}
              onChange={(e) => up({ amount: e.target.value })}
            />
            <TextField
              label={`${t.fields.reference} ${format(t.lines.method, { n })}`}
              className={methodLabel}
              value={m.reference}
              maxLength={50}
              onChange={(e) => up({ reference: e.target.value })}
            />
            <button
              type="button"
              aria-label={format(t.lines.removeMethod, { n })}
              disabled={methods.length <= 1}
              onClick={() => onChange(methods.filter((x) => x.key !== m.key))}
              className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
            >
              <Trash2 aria-hidden className="size-4" />
            </button>
          </div>
        )
      })}
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="secondary" onClick={() => onChange([...methods, methodLine()])}>
          <Plus aria-hidden />
          {t.lines.addMethod}
        </Button>
        {diff !== 0n && target > 0n && (
          <Button size="sm" variant="ghost" onClick={fillRemaining}>
            {t.totals.fill}
          </Button>
        )}
      </div>
    </div>
  )
}

/** สถานะยอดช่องทาง: ตรง / ต่าง (ไอคอน + ข้อความ ไม่ใช้สีอย่างเดียว) */
export function MethodsBalance({ target, total, error }: { target: bigint; total: bigint; error?: string }) {
  const t = texts.finance
  const diff = target - total
  return (
    <>
      <p aria-live="polite" className="mt-3 text-[13px] font-medium">
        {target > 0n && diff === 0n ? (
          <span className="inline-flex items-center gap-1.5 text-success">
            <CircleCheck aria-hidden className="size-4" />
            {t.totals.balanced}
          </span>
        ) : (
          target > 0n && (
            <span className="inline-flex items-center gap-1.5 text-warning">
              <CircleAlert aria-hidden className="size-4" />
              {format(t.totals.unbalanced, { amount: formatAmount(diff < 0n ? -diff : diff) })}
            </span>
          )
        )}
      </p>
      {error && (
        <p role="alert" className={cx('mt-1 text-[13px] text-danger')}>
          {error}
        </p>
      )}
    </>
  )
}
