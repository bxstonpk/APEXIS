'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { PurchaseKind, Ref } from './types'

type Line = { key: number; item: Ref | null; description: string; quantity: string; unitPrice: string }

let seq = 0
const newLine = (): Line => ({ key: ++seq, item: null, description: '', quantity: '1', unitPrice: '' })

/**
 * ซื้อตรงไม่ผ่านใบสั่ง (POST /supplier-invoices | /supplier-service-invoices ไม่ส่ง order_id) — backend ออกใบสั่ง (อนุมัติ)
 * + ใบรับสินค้า / ใบตรวจรับงาน + ใบแจ้งหนี้ใน transaction เดียว
 *
 *   ซื้อสินค้า: เลือกได้เฉพาะสินค้า (stock / ไม่นับสต็อก), คลังที่รับเข้า
 *   ซื้อบริการ: เลือกได้เฉพาะบริการ, เลขที่ใบกำกับที่มาพร้อมใบแจ้งหนี้ (ไม่บังคับ)
 */
export function DirectPurchaseForm({
  flow,
  apiPath,
  suppliersApi,
  itemsApi,
  itemTypes,
  warehousesApi,
  docsPath,
  vat,
}: {
  flow: PurchaseKind
  apiPath: string
  suppliersApi: string
  itemsApi: string
  itemTypes: string
  warehousesApi: string
  docsPath: string
  vat: { registered: boolean; rate: string } | null
}) {
  const t = texts.purchasing
  const f = t.fulfil
  const tl = texts.sales.lines
  const service = flow === 'service'
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [supplier, setSupplier] = useState<Ref | null>(null)
  const [warehouse, setWarehouse] = useState<Ref | null>(null)
  const [head, setHead] = useState({
    supplierInvoiceNo: '',
    invoiceDate: today(),
    date: today(),
    taxInvoiceNo: '',
    taxInvoiceDate: '',
    reference: '',
    description: '',
    inclusive: false,
  })
  const [lines, setLines] = useState<Line[]>(() => [newLine()])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const up = (key: number, patch: Partial<Line>) => setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: Record<string, string> = {}
    if (!supplier) errs.supplier = requiredMessage(t.orders.fields.supplier)
    if (!head.supplierInvoiceNo.trim()) errs.supplierInvoiceNo = requiredMessage(service ? f.serviceInvoiceNo : f.supplierInvoiceNo)
    if (!head.date) errs.date = requiredMessage(f.postingDate)
    const body = lines.map((l) => {
      const q = normalizeAmount(l.quantity)
      const p = normalizeAmount(l.unitPrice)
      if (!l.item) errs[`item-${l.key}`] = tl.itemRequired
      if (!q || toUnits(q) <= 0n) errs[`qty-${l.key}`] = tl.qtyInvalid
      if (p === null || p === '' || toUnits(p) < 0n) errs[`price-${l.key}`] = tl.priceInvalid
      return { item_id: l.item?.id, description: l.description.trim(), quantity: q, unit_price: p }
    })
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const payload: Record<string, unknown> = {
      supplier_id: supplier!.id,
      supplier_invoice_no: head.supplierInvoiceNo.trim(),
      invoice_date: head.invoiceDate || head.date,
      date: head.date,
      price_includes_vat: head.inclusive,
      reference: head.reference.trim(),
      description: head.description.trim(),
      lines: body,
    }
    if (!service) payload.warehouse_id = warehouse?.id ?? null
    if (service && head.taxInvoiceNo.trim()) {
      payload.tax_invoice_no = head.taxInvoiceNo.trim()
      payload.tax_invoice_date = head.taxInvoiceDate || head.invoiceDate || head.date
    }
    setBusy(true)
    setError(null)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, { method: 'POST', body: payload })
      showToast(format(texts.sales.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  const cell = 'px-2 py-2'
  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={texts.sales.sections.header}>
        <ReferenceSelect
          label={t.orders.fields.supplier}
          required
          source={suppliersApi}
          params={{ status: 'active' }}
          value={supplier}
          onChange={setSupplier}
          error={errors.supplier}
          className="sm:col-span-2"
          texts={texts.reference}
        />
        <TextField
          label={service ? f.serviceInvoiceNo : f.supplierInvoiceNo}
          required
          value={head.supplierInvoiceNo}
          maxLength={50}
          error={errors.supplierInvoiceNo}
          onChange={(e) => setHead((h) => ({ ...h, supplierInvoiceNo: e.target.value }))}
        />
        <TextField
          label={service ? f.serviceInvoiceDate : f.invoiceDate}
          type="date"
          value={head.invoiceDate}
          onChange={(e) => setHead((h) => ({ ...h, invoiceDate: e.target.value }))}
        />
        <TextField label={f.postingDate} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        {service ? (
          <>
            <TextField label={f.taxInvoiceNo} value={head.taxInvoiceNo} maxLength={50} onChange={(e) => setHead((h) => ({ ...h, taxInvoiceNo: e.target.value }))} />
            <TextField
              label={f.taxInvoiceDate}
              type="date"
              value={head.taxInvoiceDate}
              disabled={!head.taxInvoiceNo.trim()}
              onChange={(e) => setHead((h) => ({ ...h, taxInvoiceDate: e.target.value }))}
            />
          </>
        ) : (
          <ReferenceSelect label={f.warehouse} source={warehousesApi} params={{ status: 'active' }} value={warehouse} onChange={setWarehouse} texts={texts.reference} />
        )}
        <TextField label={texts.sales.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
        <TextField label={texts.sales.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
        {vat?.registered !== false && (
          <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
            <input
              type="checkbox"
              checked={head.inclusive}
              onChange={(e) => setHead((h) => ({ ...h, inclusive: e.target.checked }))}
              className="mt-0.5 size-4 accent-primary"
            />
            <span className="block text-sm font-medium">{texts.sales.fields.priceIncludesVat}</span>
          </label>
        )}
      </FormSection>

      <section aria-labelledby="pur-direct-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="pur-direct-lines" className="text-[15px] font-semibold">
          {f.lines}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[44rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                  #
                </th>
                <th scope="col" className="px-2 py-2.5 font-medium">
                  {tl.item}
                </th>
                <th scope="col" className="w-28 px-2 py-2.5 text-right font-medium">
                  {tl.quantity}
                </th>
                <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                  {tl.unitPrice}
                </th>
                <th scope="col" className="w-12 px-2 py-2.5">
                  <span className="sr-only">{texts.common.delete}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {lines.map((l, i) => {
                const n = i + 1
                const label = (field: string) => `${field} ${format(tl.line, { n })}`
                return (
                  <tr key={l.key} className="border-b border-line align-top last:border-b-0">
                    <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{n}</td>
                    <td className={cell}>
                      <ReferenceSelect
                        label={label(tl.item)}
                        hideLabel
                        source={itemsApi}
                        params={{ status: 'active', type: itemTypes }}
                        value={l.item}
                        onChange={(item) => up(l.key, { item })}
                        error={errors[`item-${l.key}`]}
                        texts={texts.reference}
                      />
                    </td>
                    <td className={cell}>
                      <TextField
                        label={label(tl.quantity)}
                        hideLabel
                        inputMode="decimal"
                        value={l.quantity}
                        error={errors[`qty-${l.key}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => up(l.key, { quantity: e.target.value })}
                      />
                    </td>
                    <td className={cell}>
                      <TextField
                        label={label(tl.unitPrice)}
                        hideLabel
                        inputMode="decimal"
                        value={l.unitPrice}
                        error={errors[`price-${l.key}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => up(l.key, { unitPrice: e.target.value })}
                      />
                    </td>
                    <td className="px-2 pt-2.5">
                      <button
                        type="button"
                        aria-label={format(tl.remove, { n })}
                        disabled={lines.length <= 1}
                        onClick={() => setLines((ls) => ls.filter((x) => x.key !== l.key))}
                        className="inline-flex size-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-muted hover:bg-subtle hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <Trash2 aria-hidden className="size-4" />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
        <div>
          <Button type="button" variant="secondary" onClick={() => setLines((ls) => [...ls, newLine()])}>
            <Plus aria-hidden />
            {tl.add}
          </Button>
        </div>
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {service ? t.actions.serviceInvoice : t.actions.invoice}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
