'use client'

import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Badge, Button } from '@/components'
import { format, texts } from '@/config'
import { formatDate } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import type { FiscalPeriod } from './resources'

/**
 * งวดบัญชีของปีบัญชี — ปิด / เปิดทีละงวด (PATCH /fiscal-periods/{id} {status})
 * ปุ่มแสดงเฉพาะงวดที่ทำได้ตามลำดับ: ปิด = งวดแรกที่ยังเปิด, เปิด = งวดล่าสุดที่ปิด
 * (backend ตรวจลำดับอีกชั้น รวมถึงข้ามปีบัญชี)
 */
export function PeriodTable({ apiPath, periods, canClose }: { apiPath: string; periods: FiscalPeriod[]; canClose: boolean }) {
  const t = texts.fiscalYears.periods
  const router = useRouter()
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const sorted = [...periods].sort((a, b) => a.period_no - b.period_no)
  const closedCount = sorted.filter((p) => p.status === 'closed').length
  const firstOpen = sorted.find((p) => p.status === 'open')?.id
  const lastClosed = [...sorted].reverse().find((p) => p.status === 'closed')?.id

  async function toggle(p: FiscalPeriod) {
    const closing = p.status === 'open'
    const message = format(closing ? t.closeConfirm : t.reopenConfirm, { no: p.period_no, start: formatDate(p.start_date), end: formatDate(p.end_date) })
    if (!window.confirm(message)) return
    setError(null)
    setBusy(p.id)
    try {
      await apiRequest(`${apiPath}/${p.id}`, { method: 'PATCH', body: { status: closing ? 'closed' : 'open' } })
      showToast(format(closing ? t.closedToast : t.reopenedToast, { no: p.period_no }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-[15px] font-semibold">{t.heading}</h2>
          {canClose && <p className="mt-0.5 text-[13px] text-ink-muted">{t.orderHint}</p>}
        </div>
        <Progress closed={closedCount} total={sorted.length} />
      </div>
      {error && <Alert tone="error">{error}</Alert>}
      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full border-collapse text-sm tabular-nums">
          <thead className="bg-subtle text-[13px] text-ink-muted">
            <tr>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.no}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.startDate}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.endDate}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.status}</th>
              {canClose && <th scope="col" className="h-10 border-b border-line px-4" />}
            </tr>
          </thead>
          <tbody>
            {sorted.map((p) => {
              const action = p.id === firstOpen ? 'close' : p.id === lastClosed ? 'reopen' : null
              return (
                <tr key={p.id} className="h-12 border-b border-line last:border-b-0 hover:bg-subtle">
                  <td className="px-4 font-medium">{p.period_no}</td>
                  <td className="px-4">{formatDate(p.start_date)}</td>
                  <td className="px-4">{formatDate(p.end_date)}</td>
                  <td className="px-4">
                    <Badge tone={p.status === 'open' ? 'success' : 'neutral'} dot>
                      {t.statuses[p.status] ?? p.status}
                    </Badge>
                  </td>
                  {canClose && (
                    <td className="px-4 py-2 text-right">
                      {action && (
                        <Button
                          size="sm"
                          variant={action === 'close' ? 'primary' : 'secondary'}
                          loading={busy === p.id}
                          loadingText={action === 'close' ? t.closing : t.reopening}
                          disabled={busy !== null}
                          onClick={() => toggle(p)}
                        >
                          {action === 'close' ? t.close : t.reopen}
                        </Button>
                      )}
                    </td>
                  )}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </section>
  )
}

/** ความคืบหน้าการปิดงวด: ข้อความ + แถบ (มีข้อความบอกจำนวนเสมอ ไม่พึ่งสีอย่างเดียว) */
function Progress({ closed, total }: { closed: number; total: number }) {
  const label = format(texts.fiscalYears.periods.progress, { closed, total })
  return (
    <div className="w-full sm:w-64">
      <p className="mb-1.5 text-right text-[13px] font-medium tabular-nums text-ink-muted">{label}</p>
      <div role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={total} aria-valuenow={closed} className="h-2 overflow-hidden rounded-full bg-neutral-soft">
        <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${total ? (closed / total) * 100 : 0}%` }} />
      </div>
    </div>
  )
}
