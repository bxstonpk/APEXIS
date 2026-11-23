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
import type { PurchaseOrder, Ref } from './types'

/**
 * รับสินค้า / บันทึกใบแจ้งหนี้จากใบสั่งซื้อ — ตั้งต้นด้วยจำนวนคงเหลือของแต่ละบรรทัด (แก้ลดได้, 0 = ไม่รวมบรรทัดนั้น)
 *
 *   รับสินค้า = สั่ง − รับแล้ว (ระบุคลังเมื่อมีสินค้านับสต็อก)
 *   ใบแจ้งหนี้ = รับแล้ว − แจ้งหนี้แล้ว; ราคาตั้งต้นตามใบสั่งซื้อ แก้ได้ (ผลต่างลงบัญชีผลต่างราคา)
 */
export function remainingQty(order: PurchaseOrder, mode: 'receipt' | 'invoice'): Record<number, bigint> {
  const out: Record<number, bigint> = {}
  for (const l of order.lines) {
    const left = mode === 'receipt' ? toUnits(l.quantity) - toUnits(l.received_quantity) : toUnits(l.received_quantity) - toUnits(l.invoiced_quantity)
    out[l.line_no] = left > 0n ? left : 0n
  }
  return out
}

export function PurchaseFulfilForm({
  mode,
  order,
  apiPath,
  warehousesApi,
  docsPath,
  orderHref,
}: {
  mode: 'receipt' | 'invoice'
  order: PurchaseOrder
  apiPath: string
  warehousesApi: string
  docsPath: string
  orderHref: string
}) {
  const t = texts.purchasing
  const f = t.fulfil
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const left = remainingQty(order, mode)
  const [head, setHead] = useState({ date: today(), invoiceDate: today(), supplierInvoiceNo: '', reference: '', description: '' })
  const [warehouse, setWarehouse] = useState<Ref | null>(null)
  const [qty, setQty] = useState<Record<number, string>>(() => Object.fromEntries(order.lines.map((l) => [l.line_no, toString(left[l.line_no]!)])))
  const [price, setPrice] = useState<Record<number, string>>(() => Object.fromEntries(order.lines.map((l) => [l.line_no, l.unit_price])))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const invoice = mode === 'invoice'

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: Record<string, string> = {}
    if (!head.date) errs.date = requiredMessage(invoice ? f.postingDate : texts.sales.fields.date)
    if (invoice && !head.supplierInvoiceNo.trim()) errs.supplierInvoiceNo = requiredMessage(f.supplierInvoiceNo)
    if (invoice && !head.invoiceDate) errs.invoiceDate = requiredMessage(f.invoiceDate)
    const lines: { order_line_no: number; quantity: string; unit_price?: string }[] = []
    for (const l of order.lines) {
      const n = normalizeAmount(qty[l.line_no] ?? '')
      if (n === null || toUnits(n) < 0n) errs[`q-${l.line_no}`] = texts.sales.lines.qtyInvalid
      else if (toUnits(n) > left[l.line_no]!) errs[`q-${l.line_no}`] = f.overRemaining
      else if (toUnits(n) > 0n) {
        const line: { order_line_no: number; quantity: string; unit_price?: string } = { order_line_no: l.line_no, quantity: n }
        if (invoice) {
          const p = normalizeAmount(price[l.line_no] ?? '')
          if (p === null || p === '' || toUnits(p) < 0n) errs[`p-${l.line_no}`] = texts.sales.lines.priceInvalid
          else line.unit_price = p
        }
        lines.push(line)
      }
    }
    if (!Object.keys(errs).length && lines.length === 0) errs.lines = f.nothingToDo
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = invoice
      ? { order_id: order.id, supplier_invoice_no: head.supplierInvoiceNo.trim(), invoice_date: head.invoiceDate, date: head.date, description: head.description.trim(), lines }
      : { order_id: order.id, date: head.date, warehouse_id: warehouse?.id ?? null, reference: head.reference.trim(), description: head.description.trim(), lines }
    setBusy(true)
    setError(null)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, { method: 'POST', body })
      showToast(format(texts.sales.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  const num = 'px-3 py-2.5 text-right tabular-nums'
  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={texts.sales.sections.header} description={`${order.order_no ?? ''} · ${order.supplier.code} ${order.supplier.name}`}>
        {invoice ? (
          <>
            <TextField
              label={f.supplierInvoiceNo}
              required
              value={head.supplierInvoiceNo}
              maxLength={50}
              error={errors.supplierInvoiceNo}
              onChange={(e) => setHead((h) => ({ ...h, supplierInvoiceNo: e.target.value }))}
            />
            <TextField
              label={f.invoiceDate}
              required
              type="date"
              value={head.invoiceDate}
              error={errors.invoiceDate}
              onChange={(e) => setHead((h) => ({ ...h, invoiceDate: e.target.value }))}
            />
            <TextField
              label={f.postingDate}
              required
              hint={f.postingDateHint}
              type="date"
              value={head.date}
              error={errors.date}
              onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))}
            />
          </>
        ) : (
          <>
            <TextField label={texts.sales.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
            <ReferenceSelect
              label={f.warehouse}
              hint={f.warehouseHint}
              source={warehousesApi}
              params={{ status: 'active' }}
              value={warehouse}
              onChange={setWarehouse}
              texts={texts.reference}
            />
            <TextField label={texts.sales.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
          </>
        )}
        <TextField label={texts.sales.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <section aria-labelledby="pur-fulfil-lines" className="flex flex-col gap-3">
        <h2 id="pur-fulfil-lines" className="text-[15px] font-semibold">
          {f.lines}
        </h2>
        <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[48rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {texts.sales.lines.item}
                </th>
                {[f.ordered, f.received, f.invoiced, f.remaining].map((h) => (
                  <th key={h} scope="col" className="px-3 py-2.5 text-right font-medium">
                    {h}
                  </th>
                ))}
                <th scope="col" className="w-32 px-3 py-2.5 text-right font-medium">
                  {f.thisTime}
                </th>
                {invoice && (
                  <th scope="col" className="w-36 px-3 py-2.5 text-right font-medium">
                    {f.price}
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {order.lines.map((l) => (
                <tr key={l.line_no} className="border-b border-line align-top last:border-b-0">
                  <td className="px-3 py-2.5">
                    <span className="text-[13px] text-ink-muted">{l.item.code}</span> {l.item.name}
                    {l.description && <span className="block text-[12px] text-ink-muted">{l.description}</span>}
                  </td>
                  <td className={num}>{formatAmount(l.quantity)}</td>
                  <td className={num}>{formatAmount(l.received_quantity, { dashZero: true })}</td>
                  <td className={num}>{formatAmount(l.invoiced_quantity, { dashZero: true })}</td>
                  <td className={`${num} font-medium`}>{formatAmount(left[l.line_no]!)}</td>
                  <td className="px-3 py-2">
                    <TextField
                      label={`${f.thisTime} ${l.item.code}`}
                      hideLabel
                      inputMode="decimal"
                      value={qty[l.line_no] ?? ''}
                      disabled={left[l.line_no] === 0n}
                      error={errors[`q-${l.line_no}`]}
                      className="[&_input]:text-right [&_input]:tabular-nums"
                      onChange={(e) => setQty((q) => ({ ...q, [l.line_no]: e.target.value }))}
                    />
                  </td>
                  {invoice && (
                    <td className="px-3 py-2">
                      <TextField
                        label={`${f.price} ${l.item.code}`}
                        hideLabel
                        inputMode="decimal"
                        value={price[l.line_no] ?? ''}
                        disabled={left[l.line_no] === 0n}
                        error={errors[`p-${l.line_no}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => setPrice((p) => ({ ...p, [l.line_no]: e.target.value }))}
                      />
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {invoice && <p className="text-[13px] text-ink-muted">{f.priceHint}</p>}
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
            {invoice ? t.actions.invoice : t.actions.receive}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
