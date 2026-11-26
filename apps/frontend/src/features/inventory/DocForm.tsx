'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { cx } from '@/lib/cx'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { DocType, Ref, StockDoc } from './types'

/**
 * เอกสารสต็อก (สร้าง / แก้ร่าง): รับเข้า / เบิกออก / โอนระหว่างคลัง / ปรับปรุงยอด
 *   - ต้นทุนต่อหน่วย: รับเข้า = ต้องมี; ปรับปรุงยอดจำนวนบวก = ต้องมี; อื่นๆ ใช้ FIFO ตอนผ่านเอกสาร
 *   - ปรับปรุงยอดใส่จำนวนลบได้ (= ลดสต็อก)
 *   - "บันทึกและผ่านเอกสาร": สร้างใหม่ = POST status=posted; แก้ร่าง = PUT แล้ว PATCH posted
 */
const TYPES: DocType[] = ['receipt', 'issue', 'transfer', 'adjustment']

type Line = { key: number; item: Ref | null; quantity: string; unitCost: string; description: string }
let nextKey = 1
const newLine = (p: Partial<Line> = {}): Line => ({ key: nextKey++, item: null, quantity: '', unitCost: '', description: '', ...p })

/** ต้องระบุต้นทุนหรือไม่ — รับเข้าเสมอ, ปรับปรุงยอดเมื่อจำนวนเป็นบวก */
export function needsCost(type: DocType, quantity: string): boolean {
  if (type === 'receipt') return true
  if (type !== 'adjustment') return false
  const q = normalizeAmount(quantity)
  return !!q && toUnits(q) > 0n
}

export function StockDocForm({
  apiPath,
  itemsApi,
  warehousesApi,
  docsPath,
  listHref,
  canPost,
  doc,
}: {
  apiPath: string
  itemsApi: string
  warehousesApi: string
  docsPath: string
  listHref: string
  canPost: boolean
  doc?: StockDoc
}) {
  const t = texts.inventory.documents
  const tl = t.lines
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [type, setType] = useState<DocType>(doc?.type ?? 'receipt')
  const [head, setHead] = useState({ date: doc?.date ?? today(), reference: doc?.reference ?? '', description: doc?.description ?? '' })
  const [warehouse, setWarehouse] = useState<Ref | null>(doc?.warehouse ?? null)
  const [toWarehouse, setToWarehouse] = useState<Ref | null>(doc?.to_warehouse ?? null)
  const [lines, setLines] = useState<Line[]>(() =>
    doc ? doc.lines.map((l) => newLine({ item: l.item, quantity: l.quantity, unitCost: l.unit_cost ?? '', description: l.description })) : [newLine()],
  )
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<'draft' | 'post' | null>(null)
  const transfer = type === 'transfer'
  const costColumn = type === 'receipt' || type === 'adjustment'
  const up = (key: number, patch: Partial<Line>) => setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))

  function lineValue(l: Line): bigint | null {
    if (!needsCost(type, l.quantity)) return null
    const q = normalizeAmount(l.quantity)
    const c = normalizeAmount(l.unitCost)
    if (!q || c === null || c === '') return null
    return (toUnits(q) * toUnits(c)) / 1_000_000n
  }
  const total = lines.reduce((s, l) => s + (lineValue(l) ?? 0n), 0n)

  async function save(post: boolean) {
    setError(null)
    const errs: Record<string, string> = {}
    if (!head.date) errs.date = requiredMessage(t.fields.date)
    if (!warehouse) errs.warehouse = requiredMessage(transfer ? t.fields.fromWarehouse : t.fields.warehouse)
    if (transfer && !toWarehouse) errs.toWarehouse = requiredMessage(t.fields.toWarehouse)
    if (lines.length === 0) errs.lines = tl.noLines
    for (const l of lines) {
      if (!l.item) errs[`item-${l.key}`] = tl.itemRequired
      const q = normalizeAmount(l.quantity)
      const neg = q !== null && q !== '' && toUnits(q) < 0n
      if (!q || toUnits(q) === 0n || (neg && type !== 'adjustment')) errs[`qty-${l.key}`] = tl.qtyInvalid
      if (needsCost(type, l.quantity)) {
        const c = normalizeAmount(l.unitCost)
        if (c === null || c === '' || toUnits(c) < 0n) errs[`cost-${l.key}`] = tl.costRequired
      }
    }
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      type,
      date: head.date,
      warehouse_id: warehouse!.id,
      to_warehouse_id: transfer ? toWarehouse!.id : null,
      reference: head.reference.trim(),
      description: head.description.trim(),
      lines: lines.map((l) => ({
        item_id: l.item!.id,
        quantity: normalizeAmount(l.quantity),
        unit_cost: needsCost(type, l.quantity) ? normalizeAmount(l.unitCost) : null,
        description: l.description.trim(),
      })),
    }
    setBusy(post ? 'post' : 'draft')
    try {
      let id = doc?.id
      if (doc) {
        await apiRequest(`${apiPath}/${doc.id}`, { method: 'PUT', body })
        if (post) await apiRequest(`${apiPath}/${doc.id}`, { method: 'PATCH', body: { status: 'posted' } })
      } else {
        id = (await apiRequest<{ id: string }>(apiPath, { method: 'POST', body: { ...body, status: post ? 'posted' : 'draft' } })).id
      }
      showToast(texts.sales.toast.saved)
      router.push(`${docsPath}/${id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(null)
    }
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    void save(false)
  }

  const cell = 'px-2 py-2'
  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={t.fields.type}>
        <fieldset className="grid gap-2 sm:col-span-2 sm:grid-cols-2 lg:grid-cols-4">
          <legend className="sr-only">{t.fields.type}</legend>
          {TYPES.map((k) => (
            <label
              key={k}
              className={cx(
                'flex cursor-pointer items-center gap-2 rounded-(--radius-control) border px-3 py-2.5 text-sm transition-colors',
                type === k ? 'border-primary bg-primary-soft font-medium text-primary' : 'border-line-strong bg-surface hover:border-ink-subtle',
              )}
            >
              <input type="radio" name="doc-type" value={k} checked={type === k} onChange={() => setType(k)} className="size-4 accent-primary" />
              {t.types[k]}
            </label>
          ))}
        </fieldset>
      </FormSection>

      <FormSection title={texts.sales.sections.header}>
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <ReferenceSelect
          label={transfer ? t.fields.fromWarehouse : t.fields.warehouse}
          required
          source={warehousesApi}
          params={{ status: 'active' }}
          value={warehouse}
          onChange={setWarehouse}
          error={errors.warehouse}
          texts={texts.reference}
        />
        {transfer && (
          <ReferenceSelect
            label={t.fields.toWarehouse}
            required
            source={warehousesApi}
            params={{ status: 'active' }}
            value={toWarehouse}
            onChange={setToWarehouse}
            error={errors.toWarehouse}
            texts={texts.reference}
          />
        )}
        <TextField label={t.fields.reference} value={head.reference} maxLength={100} onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))} />
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <section aria-labelledby="stock-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="stock-lines" className="text-[15px] font-semibold">
          {tl.title}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[48rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                  #
                </th>
                <th scope="col" className="w-[32%] px-2 py-2.5 font-medium">
                  {tl.item}
                </th>
                <th scope="col" className="w-28 px-2 py-2.5 text-right font-medium">
                  {tl.quantity}
                </th>
                {costColumn && (
                  <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                    {tl.unitCost}
                  </th>
                )}
                <th scope="col" className="w-32 px-2 py-2.5 text-right font-medium">
                  {tl.value}
                </th>
                <th scope="col" className="px-2 py-2.5 font-medium">
                  {tl.description}
                </th>
                <th scope="col" className="w-12 px-2 py-2.5">
                  <span className="sr-only">{texts.common.delete}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {lines.map((l, i) => {
                const n = i + 1
                const label = (f: string) => `${f} ${format(tl.line, { n })}`
                const v = lineValue(l)
                const cost = needsCost(type, l.quantity)
                return (
                  <tr key={l.key} className="border-b border-line align-top last:border-b-0">
                    <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{n}</td>
                    <td className={cell}>
                      <ReferenceSelect
                        label={label(tl.item)}
                        hideLabel
                        source={itemsApi}
                        params={{ status: 'active', type: 'stock' }}
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
                    {costColumn && (
                      <td className={cell}>
                        {cost ? (
                          <TextField
                            label={label(tl.unitCost)}
                            hideLabel
                            inputMode="decimal"
                            value={l.unitCost}
                            error={errors[`cost-${l.key}`]}
                            className="[&_input]:text-right [&_input]:tabular-nums"
                            onChange={(e) => up(l.key, { unitCost: e.target.value })}
                          />
                        ) : (
                          <p className="pt-2.5 text-right text-[13px] text-ink-muted">{tl.fifo}</p>
                        )}
                      </td>
                    )}
                    <td className="px-2 pt-4 text-right tabular-nums">{v === null ? <span className="text-[13px] text-ink-muted">{tl.fifo}</span> : formatAmount(v)}</td>
                    <td className={cell}>
                      <TextField label={label(tl.description)} hideLabel value={l.description} maxLength={500} onChange={(e) => up(l.key, { description: e.target.value })} />
                    </td>
                    <td className={cell}>
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
        </div>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <Button size="sm" variant="secondary" onClick={() => setLines((ls) => [...ls, newLine()])}>
            <Plus aria-hidden />
            {tl.add}
          </Button>
          {costColumn && (
            <p aria-live="polite" className="text-sm">
              <span className="text-ink-muted">{t.totalValue}</span> <span className="ml-2 font-semibold tabular-nums">{formatAmount(total)}</span>
            </p>
          )}
        </div>
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <FormBar>
        <div className="ml-auto flex flex-wrap gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" variant={canPost ? 'secondary' : 'primary'} loading={busy === 'draft'} loadingText={texts.common.saving} disabled={busy !== null}>
            {t.actions.saveDraft}
          </Button>
          {canPost && (
            <Button
              type="button"
              loading={busy === 'post'}
              loadingText={texts.common.saving}
              disabled={busy !== null}
              onClick={() => {
                if (window.confirm(t.confirm.post)) void save(true)
              }}
            >
              {t.actions.savePost}
            </Button>
          )}
        </div>
      </FormBar>
    </form>
  )
}
