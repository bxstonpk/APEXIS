'use client'

import { CircleAlert, CircleCheck, Info, Plus, Search } from 'lucide-react'
import Link from 'next/link'
import { useEffect, useRef, useState } from 'react'
import { Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { ApiError, apiRequest, errorMessage } from '@/lib/http'
import type { VatRegistrant } from './thaiAddress'

/**
 * ช่องเลขประจำตัวผู้เสียภาษี + ดึงข้อมูลผู้จด VAT จากกรมสรรพากร
 *   1. เลขนี้มีรหัสอยู่แล้ว (เลขผู้เสียภาษี 1 เลข = 1 รหัส) → แจ้งพร้อมลิงก์ไปรหัสนั้นเพื่อเพิ่มสาขา ไม่ดึงข้อมูล
 *   2. พบในกรมสรรพากร → สำนักงานใหญ่ (หรือสาขาแรก) เป็นสาขาหลัก; สาขาอื่นที่จดทะเบียนกด "เพิ่มเป็นสาขา" ได้ทีละสาขา
 *   3. ไม่พบ / ระบบสรรพากรล่ม → แจ้งให้กรอกเอง
 * ฟอร์มใหม่ดึงเองเมื่อครบ 13 หลัก; ฟอร์มแก้ไขต้องกดปุ่ม — ผลประกาศใน role="status" (ไม่ย้าย focus)
 */

type Owner = { code: string; name: string; href: string }

type State =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'applied'; row: VatRegistrant; others: VatRegistrant[] }
  | { kind: 'owned'; owner: Owner }
  | { kind: 'notFound' }
  | { kind: 'unavailable' }
  | { kind: 'error'; message: string }

export const isTaxId = (s: string) => /^\d{13}$/.test(s)

/** สำนักงานใหญ่ / สาขาที่ 00001 ชื่อสาขา */
export function branchText(r: VatRegistrant): string {
  return r.branch_no === '00000' ? texts.sales.print.headOffice : `${format(texts.sales.print.branch, { no: r.branch_no })} ${r.branch_name}`
}

export type OwnerCheck = {
  /** API รายการลูกค้า / ผู้ขาย (ค้นด้วย ?q=เลขผู้เสียภาษี) */
  source: string
  /** path หน้ารายการ — ลิงก์ไปรหัสเดิม = `${path}/${id}` */
  path: string
  /** รหัสที่กำลังแก้ (ไม่นับว่าซ้ำกับตัวเอง) */
  selfId?: string
}

export function VatLookup({
  source,
  taxId,
  onTaxIdChange,
  onApply,
  onAddBranch,
  addedBranches = [],
  owner,
  error,
  disabled,
  autoFetch = false,
}: {
  /** GET path ของ vat-registrants */
  source: string
  taxId: string
  onTaxIdChange: (v: string) => void
  /** สาขาหลักที่ได้จากกรมสรรพากร */
  onApply: (r: VatRegistrant) => void
  /** เพิ่มสาขาอื่นที่จดทะเบียนเป็นสาขาเพิ่มเติม */
  onAddBranch?: (r: VatRegistrant) => void
  /** เลขสาขาที่มีในฟอร์มแล้ว (สาขาหลัก + สาขาเพิ่มเติม) */
  addedBranches?: string[]
  owner?: OwnerCheck
  error?: string
  disabled?: boolean
  /** ฟอร์มใหม่: ดึงให้เองเมื่อกรอกครบ 13 หลัก (ฟอร์มแก้ไขต้องกดปุ่ม เพื่อไม่ทับข้อมูลเดิมโดยไม่ตั้งใจ) */
  autoFetch?: boolean
}) {
  const t = texts.partners.lookup
  const [state, setState] = useState<State>({ kind: 'idle' })
  const fetched = useRef('')
  const applyRef = useRef(onApply)
  applyRef.current = onApply

  async function existingOwner(id: string): Promise<Owner | null> {
    if (!owner) return null
    const rows = await apiRequest<{ id: string; code: string; name: string; tax_id: string }[]>(`${owner.source}?q=${id}&page_size=5`)
    const hit = rows.find((r) => r.tax_id === id && r.id !== owner.selfId)
    return hit ? { code: hit.code, name: hit.name, href: `${owner.path}/${hit.id}` } : null
  }

  async function lookup(id: string) {
    fetched.current = id
    setState({ kind: 'loading' })
    try {
      const own = await existingOwner(id)
      if (fetched.current !== id) return // ผู้ใช้แก้เลขระหว่างรอ
      if (own) return setState({ kind: 'owned', owner: own })
      const rows = await apiRequest<VatRegistrant[]>(`${source}?tax_id=${id}`)
      if (fetched.current !== id) return
      const main = rows.find((r) => r.branch_no === '00000') ?? rows[0]!
      applyRef.current(main)
      setState({ kind: 'applied', row: main, others: rows.filter((r) => r !== main) })
    } catch (err) {
      if (fetched.current !== id) return
      if (err instanceof ApiError && err.status === 404) setState({ kind: 'notFound' })
      else if (err instanceof ApiError && err.status === 503) setState({ kind: 'unavailable' })
      else setState({ kind: 'error', message: errorMessage(err) })
    }
  }

  useEffect(() => {
    if (!autoFetch || disabled || !isTaxId(taxId) || fetched.current === taxId) return
    const timer = setTimeout(() => void lookup(taxId), 250)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- lookup อ่านค่าล่าสุดผ่าน ref
  }, [taxId, autoFetch, disabled])

  function change(v: string) {
    const digits = v.replace(/\D/g, '').slice(0, 13)
    onTaxIdChange(digits)
    if (digits !== fetched.current && state.kind !== 'loading') setState({ kind: 'idle' })
  }

  const message =
    state.kind === 'applied'
      ? { tone: 'ok', text: format(t.applied, { name: state.row.name, branch: branchText(state.row), since: state.row.vat_registered_on || '–' }) }
      : state.kind === 'owned'
        ? { tone: 'warn', text: format(t.owned, { code: state.owner.code, name: state.owner.name }) }
        : state.kind === 'notFound'
          ? { tone: 'info', text: t.notFound }
          : state.kind === 'unavailable'
            ? { tone: 'warn', text: t.unavailable }
            : state.kind === 'error'
              ? { tone: 'warn', text: state.message }
              : state.kind === 'loading'
                ? { tone: 'info', text: t.loading }
                : null
  const Icon = message?.tone === 'ok' ? CircleCheck : message?.tone === 'warn' ? CircleAlert : Info
  const others = state.kind === 'applied' ? state.others : []

  return (
    <div className="flex flex-col gap-2 sm:col-span-2">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
        <TextField
          label={t.taxId}
          hint={t.hint}
          value={taxId}
          inputMode="numeric"
          autoComplete="off"
          maxLength={13}
          disabled={disabled}
          error={error}
          className="sm:flex-1"
          onChange={(e) => change(e.target.value)}
        />
        <Button
          type="button"
          variant="secondary"
          className="sm:mt-[26px]"
          disabled={disabled || !isTaxId(taxId)}
          loading={state.kind === 'loading'}
          loadingText={t.loadingShort}
          onClick={() => void lookup(taxId)}
        >
          <Search aria-hidden />
          {t.button}
        </Button>
      </div>

      <div role="status" aria-live="polite">
        {message && (
          <div
            className={`flex items-start gap-2 rounded-(--radius-control) px-3 py-2 text-[13px] leading-snug ${
              message.tone === 'ok' ? 'bg-success-soft text-success' : message.tone === 'warn' ? 'bg-warning-soft text-warning' : 'bg-info-soft text-ink'
            }`}
          >
            <Icon aria-hidden className="mt-px size-4 shrink-0" />
            <span>
              {message.text}
              {state.kind === 'owned' && (
                <>
                  {' '}
                  <Link href={state.owner.href} className="font-medium text-primary underline underline-offset-2">
                    {format(t.openOwner, { code: state.owner.code })}
                  </Link>
                </>
              )}
            </span>
          </div>
        )}
      </div>

      {others.length > 0 && onAddBranch && (
        <section aria-label={t.otherBranches} className="flex flex-col gap-2">
          <p className="text-[13px] font-medium">{format(t.otherBranchesHint, { n: String(others.length) })}</p>
          {others.map((r) => {
            const added = addedBranches.includes(r.branch_no)
            return (
              <div key={r.branch_no} className="flex flex-col gap-2 rounded-(--radius-control) border border-line bg-surface px-3 py-2.5 text-sm sm:flex-row sm:items-center">
                <span className="min-w-0 flex-1">
                  <span className="font-medium">
                    <span className="font-mono text-[13px] text-ink-muted">{r.branch_no}</span> {r.branch_name}
                  </span>
                  <span className="block text-[13px] text-ink-muted">{r.address}</span>
                </span>
                <Button type="button" size="sm" variant="secondary" disabled={disabled || added} onClick={() => onAddBranch(r)}>
                  {!added && <Plus aria-hidden />}
                  {added ? t.added : t.addBranch}
                </Button>
              </div>
            )
          })}
        </section>
      )}
    </div>
  )
}
