'use client'

import { useEffect, useState } from 'react'
import { TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import { apiRequest } from '@/lib/http'
import { roundUnits } from './calc'
import type { OpenDeposit } from './types'

/**
 * หักเงินมัดจำในเอกสารขาย — แสดงมัดจำที่รับเงินครบแล้วและยังหักได้ของลูกค้า (GET /sales-deposits/open?customer_id=)
 * ยอดที่หัก = ก่อน VAT, ไม่เกินคงเหลือ; ภาษีของมัดจำ (ตามอัตราของใบมัดจำ) หักจากภาษีของเอกสารนี้
 */
export type DepositInput = Record<string, string>

const SCALE = 1_000_000n

/** ภาษีของยอดมัดจำที่หัก (ตัวอย่าง — backend คำนวณจริง) */
export function depositVat(amount: bigint, rate: string): bigint {
  return roundUnits((amount * toUnits(rate)) / (100n * SCALE))
}

/** ตรวจยอดหักมัดจำ: ตัวเลข ≥ 0 และไม่เกินคงเหลือ — คืน error ตาม key `dep-{id}` */
export function validateDeposits(deposits: OpenDeposit[], input: DepositInput): Record<string, string> {
  const d = texts.sales.depositsBlock
  const e: Record<string, string> = {}
  for (const dep of deposits) {
    const raw = input[dep.id] ?? ''
    if (raw.trim() === '') continue
    const n = normalizeAmount(raw)
    if (n === null || toUnits(n) < 0n) e[`dep-${dep.id}`] = d.invalid
    else if (toUnits(n) > toUnits(dep.remaining_amount)) e[`dep-${dep.id}`] = format(d.over, { max: formatAmount(dep.remaining_amount) })
  }
  return e
}

/** รวมยอดหัก (ก่อน VAT) + ภาษีของมัดจำ */
export function depositTotals(deposits: OpenDeposit[], input: DepositInput): { base: bigint; vat: bigint } {
  let base = 0n
  let vat = 0n
  for (const dep of deposits) {
    const n = normalizeAmount(input[dep.id] ?? '')
    if (!n || toUnits(n) <= 0n) continue
    base += toUnits(n)
    vat += depositVat(toUnits(n), dep.vat_rate)
  }
  return { base, vat }
}

export const depositsBody = (deposits: OpenDeposit[], input: DepositInput) =>
  deposits
    .map((dep) => ({ deposit_id: dep.id, amount: normalizeAmount(input[dep.id] ?? '') }))
    .filter((x): x is { deposit_id: string; amount: string } => !!x.amount && toUnits(x.amount) > 0n)

/** โหลดมัดจำคงเหลือของลูกค้า (เปลี่ยนลูกค้า = โหลดใหม่) */
export function useOpenDeposits(api: string, customerId: string | null): { deposits: OpenDeposit[]; loading: boolean } {
  const [deposits, setDeposits] = useState<OpenDeposit[]>([])
  const [loading, setLoading] = useState(false)
  useEffect(() => {
    if (!customerId) {
      setDeposits([])
      return
    }
    let live = true
    setLoading(true)
    apiRequest<OpenDeposit[]>(`${api}?customer_id=${encodeURIComponent(customerId)}`)
      .then((rows) => live && setDeposits(rows))
      .catch(() => live && setDeposits([]))
      .finally(() => live && setLoading(false))
    return () => {
      live = false
    }
  }, [api, customerId])
  return { deposits, loading }
}

export function DepositsApply({
  deposits,
  loading,
  hasCustomer,
  input,
  onChange,
  errors,
}: {
  deposits: OpenDeposit[]
  loading: boolean
  hasCustomer: boolean
  input: DepositInput
  onChange: (next: DepositInput) => void
  errors: Record<string, string>
}) {
  const t = texts.sales
  const d = t.depositsBlock
  return (
    <section aria-labelledby="sale-deposits" className="flex min-w-0 flex-col gap-3">
      <h2 id="sale-deposits" className="text-[15px] font-semibold">
        {t.sections.deposits}
      </h2>
      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        {!hasCustomer || loading || deposits.length === 0 ? (
          <p className="px-4 py-6 text-center text-sm text-ink-muted" aria-live="polite">
            {!hasCustomer ? d.pickCustomer : loading ? d.loading : d.none}
          </p>
        ) : (
          <table className="w-full min-w-[36rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {d.no}
                </th>
                <th scope="col" className="px-3 py-2.5 text-right font-medium">
                  {d.remaining}
                </th>
                <th scope="col" className="w-40 px-3 py-2.5 text-right font-medium">
                  {d.apply}
                </th>
              </tr>
            </thead>
            <tbody>
              {deposits.map((dep) => (
                <tr key={dep.id} className="border-b border-line align-top last:border-b-0">
                  <td className="px-3 py-2.5">
                    <span className="font-mono text-[13px]">{dep.document_no}</span>
                    <span className="block text-[12px] text-ink-muted">{formatDate(dep.date)}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(dep.remaining_amount)}</td>
                  <td className="px-3 py-2">
                    <TextField
                      label={`${d.apply} ${dep.document_no}`}
                      hideLabel
                      inputMode="decimal"
                      value={input[dep.id] ?? ''}
                      error={errors[`dep-${dep.id}`]}
                      className="[&_input]:text-right [&_input]:tabular-nums"
                      onChange={(e) => onChange({ ...input, [dep.id]: e.target.value })}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}
