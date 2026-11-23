'use client'

import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toString, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { Order, Ref } from './types'

/**
 * ส่งของ / ออกใบกำกับภาษีจากใบสั่งขาย — ตั้งต้นด้วยจำนวนคงเหลือของแต่ละบรรทัด (แก้ลดได้, ใส่ 0 = ไม่ส่งบรรทัดนั้น)
 *
 *   ส่งของ = สั่ง - ส่งแล้ว
 *   ออกบิลจากของที่ส่ง = ส่งแล้ว - ออกบิลแล้ว
 *   ส่งของพร้อมออกบิล = สั่ง - ส่งแล้ว (ส่งและออกบิลในเอกสารเดียว)
 */
export function remaining(order: Order, mode: 'delivery' | 'invoice', withDelivery: boolean): Record<number, bigint> {
  const out: Record<number, bigint> = {}
  for (const l of order.lines) {
    const q = toUnits(l.quantity)
    const d = toUnits(l.delivered_quantity)
    const inv = toUnits(l.invoiced_quantity)
    const left = mode === 'delivery' || withDelivery ? q - d : d - inv
    out[l.line_no] = left > 0n ? left : 0n
  }
  return out
}

export function FulfilForm({
  mode,
  order,
  apiPath,
  warehousesApi,
  docsPath,
  orderHref,
}: {
  mode: 'delivery' | 'invoice'
  order: Order
  apiPath: string
  warehousesApi: string
  docsPath: string
  orderHref: string
}) {
  const t = texts.sales
  const tl = t.lines
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const initialWith = mode === 'invoice' && order.lines.every((l) => toUnits(l.delivered_quantity) === 0n)
  const [withDelivery, setWithDelivery] = useState(initialWith)
  const [head, setHead] = useState({ date: today(), reference: '', description: '' })
  const [warehouse, setWarehouse] = useState<Ref | null>(null)
  const left = remaining(order, mode, withDelivery)
  const [qty, setQty] = useState<Record<number, string>>(() => Object.fromEntries(order.lines.map((l) => [l.line_no, toString(left[l.line_no]!)])))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const delivers = mode === 'delivery' || withDelivery

  function switchWith(v: boolean) {
    setWithDelivery(v)
    const next = remaining(order, mode, v)
    setQty(Object.fromEntries(order.lines.map((l) => [l.line_no, toString(next[l.line_no]!)])))
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: Record<string, string> = {}
    if (!head.date) errs.date = requiredMessage(t.fields.date)
    const lines: { order_line_no: number; quantity: string }[] = []
    for (const l of order.lines) {
      const n = normalizeAmount(qty[l.line_no] ?? '')
      if (n === null || toUnits(n) < 0n) errs[`q-${l.line_no}`] = tl.qtyInvalid
      else if (toUnits(n) > left[l.line_no]!) errs[`q-${l.line_no}`] = tl.overRemaining
      else if (toUnits(n) > 0n) lines.push({ order_line_no: l.line_no, quantity: n })
    }
    if (!Object.keys(errs).length && lines.length === 0) errs.lines = tl.nothingToDo
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      order_id: order.id,
      date: head.date,
      reference: head.reference.trim(),
      description: head.description.trim(),
      warehouse_id: delivers ? (warehouse?.id ?? null) : null,
      lines,
    }
    if (mode === 'invoice') body.with_delivery = withDelivery
    setBusy(true)
    setError(null)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, { method: 'POST', body })
      showToast(format(t.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={t.sections.header} description={`${order.document_no ?? ''} · ${order.customer.code} ${order.customer.name}`}>
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField label={t.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
        {mode === 'invoice' && (
          <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
            <input type="checkbox" checked={withDelivery} onChange={(e) => switchWith(e.target.checked)} className="mt-0.5 size-4 accent-primary" />
            <span>
              <span className="block text-sm font-medium">{t.fields.withDelivery}</span>
              <span className="block text-[13px] text-ink-muted">{t.fields.withDeliveryHint}</span>
            </span>
          </label>
        )}
        {delivers && (
          <ReferenceSelect
            label={t.fields.warehouse}
            hint={t.fields.warehouseHint}
            source={warehousesApi}
            params={{ status: 'active' }}
            value={warehouse}
            onChange={setWarehouse}
            texts={texts.reference}
          />
        )}
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <section aria-labelledby="fulfil-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="fulfil-lines" className="text-[15px] font-semibold">
          {t.sections.fulfil}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[44rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {tl.item}
                </th>
                {[tl.ordered, tl.delivered, tl.invoiced, tl.remaining].map((h) => (
                  <th key={h} scope="col" className="px-3 py-2.5 text-right font-medium">
                    {h}
                  </th>
                ))}
                <th scope="col" className="w-36 px-3 py-2.5 text-right font-medium">
                  {tl.thisTime}
                </th>
              </tr>
            </thead>
            <tbody>
              {order.lines.map((l) => (
                <tr key={l.line_no} className="border-b border-line align-top last:border-b-0">
                  <td className="px-3 py-2.5">
                    <span className="text-[13px] text-ink-muted">{l.item.code}</span> {l.item.name}
                    {l.description && <span className="block text-[12px] text-ink-muted">{l.description}</span>}
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(l.quantity)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(l.delivered_quantity, { dashZero: true })}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(l.invoiced_quantity, { dashZero: true })}</td>
                  <td className="px-3 py-2.5 text-right font-medium tabular-nums">{formatAmount(left[l.line_no]!)}</td>
                  <td className="px-3 py-2">
                    <TextField
                      label={`${tl.thisTime} ${l.item.code}`}
                      hideLabel
                      inputMode="decimal"
                      value={qty[l.line_no] ?? ''}
                      disabled={left[l.line_no] === 0n}
                      error={errors[`q-${l.line_no}`]}
                      className="[&_input]:text-right [&_input]:tabular-nums"
                      onChange={(e) => setQty((q) => ({ ...q, [l.line_no]: e.target.value }))}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={orderHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {mode === 'delivery' ? t.actions.deliver : t.actions.invoice}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
