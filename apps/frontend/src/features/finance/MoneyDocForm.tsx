'use client'

import { Plus, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Badge, Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { cx } from '@/lib/cx'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { methodLine, methodsBody, MethodsBalance, MethodsEditor, methodsTotal, validateMethods, type MethodLine } from './MethodsEditor'
import type { OpenInvoice, Ref, WhtType } from './types'

/**
 * ฟอร์มรับเงิน / จ่ายเงิน (ใช้ร่วม — สองเอกสารเป็นภาพสะท้อนกัน)
 *
 * - ชนิด: ชำระหนี้คู่ค้า (เลือกลูกค้า / ผู้ขาย -> แสดงใบแจ้งหนี้ค้าง -> ติ๊กเลือก + ใส่ยอดที่ตัด) หรือรับ / จ่ายเงินอื่น (ระบุบัญชีเอง)
 * - ภาษีหัก ณ ที่จ่าย: รับเงิน = กรอกยอดที่ลูกค้าหัก, จ่ายเงิน = เลือกประเภท (คำนวณตัวอย่างจากยอดก่อน VAT — backend คำนวณจริงอีกครั้ง)
 * - ช่องทาง: หลายบัญชีเงินสด / ธนาคาร — สถานะ "ตรง / ต่าง" แสดงทั้งไอคอนและข้อความ, ปุ่ม "ใส่ยอดที่เหลือ"
 * - บันทึก = ลงบัญชีทันที (backend ตรวจกฎทั้งหมดซ้ำใน transaction เดียว)
 * - รับชำระหนี้ลูกค้า: เลือกใบวางบิลเพื่อเลือกใบแจ้งหนี้ให้อัตโนมัติ (?customer_id=&billing_note_id= จากหน้าใบวางบิล),
 *   ใบแจ้งหนี้แสดงชนิด (ขายสินค้า / ใบแจ้งหนี้ / มัดจำ) และภาษียังไม่ถึงกำหนด (รับเงินครั้งนี้ = ใบเสร็จรับเงิน / ใบกำกับภาษี)
 *   — การรับเงินทั้งหมดยืนยันที่การเงิน (แอปขายออกเอกสารอย่างเดียว)
 */

export type MoneyDocMode = 'receipt' | 'payment'

export type MoneyDocApi = {
  submit: string
  open: string // .../open-invoices
  parties: string // customers / suppliers
  accounts: string
  cashAccounts: string
  whtTypes?: string // จ่ายเงินเท่านั้น
  /** รับชำระหนี้ลูกค้า: ใบวางบิลของลูกค้า (GET /billing-notes?customer_id=&status=open) */
  billingNotes?: string
  /** จ่ายเงิน: คำขอเงินสดย่อยที่รอจ่าย (GET /payments/petty-cash-requests) — มี = แสดงชนิด "ชดเชยเงินสดย่อย" */
  pettyCash?: string
}

/** คำขอเงินสดย่อยที่รอการเงินจ่าย (ตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย) */
export type PettyCashRequest = { id: string; document_no: string; fund_code: string; fund_name: string; kind: string; date: string; amount: string }

type Selected = { on: boolean; amount: string; wht: string; whtType: string }
type OtherLine = { key: number; account: Ref | null; description: string; amount: string; wht: string; whtType: string }

let nextKey = 1
const otherLine = (): OtherLine => ({ key: nextKey++, account: null, description: '', amount: '', wht: '', whtType: '' })

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

type BillingNoteOption = { id: string; document_no: string; due_date: string; outstanding: string }

export function MoneyDocForm({
  mode,
  api,
  docsPath,
  listHref,
  initialParty = null,
  initialBillingNote = '',
}: {
  mode: MoneyDocMode
  api: MoneyDocApi
  docsPath: string
  listHref: string
  initialParty?: Ref | null
  /** ?billing_note_id= — เลือกใบแจ้งหนี้ตามใบวางบิลให้ตั้งแต่เปิดหน้า */
  initialBillingNote?: string
}) {
  const t = texts.finance
  const tr = texts.finance.receipts
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const partyKind = mode === 'receipt' ? 'customer' : 'supplier'
  const kinds = Object.fromEntries(
    Object.entries(mode === 'receipt' ? t.kinds.receipt : t.kinds.payment).filter(([k]) => k !== 'petty_cash' || !!api.pettyCash),
  )
  const partyLabel = mode === 'receipt' ? t.fields.customer : t.fields.supplier

  const [kind, setKind] = useState<string>(partyKind)
  const [head, setHead] = useState({ date: today(), description: '' })
  const [party, setParty] = useState<Ref | null>(initialParty)
  const [payee, setPayee] = useState({ name: '', tax_id: '', branch_no: '', address: '', tax_type: '' })
  const [notes, setNotes] = useState<BillingNoteOption[]>([])
  const [note, setNote] = useState(initialBillingNote)
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
  const isPetty = kind === 'petty_cash'
  const [requests, setRequests] = useState<PettyCashRequest[] | null>(null)
  const [requestId, setRequestId] = useState('')
  const request = requests?.find((r) => r.id === requestId) ?? null

  useEffect(() => {
    if (!isPetty || !api.pettyCash || requests) return
    apiRequest<PettyCashRequest[]>(api.pettyCash).then(setRequests, () => setRequests([]))
  }, [isPetty, api.pettyCash, requests])

  useEffect(() => {
    if (!api.whtTypes) return
    apiRequest<WhtType[]>(`${api.whtTypes}?page_size=100`).then(setWhtTypes, () => setWhtTypes([]))
  }, [api.whtTypes])

  // ใบวางบิลที่ยังค้างของลูกค้า (เฉพาะแอปขาย)
  useEffect(() => {
    if (!api.billingNotes || !party) {
      setNotes([])
      return
    }
    let live = true
    apiRequest<BillingNoteOption[]>(`${api.billingNotes}?customer_id=${encodeURIComponent(party.id)}&status=open&page_size=50`).then(
      (rows) => live && setNotes(rows),
      () => live && setNotes([]),
    )
    return () => {
      live = false
    }
  }, [api.billingNotes, party])

  // ใบแจ้งหนี้ค้าง — มีใบวางบิล = เฉพาะใบในใบวางบิลและเลือกให้พร้อมยอดค้าง
  useEffect(() => {
    if (!party) {
      setInvoices(null)
      return
    }
    let live = true
    setLoadingInvoices(true)
    const q = new URLSearchParams({ [`${partyKind}_id`]: party.id })
    if (note) q.set('billing_note_id', note)
    apiRequest<OpenInvoice[]>(`${api.open}?${q.toString()}`)
      .then((rows) => {
        if (!live) return
        setInvoices(rows)
        setSelected(note ? Object.fromEntries(rows.map((r) => [r.id, { on: true, amount: r.outstanding, wht: '', whtType: '' }])) : {})
      })
      .catch((err: unknown) => live && (setInvoices([]), setFormError(errorMessage(err))))
      .finally(() => live && setLoadingInvoices(false))
    return () => {
      live = false
    }
  }, [party, note, api.open, partyKind])

  const rateOf = (id: string) => toUnits(whtTypes.find((w) => w.id === id)?.rate ?? '0')

  // ---------- ยอดรวม ----------
  const lineTotals = isPetty
    ? request
      ? [{ amount: toUnits(request.amount), wht: 0n }]
      : []
    : isParty
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
  const paid = methodsTotal(methods)
  const balanced = net > 0n && net === paid

  const setSel = (id: string, patch: Partial<Selected>) =>
    setSelected((s) => ({ ...s, [id]: { on: false, amount: '', wht: '', whtType: '', ...s[id], ...patch } }))
  const toggle = (inv: OpenInvoice, on: boolean) => setSel(inv.id, on ? { on, amount: selected[inv.id]?.amount || inv.outstanding } : { on })
  const allOn = !!invoices?.length && invoices.every((inv) => selected[inv.id]?.on)

  function validate() {
    const e: Record<string, string> = {}
    if (!head.date) e.date = requiredMessage(t.fields.date)
    if (isPetty) {
      if (!request) e.request = t.errors.pettyCashRequired
      Object.assign(e, validateMethods(methods))
      if (request && !balanced) e.methods = t.errors.methodsMismatch
      setErrors(e)
      return Object.keys(e).length === 0
    }
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
    Object.assign(e, validateMethods(methods))
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
      methods: methodsBody(methods),
    }
    if (isPetty) {
      body.petty_cash_request_id = request!.id
    } else if (isParty) {
      body[`${partyKind}_id`] = party!.id
      if (note) body.billing_note_id = note
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

  const amountField = 'w-36 [&_input]:text-right [&_input]:tabular-nums'

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}

      <FormSection title={t.fields.kind}>
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
                <span className="block text-sm font-medium">{label}</span>
              </label>
            ))}
          </div>
        </fieldset>
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      {isPetty ? (
        <FormSection title={t.sections.pettyCash}>
          <SelectField
            label={t.fields.pettyCashRequest}
            required
            value={requestId}
            error={errors.request}
            className="sm:col-span-2"
            options={[
              { value: '', label: requests === null ? t.pettyCashRequests.loading : requests.length ? t.pettyCashRequests.pick : t.pettyCashRequests.none },
              ...(requests ?? []).map((r) => ({
                value: r.id,
                label: `${r.document_no} · ${r.fund_code} ${r.fund_name} · ${t.pettyCashKinds[r.kind] ?? r.kind} · ${formatAmount(r.amount)}`,
              })),
            ]}
            onChange={(e) => setRequestId(e.target.value)}
          />
          {request && (
            <dl className="grid gap-x-6 gap-y-2 text-sm sm:col-span-2 sm:grid-cols-3">
              {(
                [
                  [t.fields.date, formatDate(request.date)],
                  [t.fields.pettyCashFund, `${request.fund_code} ${request.fund_name}`],
                  [t.totals.amount, formatAmount(request.amount)],
                ] as const
              ).map(([k, v]) => (
                <div key={k}>
                  <dt className="text-[13px] text-ink-muted">{k}</dt>
                  <dd className="font-medium tabular-nums">{v}</dd>
                </div>
              ))}
            </dl>
          )}
        </FormSection>
      ) : (
      <FormSection title={t.sections.party}>
        {isParty ? (
          <>
            <ReferenceSelect
              label={partyLabel}
              required
              source={api.parties}
              params={{ status: 'active' }}
              value={party}
              onChange={(p) => {
                setParty(p)
                setNote('')
              }}
              error={errors.party}
              texts={texts.reference}
              className="sm:col-span-2"
            />
            {api.billingNotes && party && (
              <div className="flex flex-col gap-1 sm:col-span-2">
                <SelectField
                  label={tr.billingNote}                  value={note}
                  options={[
                    { value: '', label: texts.common.none },
                    ...notes.map((n) => ({ value: n.id, label: `${n.document_no} · ${formatDate(n.due_date)} · ${formatAmount(n.outstanding)}` })),
                    ...(note && !notes.some((n) => n.id === note) ? [{ value: note, label: note }] : []),
                  ]}
                  onChange={(e) => setNote(e.target.value)}
                />              </div>
            )}
          </>
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
      )}

      {!isPetty && (
      <section aria-labelledby="doc-lines" className="flex min-w-0 flex-col gap-3">
        <h2 id="doc-lines" className="text-[15px] font-semibold">
          {isParty ? t.sections.invoices : t.sections.lines}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
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
                    const undue = toUnits(inv.vat_undue ?? '0') > 0n && !inv.tax_invoice
                    return (
                      <tr key={inv.id} className={cx('border-b border-line align-top last:border-b-0', s?.on && 'bg-primary-soft/30')}>
                        <td className="px-3 py-2">
                          <input
                            type="checkbox"
                            aria-label={format(t.invoices.select, { no: inv.document_no })}
                            checked={!!s?.on}
                            onChange={(e) => toggle(inv, e.target.checked)}
                            className="mt-2.5 size-4 cursor-pointer accent-primary"
                          />
                        </td>
                        <td className="px-2 py-2">
                          <span className="font-mono text-[13px]">{inv.document_no}</span>
                          {inv.partner_ref && <span className="block text-[12px] text-ink-muted">{inv.partner_ref}</span>}
                          {(inv.kind || undue || inv.tax_invoice) && (
                            <span className="mt-1 flex flex-wrap gap-1">
                              {inv.kind && <Badge tone={inv.kind === 'service_invoice' ? 'info' : 'neutral'}>{tr.kindBadge[inv.kind] ?? inv.kind}</Badge>}
                              {undue && (
                                <Badge tone="warning">
                                  {format(tr.vatUndue, { vat: formatAmount(inv.vat_undue) })}
                                  <span className="sr-only"> — {tr.vatUndueHint}</span>
                                </Badge>
                              )}
                              {inv.tax_invoice && <Badge tone="success">{tr.taxInvoiceIssued}</Badge>}
                            </span>
                          )}
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
      )}

      <section aria-labelledby="doc-methods" className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-3">
          <h2 id="doc-methods" className="text-[15px] font-semibold">
            {t.sections.methods}
          </h2>
          <MethodsEditor methods={methods} onChange={setMethods} cashAccountsApi={api.cashAccounts} target={net} errors={errors} />
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
              <dd className="tabular-nums">{formatAmount(paid)}</dd>
            </div>
          </dl>
          <MethodsBalance target={net} total={paid} error={errors.methods} />
        </aside>
      </section>

      <FormBar>
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
