'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Button, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, sum, toUnits } from '@/features/accounting/money'
import { FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { focusFirstInvalid, normalizeCode, requiredMessage } from '@/lib/validation'
import type { Bom, Ref } from './types'

/**
 * สูตรการผลิต (BOM): สินค้าที่ผลิต + จำนวนต่อรอบ, วัตถุดิบ (จำนวนต่อรอบ + % สูญเสีย), ค่าแปลงสภาพต่อรอบ (บัญชี + จำนวนเงิน)
 * body ตรงกับ POST / PUT /boms — ตรวจซ้ำที่ backend (วัตถุดิบซ้ำ / ใช้ตัวเองเป็นวัตถุดิบ / จำนวน > 0)
 */
type Component = { key: number; item: Ref | null; quantity: string; scrap: string }
type Conversion = { key: number; description: string; account: Ref | null; amount: string }

let nextKey = 1
const newComponent = (p: Partial<Component> = {}): Component => ({ key: nextKey++, item: null, quantity: '', scrap: '0', ...p })
const newConversion = (p: Partial<Conversion> = {}): Conversion => ({ key: nextKey++, description: '', account: null, amount: '', ...p })

const positive = (v: string) => {
  const n = normalizeAmount(v)
  return !!n && toUnits(n) > 0n
}

export function BomForm({
  apiPath,
  listHref,
  itemsApi,
  accountsApi,
  canManage,
  bom,
}: {
  apiPath: string
  listHref: string
  itemsApi: string
  accountsApi: string
  canManage: boolean
  bom?: Bom
}) {
  const t = texts.manufacturing.boms
  const f = t.fields
  const e = t.errors
  const readOnly = !canManage
  const formRef = useRef<HTMLFormElement>(null)
  const form = useResourceForm({ apiPath, id: bom?.id, listHref })
  const [head, setHead] = useState({
    code: bom?.code ?? '',
    name: bom?.name ?? '',
    output: bom?.output_quantity ?? '1',
    status: bom?.status ?? 'active',
    isDefault: bom?.is_default ?? true,
  })
  const [item, setItem] = useState<Ref | null>(bom?.item ?? null)
  const [components, setComponents] = useState<Component[]>(() =>
    bom?.components?.length ? bom.components.map((c) => newComponent({ item: c.item, quantity: c.quantity, scrap: c.scrap_percent })) : [newComponent()],
  )
  const [conversions, setConversions] = useState<Conversion[]>(() =>
    (bom?.conversion_costs ?? []).map((c) => newConversion({ description: c.description, account: c.account, amount: c.amount })),
  )
  const [errors, setErrors] = useState<Record<string, string>>({})
  const upC = (key: number, patch: Partial<Component>) => setComponents((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))
  const upV = (key: number, patch: Partial<Conversion>) => setConversions((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))
  const conversionTotal = sum(conversions.map((c) => normalizeAmount(c.amount) || '0'))

  function validate(): Record<string, string> {
    const errs: Record<string, string> = {}
    if (!head.code) errs.code = requiredMessage(f.code)
    if (!head.name.trim()) errs.name = requiredMessage(f.name)
    if (!item) errs.item = e.itemRequired
    if (!positive(head.output)) errs.output = e.qtyInvalid
    if (components.length === 0) errs.components = e.noComponents
    const seen = new Set<string>()
    for (const c of components) {
      if (!c.item) errs[`c-item-${c.key}`] = e.itemRequired
      else if (item && c.item.id === item.id) errs[`c-item-${c.key}`] = e.selfComponent
      else if (seen.has(c.item.id)) errs[`c-item-${c.key}`] = e.duplicate
      if (c.item) seen.add(c.item.id)
      if (!positive(c.quantity)) errs[`c-qty-${c.key}`] = e.qtyInvalid
      const s = normalizeAmount(c.scrap || '0')
      if (s === null || toUnits(s) < 0n || toUnits(s) > 100_000_000n) errs[`c-scrap-${c.key}`] = e.scrapInvalid
    }
    for (const v of conversions) {
      if (!v.description.trim()) errs[`v-desc-${v.key}`] = e.descriptionRequired
      if (!v.account) errs[`v-acc-${v.key}`] = e.accountRequired
      if (!positive(v.amount)) errs[`v-amt-${v.key}`] = e.amountInvalid
    }
    return errs
  }

  function submit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    if (readOnly) return
    const errs = validate()
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    void form.save({
      code: head.code,
      name: head.name.trim(),
      item_id: item!.id,
      output_quantity: normalizeAmount(head.output),
      status: head.status,
      is_default: head.isDefault,
      components: components.map((c) => ({ item_id: c.item!.id, quantity: normalizeAmount(c.quantity), scrap_percent: normalizeAmount(c.scrap || '0') })),
      conversion_costs: conversions.map((v) => ({ description: v.description.trim(), account_id: v.account!.id, amount: normalizeAmount(v.amount) })),
    })
  }

  const num = '[&_input]:text-right [&_input]:tabular-nums'
  const line = (label: string, n: number) => `${label} ${format(f.line, { n })}`

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.general} description={t.sections.generalHint}>
        <TextField
          label={f.code}
          required
          value={head.code}
          maxLength={30}
          disabled={readOnly}
          error={errors.code}
          onChange={(ev) => setHead((h) => ({ ...h, code: normalizeCode(ev.target.value) }))}
        />
        <TextField label={f.name} required value={head.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(ev) => setHead((h) => ({ ...h, name: ev.target.value }))} />
        <ReferenceSelect
          label={f.item}
          required
          source={itemsApi}
          params={{ status: 'active', type: 'stock' }}
          value={item}
          onChange={setItem}
          disabled={readOnly}
          error={errors.item}
          texts={texts.reference}
        />
        <TextField
          label={f.outputQty}
          required
          inputMode="decimal"
          value={head.output}
          disabled={readOnly}
          error={errors.output}
          className={num}
          onChange={(ev) => setHead((h) => ({ ...h, output: ev.target.value }))}
        />
        {bom && <StatusSelect value={head.status} disabled={readOnly} onChange={(status) => setHead((h) => ({ ...h, status: status as Bom['status'] }))} />}
        <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
          <input
            type="checkbox"
            checked={head.isDefault}
            disabled={readOnly}
            onChange={(ev) => setHead((h) => ({ ...h, isDefault: ev.target.checked }))}
            className="mt-0.5 size-4 accent-primary"
          />
          <span className="text-sm font-medium">{f.isDefault}</span>
        </label>
      </FormSection>

      <LinesSection id="bom-components" title={t.sections.components} hint={t.sections.componentsHint}>
        <EditTable head={[['#', 'w-10 text-center'], [f.component, 'w-[45%]'], [f.quantity, 'w-36 text-right'], [f.scrap, 'w-28 text-right'], ['', 'w-12']]}>
          {components.map((c, i) => (
            <tr key={c.key} className="border-b border-line align-top last:border-b-0">
              <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{i + 1}</td>
              <td className="px-2 py-2">
                <ReferenceSelect
                  label={line(f.component, i + 1)}
                  hideLabel
                  source={itemsApi}
                  params={{ status: 'active', type: 'stock' }}
                  value={c.item}
                  onChange={(v) => upC(c.key, { item: v })}
                  disabled={readOnly}
                  error={errors[`c-item-${c.key}`]}
                  texts={texts.reference}
                />
              </td>
              <td className="px-2 py-2">
                <TextField
                  label={line(f.quantity, i + 1)}
                  hideLabel
                  inputMode="decimal"
                  value={c.quantity}
                  disabled={readOnly}
                  error={errors[`c-qty-${c.key}`]}
                  className={num}
                  onChange={(ev) => upC(c.key, { quantity: ev.target.value })}
                />
              </td>
              <td className="px-2 py-2">
                <TextField
                  label={line(f.scrap, i + 1)}
                  hideLabel
                  inputMode="decimal"
                  value={c.scrap}
                  disabled={readOnly}
                  error={errors[`c-scrap-${c.key}`]}
                  className={num}
                  onChange={(ev) => upC(c.key, { scrap: ev.target.value })}
                />
              </td>
              <td className="px-2 py-2">
                {!readOnly && (
                  <RemoveButton label={format(t.remove, { n: i + 1 })} disabled={components.length <= 1} onClick={() => setComponents((ls) => ls.filter((x) => x.key !== c.key))} />
                )}
              </td>
            </tr>
          ))}
        </EditTable>
        {!readOnly && (
          <div>
            <Button size="sm" variant="secondary" onClick={() => setComponents((ls) => [...ls, newComponent()])}>
              <Plus aria-hidden />
              {t.addComponent}
            </Button>
          </div>
        )}
        {errors.components && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.components}
          </p>
        )}
      </LinesSection>

      <LinesSection id="bom-conversion" title={t.sections.conversion} hint={t.sections.conversionHint}>
        {conversions.length === 0 ? (
          <p className="rounded-(--radius-panel) border border-dashed border-line-strong bg-surface px-4 py-5 text-center text-sm text-ink-muted">{t.noConversion}</p>
        ) : (
          <EditTable head={[['#', 'w-10 text-center'], [f.description, ''], [f.account, 'w-[34%]'], [f.amount, 'w-40 text-right'], ['', 'w-12']]}>
            {conversions.map((v, i) => (
              <tr key={v.key} className="border-b border-line align-top last:border-b-0">
                <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{i + 1}</td>
                <td className="px-2 py-2">
                  <TextField
                    label={line(f.description, i + 1)}
                    hideLabel
                    value={v.description}
                    maxLength={200}
                    disabled={readOnly}
                    error={errors[`v-desc-${v.key}`]}
                    onChange={(ev) => upV(v.key, { description: ev.target.value })}
                  />
                </td>
                <td className="px-2 py-2">
                  <ReferenceSelect
                    label={line(f.account, i + 1)}
                    hideLabel
                    source={accountsApi}
                    params={{ postable: 'true', status: 'active', type: 'expense' }}
                    value={v.account}
                    onChange={(a) => upV(v.key, { account: a })}
                    disabled={readOnly}
                    error={errors[`v-acc-${v.key}`]}
                    texts={texts.reference}
                  />
                </td>
                <td className="px-2 py-2">
                  <TextField
                    label={line(f.amount, i + 1)}
                    hideLabel
                    inputMode="decimal"
                    value={v.amount}
                    disabled={readOnly}
                    error={errors[`v-amt-${v.key}`]}
                    className={num}
                    onChange={(ev) => upV(v.key, { amount: ev.target.value })}
                  />
                </td>
                <td className="px-2 py-2">
                  {!readOnly && <RemoveButton label={format(t.remove, { n: i + 1 })} onClick={() => setConversions((ls) => ls.filter((x) => x.key !== v.key))} />}
                </td>
              </tr>
            ))}
          </EditTable>
        )}
        <div className="flex flex-wrap items-center justify-between gap-4">
          {!readOnly ? (
            <Button size="sm" variant="secondary" onClick={() => setConversions((ls) => [...ls, newConversion()])}>
              <Plus aria-hidden />
              {t.addConversion}
            </Button>
          ) : (
            <span />
          )}
          <p aria-live="polite" className="text-sm">
            <span className="text-ink-muted">{t.conversionTotal}</span> <span className="ml-2 font-semibold tabular-nums">{formatAmount(conversionTotal)}</span>
          </p>
        </div>
      </LinesSection>

      <FormActions listHref={listHref} canManage={canManage} canDelete={!!bom} busy={form.busy} onDelete={() => form.remove(format(t.deleteConfirm, { code: bom?.code ?? '' }))} />
    </form>
  )
}

function LinesSection({ id, title, hint, children }: { id: string; title: string; hint: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col gap-3">
      <div>
        <h2 id={id} className="text-[15px] font-semibold">
          {title}
        </h2>
        <p className="mt-1 text-[13px] text-ink-muted">{hint}</p>
      </div>
      {children}
    </section>
  )
}

export function EditTable({ head, children }: { head: [string, string][]; children: ReactNode }) {
  return (
    <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <table className="w-full min-w-[40rem] border-collapse text-sm">
        <thead>
          <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
            {head.map(([h, cls], i) => (
              <th key={i} scope="col" className={`px-2 py-2.5 font-medium ${cls}`}>
                {h || <span className="sr-only">{texts.common.delete}</span>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  )
}

function RemoveButton({ label, disabled, onClick }: { label: string; disabled?: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40"
    >
      <Trash2 aria-hidden className="size-4" />
    </button>
  )
}
