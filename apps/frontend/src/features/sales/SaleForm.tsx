'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toString, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { lineAmounts, sumAmounts } from './calc'
import { DepositsApply, depositsBody, depositTotals, useOpenDeposits, validateDeposits, type DepositInput } from './DepositsApply'
import type { Vat } from './SalesDocForm'
import type { ItemType, Order, Ref, SaleKind } from './types'

/**
 * ฟอร์มเอกสารขาย (ลงบัญชีทันทีที่บันทึก; รับเงินยืนยันที่แอปการเงิน) — ใช้ร่วม 2 ชนิด:
 *
 *   ขายสินค้า (invoice): ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี — ภาษีขายเกิดวันที่เอกสาร, ตัดสต็อกจากคลังที่เลือก
 *   ใบแจ้งหนี้ค่าบริการ (service_invoice): เฉพาะสินค้าชนิดบริการ, ภาษีขายยังไม่ถึงกำหนด (เกิดเมื่อรับเงิน / ออกใบกำกับภาษี)
 *
 * ที่มาของบรรทัด: ใบสั่งขาย (?order_id= — ตั้งต้นด้วยคงเหลือ สั่ง − ออกเอกสารแล้ว) หรือกรอกเอง
 * ชนิดสินค้าโหลดตอนเลือก (1 คำขอต่อสินค้า) เพื่อรู้ว่าต้องเลือกคลัง (สินค้านับสต็อก) และกันสินค้าในใบขายบริการ
 * หักเงินมัดจำ: เฉพาะมัดจำที่รับเงินครบแล้ว — ยอดก่อน VAT ไม่เกินคงเหลือ
 */

type Line = { key: number; item: Ref | null; description: string; quantity: string; unitPrice: string }
let nextKey = 1
const newLine = (p: Partial<Line> = {}): Line => ({ key: nextKey++, item: null, description: '', quantity: '1', unitPrice: '', ...p })

export type SaleApis = {
  submit: string
  customers: string
  items: string
  warehouses: string
  /** GET /sales-deposits/open */
  openDeposits: string
}

export function SaleForm({ kind, apis, docsPath, listHref, vat, order }: { kind: SaleKind; apis: SaleApis; docsPath: string; listHref: string; vat: Vat; order?: Order }) {
  const t = texts.sales
  const tl = t.lines
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const service = kind === 'service_invoice'

  // ---------- หัวเอกสาร ----------
  const [customer, setCustomer] = useState<Ref | null>(order?.customer ?? null)
  const [branchNo, setBranchNo] = useState(order?.customer_branch_no ?? '')
  const [branchOptions, setBranchOptions] = useState<{ value: string; label: string }[]>([])
  const [head, setHead] = useState({ date: today(), reference: order?.reference ?? '', description: '', inclusive: order?.price_includes_vat ?? false })
  const [warehouse, setWarehouse] = useState<Ref | null>(null)

  useEffect(() => {
    if (!customer || order) {
      setBranchOptions([])
      return
    }
    let live = true
    apiRequest<{ branch_no: string; branches?: { branch_no: string; name: string }[] }>(`${apis.customers}/${customer.id}`)
      .then((c) => {
        if (!live) return
        const main = c.branch_no === '00000' ? t.print.headOffice : texts.partners.branches.main
        const opts = [{ value: c.branch_no, label: `${c.branch_no} ${main}` }, ...(c.branches ?? []).map((b) => ({ value: b.branch_no, label: `${b.branch_no} ${b.name}`.trim() }))]
        setBranchOptions(opts)
        setBranchNo((cur) => (opts.some((o) => o.value === cur) ? cur : c.branch_no))
      })
      .catch(() => live && setBranchOptions([]))
    return () => {
      live = false
    }
  }, [customer, order, apis.customers, t.print.headOffice])

  // ---------- บรรทัด ----------
  const orderLines = order ? order.lines.filter((l) => !service || !l.item.type || l.item.type === 'service') : []
  const leftOf = (l: Order['lines'][number]) => {
    const left = toUnits(l.quantity) - toUnits(l.invoiced_quantity)
    return left > 0n ? left : 0n
  }
  const [qty, setQty] = useState<Record<number, string>>(() => Object.fromEntries(orderLines.map((l) => [l.line_no, toString(leftOf(l))])))
  const [lines, setLines] = useState<Line[]>(() => [newLine()])
  const up = (key: number, patch: Partial<Line>) => setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))

  // ชนิดสินค้า (stock / non_stock / service) — จากใบสั่งขาย หรือโหลดเมื่อเลือกสินค้า
  const [types, setTypes] = useState<Record<string, ItemType>>(() =>
    Object.fromEntries((order?.lines ?? []).filter((l) => l.item.type).map((l) => [l.item.id, l.item.type!])),
  )
  function pickItem(key: number, item: Ref | null) {
    up(key, { item })
    if (!item || types[item.id]) return
    apiRequest<{ type: ItemType }>(`${apis.items}/${item.id}`).then(
      (it) => setTypes((m) => ({ ...m, [item.id]: it.type })),
      () => undefined,
    )
  }
  const usedItems = order ? orderLines.filter((l) => toUnits(qty[l.line_no] ?? '0') > 0n).map((l) => l.item.id) : lines.flatMap((l) => (l.item ? [l.item.id] : []))
  const hasStock = usedItems.some((id) => types[id] === 'stock')
  const goodsIn = (id: string) => types[id] !== undefined && types[id] !== 'service'

  // ---------- มัดจำ ----------
  const { deposits, loading: loadingDeposits } = useOpenDeposits(apis.openDeposits, customer?.id ?? null)
  const [depositInput, setDepositInput] = useState<DepositInput>({})

  // ---------- ยอดรวม ----------
  const rate = order ? order.vat_rate : vat?.registered ? vat.rate : '0'
  const vatKnown = !!order || vat !== null
  const amounts = order
    ? orderLines.map((l) => lineAmounts(normalizeAmount(qty[l.line_no] ?? '') ?? '0', l.unit_price, rate, head.inclusive))
    : lines.map((l) => lineAmounts(l.quantity, l.unitPrice, vatKnown ? rate : '0', head.inclusive))
  const total = sumAmounts(amounts)
  const dep = depositTotals(deposits, depositInput)
  const payable = total.gross - dep.base - dep.vat

  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  function validate(): Record<string, string> {
    const e: Record<string, string> = {}
    if (!customer) e.customer = requiredMessage(t.fields.customer)
    if (!head.date) e.date = requiredMessage(t.fields.date)
    if (order) {
      let any = false
      for (const l of orderLines) {
        const n = normalizeAmount(qty[l.line_no] ?? '')
        if (n === null || toUnits(n) < 0n) e[`q-${l.line_no}`] = tl.qtyInvalid
        else if (toUnits(n) > leftOf(l)) e[`q-${l.line_no}`] = tl.overRemaining
        else if (toUnits(n) > 0n) any = true
      }
      if (!Object.keys(e).some((k) => k.startsWith('q-')) && !any) e.lines = tl.nothingToDo
    } else {
      if (lines.length === 0) e.lines = tl.noLines
      for (const l of lines) {
        if (!l.item) e[`item-${l.key}`] = tl.itemRequired
        else if (service && goodsIn(l.item.id)) e[`item-${l.key}`] = tl.serviceOnly
        const q = normalizeAmount(l.quantity)
        if (!q || toUnits(q) <= 0n) e[`qty-${l.key}`] = tl.qtyInvalid
        const p = normalizeAmount(l.unitPrice)
        if (p === null || p === '' || toUnits(p) < 0n) e[`price-${l.key}`] = tl.priceInvalid
      }
    }
    if (!service && hasStock && !warehouse) e.warehouse = t.fields.warehouseRequired
    Object.assign(e, validateDeposits(deposits, depositInput))
    return e
  }

  async function submit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    setFormError(null)
    const errs = validate()
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      date: head.date,
      reference: head.reference.trim(),
      description: head.description.trim(),
      deposits: depositsBody(deposits, depositInput),
    }
    if (!service) body.warehouse_id = warehouse?.id ?? null
    if (order) {
      body.order_id = order.id
      body.lines = orderLines
        .map((l) => ({ order_line_no: l.line_no, quantity: normalizeAmount(qty[l.line_no] ?? '') ?? '0' }))
        .filter((l) => toUnits(l.quantity) > 0n)
    } else {
      body.customer_id = customer!.id
      body.customer_branch_no = branchNo
      body.price_includes_vat = head.inclusive
      body.lines = lines.map((l) => ({ item_id: l.item!.id, description: l.description.trim(), quantity: normalizeAmount(l.quantity), unit_price: normalizeAmount(l.unitPrice) }))
    }
    setBusy(true)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apis.submit, { method: 'POST', body })
      showToast(format(t.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  const num = 'px-3 py-2.5 text-right tabular-nums'

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      <FormSection title={t.sections.header}>
        {order ? (
          <TextField label={t.fields.customer} value={`${order.customer.code} ${order.customer.name}`} readOnly className="sm:col-span-2" />
        ) : (
          <ReferenceSelect
            label={t.fields.customer}
            required
            source={apis.customers}
            params={{ status: 'active' }}
            value={customer}
            onChange={(c) => {
              setCustomer(c)
              setDepositInput({})
            }}
            error={errors.customer}
            texts={texts.reference}
            className="sm:col-span-2"
          />
        )}
        {branchOptions.length > 1 && (
          <SelectField label={t.fields.customerBranch} value={branchNo} options={branchOptions} onChange={(e) => setBranchNo(e.target.value)} className="sm:col-span-2" />
        )}
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField label={t.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
        {!service && (
          <ReferenceSelect
            label={t.fields.warehouse}            required={hasStock}
            source={apis.warehouses}
            params={{ status: 'active' }}
            value={warehouse}
            onChange={setWarehouse}
            error={errors.warehouse}
            texts={texts.reference}
          />
        )}
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
        {!order && (
          <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
            <input
              type="checkbox"
              checked={head.inclusive}
              disabled={vat?.registered === false}
              onChange={(e) => setHead((h) => ({ ...h, inclusive: e.target.checked }))}
              className="mt-0.5 size-4 accent-primary"
            />
            <span className="block text-sm font-medium">{t.fields.priceIncludesVat}</span>
          </label>
        )}
      </FormSection>

      <section aria-labelledby="sale-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="sale-lines" className="text-[15px] font-semibold">
          {order ? t.sections.fulfil : t.sections.lines}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          {order ? (
            <table className="w-full min-w-[40rem] border-collapse text-sm">
              <thead>
                <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                  <th scope="col" className="px-3 py-2.5 font-medium">
                    {tl.item}
                  </th>
                  {[tl.ordered, tl.invoiced, tl.remaining].map((h) => (
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
                {orderLines.map((l) => (
                  <tr key={l.line_no} className="border-b border-line align-top last:border-b-0">
                    <td className="px-3 py-2.5">
                      <span className="text-[13px] text-ink-muted">{l.item.code}</span> {l.item.name}
                      {l.description && <span className="block text-[12px] text-ink-muted">{l.description}</span>}
                    </td>
                    <td className={num}>{formatAmount(l.quantity)}</td>
                    <td className={num}>{formatAmount(l.invoiced_quantity, { dashZero: true })}</td>
                    <td className={`${num} font-medium`}>{formatAmount(leftOf(l))}</td>
                    <td className="px-3 py-2">
                      <TextField
                        label={`${tl.thisTime} ${l.item.code}`}
                        hideLabel
                        inputMode="decimal"
                        value={qty[l.line_no] ?? ''}
                        disabled={leftOf(l) === 0n}
                        error={errors[`q-${l.line_no}`]}
                        className="[&_input]:text-right [&_input]:tabular-nums"
                        onChange={(e) => setQty((q) => ({ ...q, [l.line_no]: e.target.value }))}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <table className="w-full min-w-[52rem] border-collapse text-sm">
              <thead>
                <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                  <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                    #
                  </th>
                  <th scope="col" className="w-[30%] px-2 py-2.5 font-medium">
                    {tl.item}
                  </th>
                  <th scope="col" className="px-2 py-2.5 font-medium">
                    {tl.description}
                  </th>
                  <th scope="col" className="w-28 px-2 py-2.5 text-right font-medium">
                    {tl.quantity}
                  </th>
                  <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                    {tl.unitPrice}
                  </th>
                  <th scope="col" className="w-32 px-2 py-2.5 text-right font-medium">
                    {tl.amount}
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
                      <td className="px-2 py-2">
                        <ReferenceSelect
                          label={label(tl.item)}
                          hideLabel
                          source={apis.items}
                          params={service ? { status: 'active', type: 'service' } : { status: 'active' }}
                          value={l.item}
                          onChange={(item) => pickItem(l.key, item)}
                          error={errors[`item-${l.key}`]}
                          texts={texts.reference}
                        />
                      </td>
                      <td className="px-2 py-2">
                        <TextField label={label(tl.description)} hideLabel value={l.description} maxLength={500} onChange={(e) => up(l.key, { description: e.target.value })} />
                      </td>
                      <td className="px-2 py-2">
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
                      <td className="px-2 py-2">
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
                      <td className="px-2 pt-4 text-right tabular-nums">{formatAmount(head.inclusive ? amounts[i]!.gross : amounts[i]!.net)}</td>
                      <td className="px-2 py-2">
                        <button
                          type="button"
                          aria-label={format(tl.remove, { n })}
                          disabled={lines.length <= 1}
                          onClick={() => setLines((ls) => ls.filter((x) => x.key !== l.key))}
                          className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          <Trash2 aria-hidden className="size-4" />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
        {!order && (
          <div>
            <Button size="sm" variant="secondary" onClick={() => setLines((ls) => [...ls, newLine()])}>
              <Plus aria-hidden />
              {tl.add}
            </Button>
          </div>
        )}
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <DepositsApply deposits={deposits} loading={loadingDeposits} hasCustomer={!!customer} input={depositInput} onChange={setDepositInput} errors={errors} />

      <div className="flex justify-end">
        <dl aria-live="polite" className="w-full max-w-sm rounded-(--radius-panel) border border-line bg-surface p-4 text-sm shadow-card">
          <div className="flex justify-between gap-4">
            <dt className="text-ink-muted">{t.totals.net}</dt>
            <dd className="tabular-nums">{formatAmount(total.net)}</dd>
          </div>
          <div className="mt-2 flex justify-between gap-4">
            <dt className="text-ink-muted">{format(t.totals.vat, { rate: vatKnown ? rate : '–' })}</dt>
            <dd className="tabular-nums">{vatKnown ? formatAmount(total.vat) : '–'}</dd>
          </div>
          <div className="mt-2 flex justify-between gap-4 border-t border-line pt-2 font-semibold">
            <dt>{t.totals.gross}</dt>
            <dd className="tabular-nums">{formatAmount(total.gross)}</dd>
          </div>
          {dep.base > 0n && (
            <>
              <div className="mt-2 flex justify-between gap-4">
                <dt className="text-ink-muted">{t.totals.deposit}</dt>
                <dd className="tabular-nums">{formatAmount(-(dep.base + dep.vat))}</dd>
              </div>
              <div className="mt-2 flex justify-between gap-4 border-t border-line pt-2 font-semibold">
                <dt>{t.totals.payable}</dt>
                <dd className="tabular-nums">{formatAmount(payable)}</dd>
              </div>
            </>
          )}
          {service && vatKnown && total.vat > 0n && <p className="mt-2 text-[12px] text-ink-muted">{t.totals.undue}</p>}
          {!vatKnown && <p className="mt-2 text-[12px] text-ink-muted">{t.totals.vatLater}</p>}
        </dl>
      </div>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.postSale}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
