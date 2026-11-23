'use client'

import * as React from 'react'
import * as RechartsPrimitive from 'recharts'
import { cn } from '@/lib/utils'

/**
 * shadcn/ui Chart (Recharts) — ChartContainer กำหนดสีต่อ series เป็น CSS variable --color-{key} จาก config
 * แล้วกราฟอ้าง fill="var(--color-{key})"; tooltip / legend ใช้ label จาก config (ข้อความไทยมาจาก th.ts)
 */

export type ChartConfig = Record<string, { label?: React.ReactNode; icon?: React.ComponentType; color?: string }>

const ChartContext = React.createContext<{ config: ChartConfig } | null>(null)

function useChart() {
  const ctx = React.useContext(ChartContext)
  if (!ctx) throw new Error('useChart must be used within a <ChartContainer />')
  return ctx
}

function ChartContainer({
  id,
  className,
  children,
  config,
  ...props
}: React.ComponentProps<'div'> & {
  config: ChartConfig
  children: React.ComponentProps<typeof RechartsPrimitive.ResponsiveContainer>['children']
}) {
  const uniqueId = React.useId()
  const chartId = `chart-${id || uniqueId.replace(/:/g, '')}`
  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-slot="chart"
        data-chart={chartId}
        className={cn(
          "flex aspect-video justify-center text-xs [&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground [&_.recharts-cartesian-grid_line[stroke='#ccc']]:stroke-border [&_.recharts-layer]:outline-hidden [&_.recharts-rectangle.recharts-tooltip-cursor]:fill-muted [&_.recharts-surface]:outline-hidden",
          className,
        )}
        {...props}
      >
        <ChartStyle id={chartId} config={config} />
        <RechartsPrimitive.ResponsiveContainer>{children}</RechartsPrimitive.ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  )
}

function ChartStyle({ id, config }: { id: string; config: ChartConfig }) {
  const colors = Object.entries(config).filter(([, c]) => c.color)
  if (!colors.length) return null
  return <style dangerouslySetInnerHTML={{ __html: `[data-chart=${id}] {\n${colors.map(([key, c]) => `  --color-${key}: ${c.color};`).join('\n')}\n}` }} />
}

const ChartTooltip = RechartsPrimitive.Tooltip

type PayloadItem = { dataKey?: string | number; name?: string | number; value?: number | string; color?: string; fill?: string }

function ChartTooltipContent({
  active,
  payload,
  label,
  className,
  hideLabel = false,
  labelFormatter,
  valueFormatter = (v) => String(v),
}: {
  active?: boolean
  payload?: readonly PayloadItem[]
  label?: React.ReactNode
  className?: string
  hideLabel?: boolean
  labelFormatter?: (label: React.ReactNode) => React.ReactNode
  valueFormatter?: (value: number | string) => React.ReactNode
}) {
  const { config } = useChart()
  if (!active || !payload?.length) return null
  return (
    <div className={cn('grid min-w-36 gap-1.5 rounded-(--radius-control) border border-border bg-card px-3 py-2 text-xs shadow-pop', className)}>
      {!hideLabel && <div className="font-medium text-card-foreground">{labelFormatter ? labelFormatter(label) : label}</div>}
      <div className="grid gap-1">
        {payload.map((item) => {
          const key = String(item.dataKey ?? item.name ?? 'value')
          const color = item.color ?? item.fill ?? `var(--color-${key})`
          return (
            <div key={key} className="flex items-center gap-2">
              <span aria-hidden className="size-2.5 shrink-0 rounded-[2px]" style={{ background: color }} />
              <span className="text-muted-foreground">{config[key]?.label ?? item.name}</span>
              <span className="ml-auto font-medium text-card-foreground tabular-nums">{item.value !== undefined ? valueFormatter(item.value) : '–'}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

const ChartLegend = RechartsPrimitive.Legend

function ChartLegendContent({ payload, className }: { payload?: readonly PayloadItem[]; className?: string }) {
  const { config } = useChart()
  if (!payload?.length) return null
  // เรียงตามลำดับ key ใน config (= ลำดับ series) — Recharts เรียง legend เองตามชื่อ
  const order = Object.keys(config)
  const items = [...payload].sort((a, b) => order.indexOf(String(a.dataKey)) - order.indexOf(String(b.dataKey)))
  return (
    <div className={cn('flex flex-wrap items-center justify-center gap-4 pt-3', className)}>
      {items.map((item) => {
        const key = String(item.dataKey ?? 'value')
        return (
          <div key={key} className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span aria-hidden className="size-2.5 shrink-0 rounded-[2px]" style={{ background: item.color }} />
            {config[key]?.label ?? item.name}
          </div>
        )
      })}
    </div>
  )
}

export { ChartContainer, ChartLegend, ChartLegendContent, ChartStyle, ChartTooltip, ChartTooltipContent }
