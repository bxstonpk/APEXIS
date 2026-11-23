import { ArrowDownRight, ArrowRight, ArrowUpRight, CircleAlert } from 'lucide-react'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert } from '@/components'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { PeriodFilter } from './PeriodFilter'
import type { Period } from './period'

/** ส่วนประกอบของหน้าภาพรวม (server) — shadcn Card + โทเคนของ APEXIS */

export function DashboardHeader({ company, title, description, period }: { company: string; title: string; description: string; period: Period }) {
  return (
    <header className="flex flex-col gap-4 border-b border-line pb-5 lg:flex-row lg:items-end lg:justify-between">
      <div className="min-w-0">
        <p className="text-[13px] text-ink-muted">{company}</p>
        <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{title}</h1>
        <p className="mt-1 text-sm text-ink-muted">
          {description} · {format(texts.reports.period, { start: formatDate(period.start), end: formatDate(period.end) })}
        </p>
      </div>
      <PeriodFilter period={period} />
    </header>
  )
}

export function DashboardError({ message }: { message: string }) {
  return <Alert tone="error">{message}</Alert>
}

/** เปอร์เซ็นต์เปลี่ยนแปลงเทียบช่วงก่อน — ช่วงก่อนเป็น 0 = null */
export function changePercent(current: string, previous: string): number | null {
  const c = toUnits(current)
  const p = toUnits(previous)
  if (p === 0n) return null
  return Number(((c - p) * 10000n) / (p < 0n ? -p : p)) / 100
}

/**
 * ตัวเลขหลัก (KPI) — ทั้งการ์ดเป็นลิงก์ไปหน้ารายการที่กรองไว้แล้วเมื่อมี href
 * delta: บอกทั้งไอคอนลูกศรและข้อความ (ไม่ใช้สีอย่างเดียว); upIsGood=false สำหรับตัวเลขที่ยิ่งน้อยยิ่งดี (เช่น ค่าใช้จ่าย)
 */
export function KpiCard({
  label,
  value,
  sub,
  href,
  delta,
  warn,
}: {
  label: string
  value: string
  sub?: ReactNode
  href?: string | null
  delta?: { current: string; previous: string; upIsGood?: boolean }
  warn?: boolean
}) {
  const t = texts.dashboard.compare
  const pct = delta ? changePercent(delta.current, delta.previous) : null
  const good = pct === null || pct === 0 ? null : (pct > 0) === (delta?.upIsGood ?? true)
  const Arrow = pct === null || pct === 0 ? ArrowRight : pct > 0 ? ArrowUpRight : ArrowDownRight
  const body = (
    <Card className={cn('h-full gap-2 py-4', href && 'transition-colors group-hover:border-ink-subtle group-focus-visible:border-primary', warn && 'border-warning/40 bg-warning-soft/40')}>
      <CardHeader className="px-4">
        <CardDescription className="flex items-center gap-1.5">
          {warn && <CircleAlert aria-hidden className="size-3.5 text-warning" />}
          {label}
        </CardDescription>
        {href && (
          <CardAction>
            <ArrowRight aria-hidden className="size-4 text-ink-subtle transition-transform group-hover:translate-x-0.5" />
          </CardAction>
        )}
      </CardHeader>
      <CardContent className="flex flex-col gap-1 px-4">
        <p className="text-2xl leading-tight font-semibold tracking-tight tabular-nums">{value}</p>
        {delta && (
          <p className={cn('flex items-center gap-1 text-[13px]', good === true ? 'text-success' : good === false ? 'text-danger' : 'text-ink-muted')}>
            <Arrow aria-hidden className="size-3.5" />
            {pct === null ? t.noPrevious : format(pct >= 0 ? t.up : t.down, { pct: Math.abs(pct).toLocaleString('th-TH', { maximumFractionDigits: 1 }) })}
          </p>
        )}
        {sub && <p className="text-[13px] text-ink-muted">{sub}</p>}
      </CardContent>
    </Card>
  )
  return href ? (
    <Link href={href} className="group block rounded-(--radius-panel) focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary">
      {body}
    </Link>
  ) : (
    body
  )
}

export function KpiGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">{children}</div>
}

export function Panel({ title, description, action, children, className }: { title: string; description?: string; action?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <Card className={cn('min-w-0', className)}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
        {action && <CardAction>{action}</CardAction>}
      </CardHeader>
      <CardContent className="min-w-0">{children}</CardContent>
    </Card>
  )
}

export function PanelLink({ href, label }: { href: string; label: string }) {
  return (
    <Link href={href} className="inline-flex items-center gap-1 text-[13px] font-medium text-primary hover:underline">
      {label}
      <ArrowRight aria-hidden className="size-3.5" />
    </Link>
  )
}

/**
 * แถบแนวนอนเทียบขนาด (อายุหนี้ / อันดับ) — ความยาวสัมพันธ์กับค่ามากสุด, ตัวเลขและชื่อเป็นข้อความเสมอ (ไม่พึ่งสี)
 * สีเดียว (chart-1) เพราะเป็นขนาดของสิ่งเดียวกัน; แถวที่ emphasize (เช่น เกินกำหนด) เพิ่มน้ำหนักตัวอักษร
 */
export function BarList({ rows, empty }: { rows: { key: string; label: ReactNode; value: string; href?: string; emphasize?: boolean }[]; empty?: string }) {
  const max = rows.reduce((m, r) => (toUnits(r.value) > m ? toUnits(r.value) : m), 0n)
  if (rows.length === 0 || max === 0n) return <p className="py-6 text-center text-sm text-ink-muted">{empty ?? texts.dashboard.noData}</p>
  return (
    <ul className="flex flex-col gap-3">
      {rows.map((r) => {
        const pct = max > 0n ? Number((toUnits(r.value) * 1000n) / max) / 10 : 0
        const label = r.href ? (
          <Link href={r.href} className="truncate hover:text-primary hover:underline">
            {r.label}
          </Link>
        ) : (
          <span className="truncate">{r.label}</span>
        )
        return (
          <li key={r.key} className="flex flex-col gap-1">
            <div className={cn('flex items-baseline justify-between gap-3 text-sm', r.emphasize && 'font-medium')}>
              {label}
              <span className="shrink-0 tabular-nums">{formatAmount(r.value)}</span>
            </div>
            <div aria-hidden className="h-2 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-chart-1" style={{ width: `${Math.max(pct, toUnits(r.value) > 0n ? 2 : 0)}%` }} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}

/** รายการล่าสุด / งานค้าง — แถวกระชับ, ทั้งแถวเป็นลิงก์ */
export function RowList({ rows, empty }: { rows: { key: string; href?: string | null; title: ReactNode; meta?: ReactNode; value?: string }[]; empty?: string }) {
  if (rows.length === 0) return <p className="py-6 text-center text-sm text-ink-muted">{empty ?? texts.dashboard.noData}</p>
  return (
    <ul className="-mx-2 flex flex-col">
      {rows.map((r) => {
        const inner = (
          <>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm">{r.title}</span>
              {r.meta && <span className="block truncate text-[12px] text-ink-muted">{r.meta}</span>}
            </span>
            {r.value !== undefined && <span className="shrink-0 text-sm tabular-nums">{formatAmount(r.value)}</span>}
          </>
        )
        return (
          <li key={r.key}>
            {r.href ? (
              <Link href={r.href} className="flex items-center gap-3 rounded-(--radius-control) px-2 py-2 hover:bg-subtle">
                {inner}
              </Link>
            ) : (
              <div className="flex items-center gap-3 px-2 py-2">{inner}</div>
            )}
          </li>
        )
      })}
    </ul>
  )
}

/** อายุหนี้: 5 ช่วง — ช่วงเกินกำหนดเน้นตัวหนา */
export function AgingList({ aging, href }: { aging: { not_due: string; days_1_30: string; days_31_60: string; days_61_90: string; over_90: string }; href?: string }) {
  const c = texts.dashboard.aging
  const rows = [
    { key: 'not_due', label: c.notDue, value: aging.not_due },
    { key: 'd1', label: c.d1, value: aging.days_1_30, emphasize: true },
    { key: 'd31', label: c.d31, value: aging.days_31_60, emphasize: true },
    { key: 'd61', label: c.d61, value: aging.days_61_90, emphasize: true },
    { key: 'd90', label: c.d90, value: aging.over_90, emphasize: true },
  ]
  return <BarList rows={rows.map((r) => ({ ...r, href }))} empty={c.empty} />
}

export function DashboardGrid({ children }: { children: ReactNode }) {
  return <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">{children}</div>
}
