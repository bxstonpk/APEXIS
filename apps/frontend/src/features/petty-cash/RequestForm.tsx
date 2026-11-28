'use client'

import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid } from '@/lib/validation'
import { selectedTotal, type Ref, type RequestKind, type VoucherRow } from './types'

/**
 * คำขอเงินสดย่อย — ตั้งวงเงิน (ยอด = วงเงินของกองทุน) / เพิ่มวงเงิน (กรอกยอด) / เบิกชดเชย (เลือกใบสำคัญจ่ายที่อนุมัติแล้ว)
 * ส่งแล้วรอการเงินจ่าย (การเงินเลือกคำขอในหน้าจ่ายเงิน)
 */
export function RequestForm({
  apiPath,
  docsPath,
  listHref,
  fundsApi,
  vouchersApi,
  kind,
  initialFund = null,
}: {
  apiPath: string
  docsPath: string
  listHref: string
  fundsApi: string
  vouchersApi: string
  kind: RequestKind
  initialFund?: (Ref & { imprest_amount?: string }) | null
}) {
  const t = texts.pettyCash.requests
  const f = t.fields
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [fund, setFund] = useState<Ref | null>(initialFund)
  const [date, setDate] = useState(today())
  const [description, setDescription] = useState('')
  const [amount, setAmount] = useState('')
  const [rows, setRows] = useState<VoucherRow[] | null>(null)
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // ใบสำคัญจ่ายที่อนุมัติแล้วและยังไม่ได้ขอเบิกของกองทุนที่เลือก
  useEffect(() => {
    if (kind !== 'replenish' || !fund) {
      setRows(null)
      return
    }
    let live = true
    setRows(null)
    apiRequest<VoucherRow[]>(`${vouchersApi}?fund_id=${encodeURIComponent(fund.id)}&unrequested=true&sort=date&page_size=200`).then(
      (r) => {
        if (!live) return
        setRows(r)
        setPicked(new Set(r.map((x) => x.id)))
      },
      () => live && setRows([]),
    )
    return () => {
      live = false
    }
  }, [kind, fund, vouchersApi])

  const total = kind === 'replenish' ? selectedTotal(rows ?? [], picked) : kind === 'increase' ? normalizeAmount(amount) ?? '0' : (initialFund?.imprest_amount ?? '0')
  const allOn = !!rows?.length && rows.every((r) => picked.has(r.id))
  const toggle = (id: string, on: boolean) =>
    setPicked((s) => {
      const n = new Set(s)
      if (on) n.add(id)
      else n.delete(id)
      return n
    })

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    const errs: Record<string, string> = {}
    if (!fund) errs.fund = t.errors.fundRequired
    const n = normalizeAmount(amount)
    if (kind === 'increase' && (!n || toUnits(n) <= 0n)) errs.amount = t.errors.amountInvalid
    if (kind === 'replenish' && picked.size === 0) errs.vouchers = t.errors.noVouchers
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    setBusy(true)
    try {
      const body: Record<string, unknown> = { fund_id: fund!.id, kind, date, description: description.trim() }
      if (kind === 'increase') body.amount = n
      if (kind === 'replenish') body.voucher_ids = (rows ?? []).filter((r) => picked.has(r.id)).map((r) => r.id)
      const saved = await apiRequest<{ id: string }>(apiPath, { method: 'POST', body })
      showToast(texts.common.toast.created)
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      <FormSection title={texts.pettyCash.requestKinds[kind] ?? kind}>
        <ReferenceSelect
          label={f.fund}
          required
          source={fundsApi}
          params={{ status: 'active' }}
          value={fund}
          onChange={(x) => {
            setFund(x)
            setPicked(new Set())
          }}
          error={errors.fund}
          disabled={kind === 'establish' && !!initialFund}
          texts={texts.reference}
        />
        <TextField label={f.date} required type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        {kind === 'increase' && (
          <TextField
            label={f.amount}
            required
            inputMode="decimal"
            value={amount}
            error={errors.amount}
            className="[&_input]:text-right [&_input]:tabular-nums"
            onChange={(e) => setAmount(e.target.value)}
          />
        )}
        <TextField label={f.description} value={description} maxLength={500} className="sm:col-span-2" onChange={(e) => setDescription(e.target.value)} />
      </FormSection>

      {kind === 'replenish' && (
        <section aria-labelledby="pcr-vouchers" className="flex min-w-0 flex-col gap-3">
          <h2 id="pcr-vouchers" className="text-[15px] font-semibold">
            {t.pick.title}
          </h2>
          <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
            {!fund || rows === null || rows.length === 0 ? (
              <p className="px-4 py-10 text-center text-sm text-ink-muted" aria-live="polite">
                {fund && rows !== null ? t.pick.none : ''}
              </p>
            ) : (
              <table className="w-full min-w-[40rem] border-collapse text-sm">
                <thead>
                  <tr className="border-b border-line bg-subtle">
                    <th scope="col" className={`${th} w-12`}>
                      <input
                        type="checkbox"
                        aria-label={t.pick.selectAll}
                        checked={allOn}
                        onChange={(e) => setPicked(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())}
                        className="size-4 accent-primary"
                      />
                    </th>
                    <th scope="col" className={th}>
                      {texts.pettyCash.vouchers.columns.date}
                    </th>
                    <th scope="col" className={th}>
                      {texts.pettyCash.vouchers.columns.no}
                    </th>
                    <th scope="col" className={th}>
                      {texts.pettyCash.vouchers.columns.payee}
                    </th>
                    <th scope="col" className={`${th} text-right`}>
                      {texts.pettyCash.vouchers.columns.gross}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.id} className="border-b border-line last:border-b-0">
                      <td className="px-3 py-2.5">
                        <input
                          type="checkbox"
                          aria-label={format(t.pick.select, { no: r.document_no })}
                          checked={picked.has(r.id)}
                          onChange={(e) => toggle(r.id, e.target.checked)}
                          className="size-4 accent-primary"
                        />
                      </td>
                      <td className="px-3 py-2.5 whitespace-nowrap">{formatDate(r.date)}</td>
                      <td className="px-3 py-2.5 font-mono text-[13px] whitespace-nowrap">{r.document_no}</td>
                      <td className="px-3 py-2.5">{r.payee_name}</td>
                      <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(r.totals.gross)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          {errors.vouchers && (
            <p role="alert" className="text-[13px] text-danger">
              {errors.vouchers}
            </p>
          )}
        </section>
      )}

      <FormBar>
        <p aria-live="polite" className="text-sm">
          <span className="text-ink-muted">{kind === 'replenish' ? t.pick.total : f.amount}</span>
          <span className="ml-2 font-semibold tabular-nums">{formatAmount(total)}</span>
        </p>
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
