'use client'

import { PackageCheck, PackageMinus, PackagePlus, Play, Printer, X } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Alert, Button, ButtonLink, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid } from '@/lib/validation'
import { remaining, type OrderMaterial, type ProductionOrder } from './types'

/**
 * ปุ่มตามสถานะใบสั่งผลิต + แผงกรอกข้อมูลในหน้า (ไม่ใช้ modal: อยู่ใต้แถบปุ่ม, โฟกัสหัวแผงเมื่อเปิด, Esc / ปิด = กลับ)
 *   ร่าง: ปล่อยผลิต (หลัก), ยกเลิก
 *   กำลังผลิต: เบิกวัตถุดิบ, คืนวัตถุดิบ, ผลิตเสร็จ (หลัก), ยกเลิก
 */
type Panel = 'issue' | 'return' | 'complete' | 'cancel' | null

/** จำนวนตั้งต้นของแผงเบิก = ยังต้องเบิก (ต้องใช้ − ใช้จริง, ไม่ติดลบ); แผงคืน = ว่าง */
export function issueDefaults(materials: OrderMaterial[]): Record<string, string> {
  return Object.fromEntries(materials.map((m) => [m.item.id, remaining(m.required_quantity, m.net_quantity)]))
}

/** เพดานต่อวัตถุดิบ: เบิก = ยังต้องเบิก, คืน = ใช้จริง */
export function limitOf(kind: 'issue' | 'return', m: OrderMaterial): string {
  return kind === 'issue' ? remaining(m.required_quantity, m.net_quantity) : remaining(m.net_quantity, '0')
}

/** ตรวจจำนวนในแผงเบิก / คืน — คืน errors (key = item id หรือ '_') และบรรทัดที่จะส่ง (ตัดบรรทัด 0) */
export function checkQuantities(kind: 'issue' | 'return', materials: OrderMaterial[], values: Record<string, string>) {
  const p = texts.manufacturing.orders.panels
  const errors: Record<string, string> = {}
  const lines: { item_id: string; quantity: string }[] = []
  for (const m of materials) {
    const raw = values[m.item.id] ?? ''
    const q = normalizeAmount(raw)
    if (q === '' ) continue
    if (q === null || toUnits(q) < 0n) {
      errors[m.item.id] = p.qtyInvalid
      continue
    }
    if (toUnits(q) === 0n) continue
    const max = limitOf(kind, m)
    if (toUnits(q) > toUnits(max)) {
      errors[m.item.id] = format(kind === 'issue' ? p.overRemaining : p.overReturnable, { max: formatAmount(max) })
      continue
    }
    lines.push({ item_id: m.item.id, quantity: q })
  }
  if (!Object.keys(errors).length && lines.length === 0) errors._ = p.nothing
  return { errors, lines }
}

export function OrderActions({ apiPath, order, canManage, printHref }: { apiPath: string; order: ProductionOrder; canManage: boolean; printHref: string }) {
  const t = texts.manufacturing.orders
  const a = t.actions
  const router = useRouter()
  const [panel, setPanel] = useState<Panel>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const materials = order.materials ?? []
  const no = order.document_no ?? t.draftTitle

  async function call(label: string, path: string, body?: unknown) {
    setBusy(label)
    setError(null)
    try {
      await apiRequest(`${apiPath}/${path}`, { method: 'POST', body: body ?? {} })
      showToast(texts.common.toast.saved)
      setPanel(null)
      router.refresh()
      return true
    } catch (err) {
      setError(errorMessage(err))
      return false
    } finally {
      setBusy(null)
    }
  }

  const draft = order.status === 'draft'
  const released = order.status === 'released'
  const anyNet = materials.some((m) => toUnits(m.net_quantity) > 0n)
  const toggle = (p: Exclude<Panel, null>) => setPanel((cur) => (cur === p ? null : p))

  return (
    <div className="flex flex-col gap-3 print:hidden">
      {error && <Alert tone="error">{error}</Alert>}
      <div className="flex flex-wrap items-center gap-2 rounded-(--radius-panel) border border-line bg-surface px-4 py-3 shadow-card">
        {canManage && (draft || released) && (
          <Button variant="danger" disabled={busy !== null} aria-expanded={panel === 'cancel'} onClick={() => toggle('cancel')}>
            {a.cancel}
          </Button>
        )}
        <div className="ml-auto flex flex-wrap gap-2">
          <ButtonLink href={printHref} variant="secondary">
            <Printer aria-hidden />
            {a.print}
          </ButtonLink>
          {canManage && draft && (
            <Button
              loading={busy === a.release}
              loadingText={a.working}
              disabled={busy !== null}
              onClick={() => {
                if (window.confirm(format(t.confirm.release!, { no }))) void call(a.release, 'release')
              }}
            >
              <Play aria-hidden />
              {a.release}
            </Button>
          )}
          {canManage && released && (
            <>
              <Button variant="secondary" disabled={busy !== null} aria-expanded={panel === 'issue'} onClick={() => toggle('issue')}>
                <PackageMinus aria-hidden />
                {a.issue}
              </Button>
              {anyNet && (
                <Button variant="secondary" disabled={busy !== null} aria-expanded={panel === 'return'} onClick={() => toggle('return')}>
                  <PackagePlus aria-hidden />
                  {a.return}
                </Button>
              )}
              <Button disabled={busy !== null} aria-expanded={panel === 'complete'} onClick={() => toggle('complete')}>
                <PackageCheck aria-hidden />
                {a.complete}
              </Button>
            </>
          )}
        </div>
      </div>

      {(panel === 'issue' || panel === 'return') && (
        <QuantityPanel key={panel} kind={panel} materials={materials} busy={busy !== null} onClose={() => setPanel(null)} onSubmit={(body) => call(panel === 'issue' ? a.issue : a.return, panel, body)} />
      )}
      {panel === 'complete' && <CompletePanel order={order} busy={busy !== null} onClose={() => setPanel(null)} onSubmit={(body) => call(a.complete, 'complete', body)} />}
      {panel === 'cancel' && <CancelPanel busy={busy !== null} onClose={() => setPanel(null)} onSubmit={(body) => call(a.cancel, 'cancel', body)} />}
    </div>
  )
}

function PanelFrame({ title, onClose, onSubmit, busy, submitLabel, danger, children }: { title: string; onClose: () => void; onSubmit: (e: FormEvent<HTMLFormElement>) => void; busy: boolean; submitLabel: string; danger?: boolean; children: ReactNode }) {
  const id = useId()
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => headingRef.current?.focus(), [])
  return (
    <form
      noValidate
      aria-labelledby={id}
      onSubmit={onSubmit}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onClose()
      }}
      className="flex flex-col gap-4 rounded-(--radius-panel) border border-primary/30 bg-surface p-5 shadow-card"
    >
      <div className="flex items-start justify-between gap-3">
        <h2 id={id} ref={headingRef} tabIndex={-1} className="text-[15px] font-semibold outline-none">
          {title}
        </h2>
        <button
          type="button"
          aria-label={texts.manufacturing.orders.actions.close}
          onClick={onClose}
          className="flex size-10 shrink-0 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-subtle hover:text-ink"
        >
          <X aria-hidden className="size-4" />
        </button>
      </div>
      {children}
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="secondary" onClick={onClose} disabled={busy}>
          {texts.common.cancel}
        </Button>
        <Button type="submit" variant={danger ? 'danger' : 'primary'} loading={busy} loadingText={texts.manufacturing.orders.actions.working}>
          {submitLabel}
        </Button>
      </div>
    </form>
  )
}

function QuantityPanel({ kind, materials, busy, onClose, onSubmit }: { kind: 'issue' | 'return'; materials: OrderMaterial[]; busy: boolean; onClose: () => void; onSubmit: (body: unknown) => Promise<boolean> }) {
  const t = texts.manufacturing.orders
  const p = t.panels
  const [date, setDate] = useState(today())
  const [values, setValues] = useState<Record<string, string>>(() => (kind === 'issue' ? issueDefaults(materials) : {}))
  const [errors, setErrors] = useState<Record<string, string>>({})
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const { errors: errs, lines } = checkQuantities(kind, materials, values)
    if (!date) errs.date = p.qtyInvalid
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    void onSubmit({ date, lines })
  }
  const th = 'px-3 py-2.5 text-[13px] font-medium text-ink-muted'
  return (
    <PanelFrame
      title={kind === 'issue' ? p.issueTitle : p.returnTitle}      onClose={onClose}
      onSubmit={submit}
      busy={busy}
      submitLabel={kind === 'issue' ? t.actions.issue : t.actions.return}
    >
      <TextField label={p.date} type="date" required value={date} error={errors.date} className="w-full sm:w-56" onChange={(e) => setDate(e.target.value)} />
      <div className="relative overflow-x-auto rounded-(--radius-control) border border-line">
        <table className="w-full min-w-[34rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle text-left">
              <th scope="col" className={th}>
                {t.materials.item}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {kind === 'issue' ? p.remaining : p.returnable}
              </th>
              <th scope="col" className={`${th} w-44 text-right`}>
                {p.qty}
              </th>
            </tr>
          </thead>
          <tbody>
            {materials.map((m) => (
              <tr key={m.item.id} className="border-b border-line align-top last:border-b-0">
                <td className="px-3 pt-4">
                  <span className="text-[13px] text-ink-muted">{m.item.code}</span> {m.item.name}
                </td>
                <td className="px-3 pt-4 text-right tabular-nums">
                  {formatAmount(limitOf(kind, m))} {m.item.uom && <span className="text-[12px] text-ink-muted">{m.item.uom}</span>}
                </td>
                <td className="px-3 py-2">
                  <TextField
                    label={format(p.qtyFor, { item: m.item.name })}
                    hideLabel
                    inputMode="decimal"
                    value={values[m.item.id] ?? ''}
                    error={errors[m.item.id]}
                    className="[&_input]:text-right [&_input]:tabular-nums"
                    onChange={(e) => setValues((v) => ({ ...v, [m.item.id]: e.target.value }))}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {errors._ && (
        <p role="alert" className="text-[13px] text-danger">
          {errors._}
        </p>
      )}
    </PanelFrame>
  )
}

function CompletePanel({ order, busy, onClose, onSubmit }: { order: ProductionOrder; busy: boolean; onClose: () => void; onSubmit: (body: unknown) => Promise<boolean> }) {
  const t = texts.manufacturing.orders
  const p = t.panels
  const [date, setDate] = useState(today())
  const [qty, setQty] = useState(order.quantity)
  const [errors, setErrors] = useState<Record<string, string>>({})
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const q = normalizeAmount(qty)
    const errs: Record<string, string> = {}
    if (!q || toUnits(q) <= 0n) errs.qty = p.qtyInvalid
    if (!date) errs.date = p.qtyInvalid
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    if (!window.confirm(format(t.confirm.complete!, { qty: formatAmount(q!) }))) return
    void onSubmit({ date, completed_quantity: q })
  }
  return (
    <PanelFrame title={p.completeTitle} onClose={onClose} onSubmit={submit} busy={busy} submitLabel={t.actions.complete}>
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField label={p.date} type="date" required value={date} error={errors.date} onChange={(e) => setDate(e.target.value)} />
        <TextField
          label={p.completedQty}
          required
          inputMode="decimal"
          value={qty}
          error={errors.qty}
          endAdornment={order.item.uom ? <span className="pr-2 text-[13px] text-ink-muted">{order.item.uom}</span> : undefined}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => setQty(e.target.value)}
        />
      </div>
    </PanelFrame>
  )
}

function CancelPanel({ busy, onClose, onSubmit }: { busy: boolean; onClose: () => void; onSubmit: (body: unknown) => Promise<boolean> }) {
  const t = texts.manufacturing.orders
  const p = t.panels
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string>()
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!reason.trim()) {
      setError(p.reasonRequired)
      return focusFirstInvalid(e.currentTarget)
    }
    setError(undefined)
    void onSubmit({ reason: reason.trim() })
  }
  return (
    <PanelFrame title={p.cancelTitle} onClose={onClose} onSubmit={submit} busy={busy} submitLabel={t.actions.cancel} danger>
      <TextField label={p.reason} required value={reason} maxLength={500} error={error} onChange={(e) => setReason(e.target.value)} />
    </PanelFrame>
  )
}
