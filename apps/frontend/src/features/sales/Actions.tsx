'use client'

import { Printer, Truck, ReceiptText, Wallet } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Button, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'

/**
 * แถบปุ่มตามสถานะเอกสาร — ปุ่มหลัก 1 ปุ่ม (ขั้นถัดไปของ workflow), ปุ่มอื่นเป็นรอง, ปุ่มยกเลิก / ลบเป็นสีแดงแยกด้านซ้าย
 * ทุกการกระทำที่เปลี่ยนสถานะยืนยันก่อน; ผลลัพธ์แจ้งด้วย toast แล้วโหลดหน้าใหม่ (หรือไปเอกสารที่สร้าง)
 */

type Action =
  | { kind: 'patch'; status: string; label: string; confirm?: string; primary?: boolean; danger?: boolean }
  | { kind: 'delete'; label: string; confirm: string }
  | { kind: 'toOrder'; label: string; primary?: boolean }
  | { kind: 'link'; href: string; label: string; icon?: 'print' | 'deliver' | 'invoice' | 'receive'; primary?: boolean }

const icons = { print: Printer, deliver: Truck, invoice: ReceiptText, receive: Wallet }

const CREDIT = /^credit limit exceeded: customer (\S+) exposure (-?[\d.]+), limit (-?[\d.]+) \(over by (-?[\d.]+)\)$/

/** คำเตือนจาก backend (ภาษาอังกฤษสำหรับ developer) → ข้อความไทย; รูปแบบที่ไม่รู้จักแสดงตามเดิม */
export function warningText(w: string): string {
  const m = CREDIT.exec(w)
  if (!m) return w
  return format(texts.sales.creditWarning, { code: m[1]!, exposure: formatAmount(m[2]), limit: formatAmount(m[3]), over: formatAmount(m[4]) })
}

export function DocActions({
  apiPath,
  listHref,
  docNo,
  actions,
  ordersApi,
  ordersPath,
  quotationId,
}: {
  apiPath: string
  listHref: string
  docNo?: string | null
  actions: Action[]
  /** สำหรับ "สร้างใบสั่งขาย" จากใบเสนอราคา */
  ordersApi?: string
  ordersPath?: string
  quotationId?: string
}) {
  const t = texts.sales
  const router = useRouter()
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [warnings, setWarnings] = useState<string[]>([])

  async function run(a: Action) {
    if ('confirm' in a && a.confirm && !window.confirm(format(a.confirm, { no: docNo ?? '' }))) return
    setBusy(a.label)
    setError(null)
    try {
      if (a.kind === 'patch') {
        const res = await apiRequest<{ warnings?: string[] }>(apiPath, { method: 'PATCH', body: { status: a.status } })
        setWarnings(res?.warnings ?? [])
        showToast(t.toast.done)
        router.refresh()
      } else if (a.kind === 'delete') {
        await apiRequest(apiPath, { method: 'DELETE' })
        showToast(texts.common.toast.deleted)
        router.push(listHref)
        router.refresh()
      } else if (a.kind === 'toOrder' && ordersApi && ordersPath && quotationId) {
        const today = new Date().toISOString().slice(0, 10)
        const order = await apiRequest<{ id: string }>(ordersApi, { method: 'POST', body: { quotation_id: quotationId, date: today } })
        showToast(t.toast.saved)
        router.push(`${ordersPath}/${order.id}`)
        router.refresh()
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  const left = actions.filter((a) => a.kind === 'delete' || (a.kind === 'patch' && a.danger))
  const right = actions.filter((a) => !left.includes(a))

  const render = (a: Action) => {
    if (a.kind === 'link') {
      const Icon = a.icon ? icons[a.icon] : null
      return (
        <ButtonLink key={a.label} href={a.href} variant={a.primary ? 'primary' : 'secondary'}>
          {Icon && <Icon aria-hidden />}
          {a.label}
        </ButtonLink>
      )
    }
    const danger = a.kind === 'delete' || (a.kind === 'patch' && a.danger)
    return (
      <Button
        key={a.label}
        variant={danger ? 'danger' : 'primary' in a && a.primary ? 'primary' : 'secondary'}
        loading={busy === a.label}
        loadingText={t.actions.working}
        disabled={busy !== null}
        onClick={() => void run(a)}
      >
        {a.label}
      </Button>
    )
  }

  if (actions.length === 0) return null
  return (
    <div className="flex flex-col gap-3 print:hidden">
      {error && <Alert tone="error">{error}</Alert>}
      {warnings.length > 0 && (
        <Alert>
          <span className="font-medium">{t.warnings}: </span>
          {warnings.map(warningText).join(' · ')}
        </Alert>
      )}
      <div className="flex flex-wrap items-center gap-2 rounded-(--radius-panel) border border-line bg-surface px-4 py-3 shadow-card">
        {left.map(render)}
        <div className="ml-auto flex flex-wrap gap-2">{right.map(render)}</div>
      </div>
    </div>
  )
}

export type { Action as DocAction }
