'use client'

import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { monthLabel } from './period'

/**
 * กราฟแท่งรายเดือน 12 เดือน (shadcn Chart + Recharts) — 1–2 series, สีช่อง 1–2 ของ palette ที่ผ่านตัวตรวจ (chart-1 / chart-2)
 * แท่งบาง มุมบนมน 4px ชิดฐาน, ช่องว่างระหว่างแท่ง, grid จาง, แกนเดียว; hover = tooltip ทุก series ของเดือนนั้น
 * ไม่มี animation (ตัวเลขขึ้นทันที, พิมพ์ / ลดการเคลื่อนไหวได้ภาพครบ); 2 series มี legend เรียงตาม series; ทุกกราฟมีตารางข้อมูล (ปุ่ม "ดูเป็นตาราง") สำหรับ screen reader / การพิมพ์
 */
export type TrendSeries = { key: string; label: string }

const COLORS = ['var(--color-chart-1)', 'var(--color-chart-2)']

const compact = new Intl.NumberFormat('th-TH', { notation: 'compact', maximumFractionDigits: 1 })

export function TrendChart({
  title,
  description,
  data,
  series,
}: {
  title: string
  description?: string
  /** month = "YYYY-MM", ค่าแต่ละ series เป็น string จำนวนเงิน */
  data: ({ month: string } & Record<string, string>)[]
  series: TrendSeries[]
}) {
  const config: ChartConfig = Object.fromEntries(series.map((s, i) => [s.key, { label: s.label, color: COLORS[i] }]))
  const rows = data.map((d) => ({ month: d.month, ...Object.fromEntries(series.map((s) => [s.key, Number(d[s.key] ?? 0)])) }))
  const t = texts.dashboard.table
  const empty = rows.every((r) => series.every((x) => !(r as Record<string, unknown>)[x.key]))
  return (
    <Card className="min-w-0 lg:col-span-2">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-3">
{empty ? (
          <p className="flex h-64 items-center justify-center text-sm text-ink-muted">{texts.dashboard.noData}</p>
        ) : (
        <ChartContainer config={config} className="aspect-auto h-64 w-full" role="img" aria-label={`${title}: ${series.map((s) => s.label).join(', ')}`}>
          <BarChart data={rows} barGap={2} barCategoryGap="28%" margin={{ top: 8, right: 4, left: 0, bottom: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--color-border)" />
            <XAxis dataKey="month" tickLine={false} axisLine={false} tickMargin={8} minTickGap={8} tickFormatter={monthLabel} />
            <YAxis tickLine={false} axisLine={false} width={48} tickFormatter={(v: number) => compact.format(v)} />
            <ChartTooltip
              cursor={{ fill: 'var(--color-muted)' }}
              content={<ChartTooltipContent labelFormatter={(l) => monthLabel(String(l))} valueFormatter={(v) => formatAmount(String(v))} />}
            />
            {series.length > 1 && <ChartLegend content={<ChartLegendContent />} />}
            {series.map((s) => (
              <Bar key={s.key} dataKey={s.key} name={s.label} fill={`var(--color-${s.key})`} radius={[4, 4, 0, 0]} maxBarSize={28} isAnimationActive={false} />
            ))}
          </BarChart>
        </ChartContainer>
        )}
        <details className="text-sm">
          <summary className="w-fit cursor-pointer rounded-(--radius-control) text-[13px] font-medium text-primary hover:underline">{t.show}</summary>
          <div className="mt-2 overflow-x-auto">
            <table className="w-full min-w-[20rem] border-collapse text-[13px]">
              <thead>
                <tr className="border-b border-line text-left text-ink-muted">
                  <th scope="col" className="py-1.5 pr-3 font-medium">
                    {t.month}
                  </th>
                  {series.map((s) => (
                    <th key={s.key} scope="col" className="py-1.5 pl-3 text-right font-medium">
                      {s.label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {data.map((d) => (
                  <tr key={d.month} className="border-b border-line last:border-b-0">
                    <th scope="row" className="py-1.5 pr-3 text-left font-normal">
                      {monthLabel(d.month)}
                    </th>
                    {series.map((s) => (
                      <td key={s.key} className="py-1.5 pl-3 text-right tabular-nums">
                        {formatAmount(d[s.key])}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      </CardContent>
    </Card>
  )
}
