'use client'

import { CircleAlert, CircleCheck, Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toString, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { cx } from '@/lib/cx'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { OpenInvoice, Ref, WhtType } from './types'

/**
 * ฟอร์มรับเงิน / จ่ายเงิน (ใช้ร่วม — สองเอกสารเป็นภาพสะท้อนกัน)
 *
 * - ชนิด: ชำระหนี้คู่ค้า (เลือกลูกค้า / ผู้ขาย -> แสดงใบแจ้งหนี้ค้าง -> ติ๊กเลือก + ใส่ยอดที่ตัด) หรือรับ / จ่ายเงินอื่น (ระบุบัญชีเอง)
 * - ภาษีหัก ณ ที่จ่าย: รับเงิน = กรอกยอดที่ลูกค้าหัก, จ่ายเงิน = เลือกประเภท (คำนวณตัวอย่างจากยอดก่อน VAT — backend คำนวณจริงอีกครั้ง)
 * - ช่องทาง: หลายบัญชีเงินสด / ธนาคาร — สถานะ "ตรง / ต่าง" แสดงทั้งไอคอนและข้อความ, ปุ่ม "ใส่ยอดที่เหลือ"
 * - บันทึก = ลงบัญชีทันที (backend ตรวจกฎทั้งหมดซ้ำใน transaction เดียว)
 */

export type MoneyDocMode = 'receipt' | 'payment'

export type MoneyDocApi = {
  submit: string
  open: string // .../open-invoices
  parties: string // customers / suppliers
  accounts: string
  cashAccounts: string
  whtTypes?: string // จ่ายเงินเท่านั้น
}

type Selected = { on: boolean; amount: string; wht: string; whtType: string }
type OtherLine = { key: number; account: Ref | null; description: string; amount: string; wht: string; whtType: string }
type MethodLine = { key: number; cash: Ref | null; amount: string; reference: string }

let nextKey = 1
const otherLine = (): OtherLine => ({ key: nextKey++, account: null, description: '', amount: '', wht: '', whtType: '' })
const methodLine = (): MethodLine => ({ key: nextKey++, cash: null, amount: '', reference: '' })

/** ปัดหน่วยย่อย (สเกล 10^6) เป็นทศนิยม 2 ตำแหน่ง แบบปัดครึ่งขึ้น */
function round2(units: bigint): bigint {
  const q = 10_000n
  return units >= 0n ? ((units + q / 2n) / q) * q : -((-units + q / 2n) / q) * q
}

const SCALE = 1_000_000n

/** ตัวอย่างภาษีหัก ณ ที่จ่าย — สูตรเดียวกับ backend (ฐานตามสัดส่วนก่อน VAT ของใบแจ้งหนี้) */
export function previewWht(amount: bigint, rate: bigint, invoice?: { net: bigint; gross: bigint }): bigint {
  let base = amount
  if (invoice) base = invoice.gross === 0n ? 0n : amount === invoice.gross ? invoice.net : round2((invoice.net * amount) / invoice.gross)
  return round2((base * rate) / (100n * SCALE))
}

export function MoneyDocForm({ mode, api, docsPath, listHref }: { mode: MoneyDocMode; api: MoneyDocApi; docsPath: string; listHref: string }) {
  const t = texts.finance
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const partyKind = mode === 'receipt' ? 'customer' : 'supplier'
  const kinds = mode === 'receipt' ? t.kinds.receipt : t.kinds.payment
  const kindHints = mode === 'receipt' ? t.kindHints.receipt : t.kindHints.payment
  const partyLabel = mode === 'receipt' ? t.fields.customer : t.fields.supplier

  const [kind, setKind] = useState<string>(partyKind)
  const [head, setHead] = useState({ date: today(), description: '' })
  const [party, setParty] = useState<Ref | null>(null)
  const [payee, setPayee] = useState({ name: '', tax_id: '', branch_no: '', address: '', tax_type: '' })
  const [invoices, setInvoices] = useState<OpenInvoice[] | null>(null)
  const [loadingInvoices, setLoadingInvoices] = useState(false)
  const [selected, setSelected] = useState<Record<string, Selected>>({})
  const [others, setOthers] = useState<OtherLine[]>([otherLine()])
  const [methods, setMethods] = useState<MethodLine[]>([methodLine()])
  const [whtTypes, setWhtTypes] = useState<WhtType[]>([])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const isParty = kind === partyKind

  useEffect(() => {
    if (!api.whtTypes) return
    apiRequest<WhtType[]>(`${api.whtTypes}?page_size=100`).then(setWhtTypes, () => setWhtTypes([]))
  }, [api.whtTypes])

  useEffect(() => {
    if (!party) {
      setInvoices(null)
      return
    }
    let live = true
    setLoadingInvoices(true)
    apiRequest<OpenInvoice[]>(`${api.open}?${partyKind}_id=${encodeURIComponent(party.id)}`)
      .then((rows) => live && (setInvoices(rows), setSelected({})))
      .catch((err: unknown) => live && (setInvoices([]), setFormError(errorMessage(err))))
      .finally(() => live && setLoadingInvoices(false))
    return () => {
      live = false
    }
  }, [party, api.open, partyKind])

  const rateOf = (id: string) => toUnits(whtTypes.find((w) => w.id === id)?.rate ?? '0')

  // ---------- ยอดรวม ----------
  const lineTotals = isParty
    ? (invoices ?? [])
        .filter((inv) => selected[inv.id]?.on)
        .map((inv) => {
          const s = selected[inv.id]!
          const amount = toUnits(s.amount)
          const wht =
            mode === 'receipt' ? toUnits(s.wht) : s.whtType ? previewWht(amount, rateOf(s.whtType), { net: toUnits(inv.net), gross: toUnits(inv.gross) }) : 0n
          return { amount, wht }
        })
    : others.map((l) => {
        const amount = toUnits(l.amount)
        return { amount, wht: mode === 'payment' && l.whtType ? previewWht(amount, rateOf(l.whtType)) : 0n }
      })
  const amount = lineTotals.reduce((a, l) => a + l.amount, 0n)
  const wht = lineTotals.reduce((a, l) => a + l.wht, 0n)
  const net = amount - wht
  const methodsTotal = methods.reduce((a, m) => a + toUnits(m.amount), 0n)
  const diff = net - methodsTotal
  const balanced = net > 0n && diff === 0n

  const setSel = (id: string, patch: Partial<Selected>) =>
    setSelected((s) => ({ ...s, [id]: { on: false, amount: '', wht: '', whtType: '', ...s[id], ...patch } }))
  const toggle = (inv: OpenInvoice, on: boolean) => setSel(inv.id, on ? { on, amount: selected[inv.id]?.amount || inv.outstanding } : { on })
  const allOn = !!invoices?.length && invoices.every((inv) => selected[inv.id]?.on)

  function fillRemaining() {
    if (diff === 0n || methods.length === 0) return
    setMethods((ms) => ms.map((m, i) => (i === ms.length - 1 ? { ...m, amount: toString(toUnits(m.amount) + diff) } : m)))
  }

  function validate() {
    const e: Record<string, string> = {}
    if (!head.date) e.date = requiredMessage(t.fields.date)
    if (isParty && !party) e.party = format(t.errors.partyRequired, { field: partyLabel })
    if (!isParty && !payee.name.trim()) e.payee = format(t.errors.nameRequired, { field: mode === 'receipt' ? t.fields.payerName : t.fields.payeeName })
    if (isParty) {
      for (const inv of invoices ?? []) {
        const s = selected[inv.id]
        if (!s?.on) continue
        const n = normalizeAmount(s.amount)
        if (!n || toUnits(n) <= 0n) e[`inv-${inv.id}`] = t.errors.amountRequired
        else if (toUnits(n) > toUnits(inv.outstanding)) e[`inv-${inv.id}`] = t.errors.overOutstanding
      }
    } else {
      for (const l of others) {
        if (!l.account) e[`acc-${l.key}`] = t.errors.accountRequired
        const n = normalizeAmount(l.amount)
        if (!n || toUnits(n) <= 0n) e[`amt-${l.key}`] = t.errors.amountRequired
      }
    }
    if (lineTotals.length === 0) e.lines = t.errors.noLines
    for (const m of methods) {
      if (!m.cash) e[`cash-${m.key}`] = requiredMessage(t.fields.cashAccount)
      const n = normalizeAmount(m.amount)
      if (!n || toUnits(n) <= 0n) e[`mamt-${m.key}`] = t.errors.amountRequired
    }
    if (!e.lines && !balanced) e.methods = t.errors.methodsMismatch
    setErrors(e)
    return Object.keys(e).length === 0
  }

  async function submit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    setFormError(null)
    if (!validate()) return focusFirstInvalid(formRef.current)
    const partyField = mode === 'receipt' ? 'payer' : 'payee'
    const body: Record<string, unknown> = {
      kind,
      date: head.date,
      description: head.description.trim(),
      methods: methods.map((m) => ({ cash_account_id: m.cash!.id, amount: normalizeAmount(m.amount), reference: m.reference.trim() })),
    }
    if (isParty) {
      body[`${partyKind}_id`] = party!.id
      body.lines = (invoices ?? [])
        .filter((inv) => selected[inv.id]?.on)
        .map((inv) => {
          const s = selected[inv.id]!
          return mode === 'receipt'
            ? { invoice_id: inv.id, amount: normalizeAmount(s.amount), wht: normalizeAmount(s.wht) || '0' }
            : { invoice_id: inv.id, amount: normalizeAmount(s.amount), wht_type_id: s.whtType || null }
        })
    } else {
      body[partyField] = { ...payee, name: payee.name.trim(), tax_type: payee.tax_type || '' }
      body.lines = others.map((l) => ({
        account_id: l.account!.id,
        description: l.description.trim(),
        amount: normalizeAmount(l.amount),
        ...(mode === 'payment' ? { wht_type_id: l.whtType || null } : {}),
      }))
    }
    setBusy(true)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(api.submit, { method: 'POST', body })
      showToast(format(t.toast.saved, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  /** ป้ายของช่องในแถวช่องทาง: จอกว้างซ่อน (มีหัวคอลัมน์แล้ว) แต่ screen reader ยังอ่านชื่อเต็ม */
  const methodLabel = 'sm:[&>span:first-child]:sr-only'
  const amountField = 'w-36 [&_input]:text-right [&_input]:tabular-nums'

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}

      <FormSection title={t.fields.kind} description={kindHints[kind]}>
        <fieldset className="sm:col-span-2">
          <legend className="sr-only">{t.fields.kind}</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {Object.entries(kinds).map(([k, label]) => (
              <label
                key={k}
                className={cx(
                  'flex cursor-pointer items-start gap-3 rounded-(--radius-control) border px-3.5 py-3 transition-colors',
                  kind === k ? 'border-primary/60 bg-primary-soft/60' : 'border-line hover:bg-subtle',
                )}
              >
                <input type="radio" name="kind" value={k} checked={kind === k} onChange={() => setKind(k)} className="mt-1 size-4 accent-primary" />
                <span>
                  <span className="block text-sm font-medium">{label}</span>
                  <span className="block text-[13px] text-ink-muted">{kindHints[k]}</span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <FormSection title={t.sections.party} description={!isParty && mode === 'payment' ? t.payeeHint : t.sections.partyHint}>
        {isParty ? (
          <ReferenceSelect
            label={partyLabel}
            required
            source={api.parties}
            params={{ status: 'active' }}
            value={party}
            onChange={setParty}
            error={errors.party}
            texts={texts.reference}
            className="sm:col-span-2"
          />
        ) : (
          <>
            <TextField
              label={mode === 'receipt' ? t.fields.payerName : t.fields.payeeName}
              required
              value={payee.name}
              maxLength={200}
              error={errors.payee}
              onChange={(e) => setPayee((p) => ({ ...p, name: e.target.value }))}
            />
            <TextField label={t.fields.taxId} value={payee.tax_id} maxLength={20} inputMode="numeric" onChange={(e) => setPayee((p) => ({ ...p, tax_id: e.target.value }))} />
            {mode === 'payment' && (
              <SelectField
                label={t.fields.taxType}
                value={payee.tax_type}
                options={[{ value: '', label: texts.common.none }, ...Object.entries(t.taxTypes).map(([value, label]) => ({ value, label }))]}
                onChange={(e) => setPayee((p) => ({ ...p, tax_type: e.target.value }))}
              />
            )}
            <TextField label={t.fields.branchNo} value={payee.branch_no} maxLength={5} inputMode="numeric" onChange={(e) => setPayee((p) => ({ ...p, branch_no: e.target.value }))} />
            <TextField label={t.fields.address} value={payee.address} maxLength={500} className="sm:col-span-2" onChange={(e) => setPayee((p) => ({ ...p, address: e.target.value }))} />
          </>
        )}
      </FormSection>

      <section aria-labelledby="doc-lines" className="flex flex-col gap-3">
        <div>
          <h2 id="doc-lines" className="text-[15px] font-semibold">
            {isParty ? t.sections.invoices : t.sections.lines}
          </h2>
          {isParty && <p className="mt-0.5 text-[13px] text-ink-muted">{t.sections.invoicesHint}</p>}
        </div>
        <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          {isParty ? (
            !party || loadingInvoices || !invoices?.length ? (
              <p className="px-4 py-10 text-center text-sm text-ink-muted" aria-live="polite">
                {!party ? t.invoices.pickParty : loadingInvoices ? t.invoices.loading : t.invoices.none}
              </p>
            ) : (
              <table className="w-full min-w-[46rem] border-collapse text-sm">
                <thead>
                  <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                    <th scope="col" className="w-12 px-3 py-2.5">
                      <input
                        type="checkbox"
                        aria-label={t.invoices.selectAll}
                        checked={allOn}
                        onChange={(e) => invoices.forEach((inv) => toggle(inv, e.target.checked))}
                        className="size-4 cursor-pointer accent-primary"
                      />
                    </th>
                    <th scope="col" className="px-2 py-2.5 font-medium">
                      {t.invoices.no}
                    </th>
                    <th scope="col" className="px-2 py-2.5 font-medium">
                      {t.invoices.due}
                    </th>
                    <th scope="col" className="px-2 py-2.5 text-right font-medium">
                      {t.invoices.outstanding}
                    </th>
                    <th scope="col" className="px-2 py-2.5 text-right font-medium">
                      {t.invoices.pay}
                    </th>
                    <th scope="col" className="px-2 py-2.5 font-medium">
                      {mode === 'receipt' ? t.fields.wht : t.fields.whtType}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {invoices.map((inv) => {
                    const s = selected[inv.id]
                    return (
                      <tr key={inv.id} className={cx('border-b border-line last:border-b-0', s?.on && 'bg-primary-soft/30')}>
                        <td className="px-3 py-2">
                          <input
                            type="checkbox"
                            aria-label={format(t.invoices.select, { no: inv.document_no })}
                            checked={!!s?.on}
                            onChange={(e) => toggle(inv, e.target.checked)}
                            className="size-4 cursor-pointer accent-primary"
                          />
                        </td>
                        <td className="px-2 py-2">
                          <span className="font-mono text-[13px]">{inv.document_no}</span>
                          {inv.partner_ref && <span className="block text-[12px] text-ink-muted">{inv.partner_ref}</span>}
                        </td>
                        <td className="px-2 py-2 whitespace-nowrap">{formatDate(inv.due_date)}</td>
                        <td className="px-2 py-2 text-right tabular-nums">{formatAmount(inv.outstanding)}</td>
                        <td className="px-2 py-2">
                          <TextField
                            label={`${t.invoices.pay} ${inv.document_no}`}
                            hideLabel
                            inputMode="decimal"
                            disabled={!s?.on}
                            value={s?.amount ?? ''}
                            error={errors[`inv-${inv.id}`]}
                            className={cx(amountField, 'ml-auto')}
                            onChange={(e) => setSel(inv.id, { amount: e.target.value })}
                          />
                        </td>
                        <td className="px-2 py-2">
                          {mode === 'receipt' ? (
                            <TextField
                              label={`${t.fields.wht} ${inv.document_no}`}
                              hideLabel
                              inputMode="decimal"
                              disabled={!s?.on}
                              value={s?.wht ?? ''}
                              className={amountField}
                              onChange={(e) => setSel(inv.id, { wht: e.target.value })}
                            />
                          ) : (
                            <SelectField
                              label={`${t.fields.whtType} ${inv.document_no}`}
                              hideLabel
                              disabled={!s?.on}
                              value={s?.whtType ?? ''}
                              options={[{ value: '', label: t.noWht }, ...whtTypes.map((w) => ({ value: w.id, label: `${w.name} ${w.rate}%` }))]}
                              className="w-48"
                              onChange={(e) => setSel(inv.id, { whtType: e.target.value })}
                            />
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )
          ) : (
            <table className="w-full min-w-[46rem] border-collapse text-sm">
              <thead>
                <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                  <th scope="col" className="w-[32%] px-3 py-2.5 font-medium">
                    {t.fields.account}
                  </th>
                  <th scope="col" className="px-2 py-2.5 font-medium">
                    {t.fields.description}
                  </th>
                  <th scope="col" className="px-2 py-2.5 text-right font-medium">
                    {t.fields.amount}
                  </th>
                  {mode === 'payment' && (
                    <th scope="col" className="px-2 py-2.5 font-medium">
                      {t.fields.whtType}
                    </th>
                  )}
                  <th scope="col" className="w-12 px-2 py-2.5">
                    <span className="sr-only">{texts.common.delete}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {others.map((l, i) => {
                  const n = i + 1
                  const up = (patch: Partial<OtherLine>) => setOthers((ls) => ls.map((x) => (x.key === l.key ? { ...x, ...patch } : x)))
                  return (
                    <tr key={l.key} className="border-b border-line align-top last:border-b-0">
                      <td className="px-3 py-2">
                        <ReferenceSelect
                          label={`${t.fields.account} ${format(t.lines.line, { n })}`}
                          hideLabel
                          source={api.accounts}
                          params={{ postable: 'true', status: 'active' }}
                          value={l.account}
                          onChange={(a) => up({ account: a })}
                          error={errors[`acc-${l.key}`]}
                          texts={texts.reference}
                        />
                      </td>
                      <td className="px-2 py-2">
                        <TextField label={`${t.fields.description} ${format(t.lines.line, { n })}`} hideLabel value={l.description} maxLength={500} onChange={(e) => up({ description: e.target.value })} />
                      </td>
                      <td className="px-2 py-2">
                        <TextField
                          label={`${t.fields.amount} ${format(t.lines.line, { n })}`}
                          hideLabel
                          inputMode="decimal"
                          value={l.amount}
                          error={errors[`amt-${l.key}`]}
                          className={cx(amountField, 'ml-auto')}
                          onChange={(e) => up({ amount: e.target.value })}
                        />
                      </td>
                      {mode === 'payment' && (
                        <td className="px-2 py-2">
                          <SelectField
                            label={`${t.fields.whtType} ${format(t.lines.line, { n })}`}
                            hideLabel
                            value={l.whtType}
                            options={[{ value: '', label: t.noWht }, ...whtTypes.map((w) => ({ value: w.id, label: `${w.name} ${w.rate}%` }))]}
                            className="w-48"
                            onChange={(e) => up({ whtType: e.target.value })}
                          />
                        </td>
                      )}
                      <td className="px-2 py-2">
                        <button
                          type="button"
                          aria-label={format(t.lines.remove, { n })}
                          disabled={others.length <= 1}
                          onClick={() => setOthers((ls) => ls.filter((x) => x.key !== l.key))}
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
        {!isParty && (
          <div>
            <Button size="sm" variant="secondary" onClick={() => setOthers((ls) => [...ls, otherLine()])}>
              <Plus aria-hidden />
              {t.lines.add}
            </Button>
          </div>
        )}
        {errors.lines && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <section aria-labelledby="doc-methods" className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-3">
          <div>
            <h2 id="doc-methods" className="text-[15px] font-semibold">
              {t.sections.methods}
            </h2>
            <p className="mt-0.5 text-[13px] text-ink-muted">{t.sections.methodsHint}</p>
          </div>
          <div className="flex flex-col gap-3 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card">
            <div aria-hidden className="hidden gap-3 text-[13px] font-medium text-ink-muted sm:grid sm:grid-cols-[minmax(0,1fr)_9rem_minmax(0,10rem)_2.5rem]">
              <span>{t.fields.cashAccount}</span>
              <span className="text-right">{t.fields.amount}</span>
              <span>{t.fields.reference}</span>
            </div>
            {methods.map((m, i) => {
              const n = i + 1
              const up = (patch: Partial<MethodLine>) => setMethods((ms) => ms.map((x) => (x.key === m.key ? { ...x, ...patch } : x)))
              return (
                <div key={m.key} className="grid items-start gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_minmax(0,10rem)_2.5rem]">
                  <ReferenceSelect
                    label={`${t.fields.cashAccount} ${format(t.lines.method, { n })}`}
                    className={methodLabel}
                    source={api.cashAccounts}
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
                    onClick={() => setMethods((ms) => ms.filter((x) => x.key !== m.key))}
                    className={cx(
                      'flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger disabled:cursor-not-allowed disabled:opacity-40',
                                    )}
                  >
                    <Trash2 aria-hidden className="size-4" />
                  </button>
                </div>
              )
            })}
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="secondary" onClick={() => setMethods((ms) => [...ms, methodLine()])}>
                <Plus aria-hidden />
                {t.lines.addMethod}
              </Button>
              {diff !== 0n && net > 0n && (
                <Button size="sm" variant="ghost" onClick={fillRemaining}>
                  {t.totals.fill}
                </Button>
              )}
            </div>
          </div>
        </div>

        <aside aria-label={t.totals.net} className="h-fit rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card lg:mt-12">
          <dl className="flex flex-col gap-2 text-sm">
            {(
              [
                [t.totals.amount, amount],
                [t.totals.wht, -wht],
              ] as const
            ).map(([label, v]) => (
              <div key={label} className="flex justify-between gap-4">
                <dt className="text-ink-muted">{label}</dt>
                <dd className="tabular-nums">{formatAmount(v)}</dd>
              </div>
            ))}
            <div className="flex justify-between gap-4 border-t border-line pt-2 font-semibold">
              <dt>{t.totals.net}</dt>
              <dd className="tabular-nums">{formatAmount(net)}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-ink-muted">{t.totals.methods}</dt>
              <dd className="tabular-nums">{formatAmount(methodsTotal)}</dd>
            </div>
          </dl>
          <p aria-live="polite" className="mt-3 text-[13px] font-medium">
            {balanced ? (
              <span className="inline-flex items-center gap-1.5 text-success">
                <CircleCheck aria-hidden className="size-4" />
                {t.totals.balanced}
              </span>
            ) : (
              net > 0n && (
                <span className="inline-flex items-center gap-1.5 text-warning">
                  <CircleAlert aria-hidden className="size-4" />
                  {format(t.totals.unbalanced, { amount: formatAmount(diff < 0n ? -diff : diff) })}
                </span>
              )
            )}
          </p>
          {errors.methods && (
            <p role="alert" className="mt-1 text-[13px] text-danger">
              {errors.methods}
            </p>
          )}
        </aside>
      </section>

      <FormBar>
        <p className="hidden text-[13px] text-ink-muted sm:block">{t.actions.saveHint}</p>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.save}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
