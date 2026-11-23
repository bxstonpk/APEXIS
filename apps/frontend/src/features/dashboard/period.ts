import { param, type SearchParams } from '@/lib/url'

/**
 * ช่วงเวลาของ dashboard จาก URL — ?period=month|quarter|year|custom&start_date=&end_date=
 * ค่าเริ่มต้น = เดือนนี้ถึงวันนี้; ไตรมาส / ปี = ตั้งแต่ต้นไตรมาส / ต้นปีปฏิทินถึงวันนี้; custom ผิดรูปแบบ = กลับเป็นเดือนนี้
 * backend เทียบกับ "ช่วงก่อน" (จำนวนวันเท่ากันก่อนหน้า) และกราฟรายเดือน = 12 เดือนถึงเดือนของ end_date
 */
export const PRESETS = ['month', 'quarter', 'year', 'custom'] as const
export type Preset = (typeof PRESETS)[number]
export type Period = { preset: Preset; start: string; end: string }

const ISO = /^\d{4}-\d{2}-\d{2}$/

export function resolvePeriod(sp: SearchParams, today: string): Period {
  const raw = param(sp, 'period')
  const preset: Preset = (PRESETS as readonly string[]).includes(raw) ? (raw as Preset) : 'month'
  const [y, m] = today.split('-')
  if (preset === 'custom') {
    const start = param(sp, 'start_date')
    const end = param(sp, 'end_date')
    if (ISO.test(start) && ISO.test(end) && start <= end) return { preset, start, end }
    return { preset: 'month', start: `${y}-${m}-01`, end: today }
  }
  if (preset === 'year') return { preset, start: `${y}-01-01`, end: today }
  if (preset === 'quarter') {
    const q = Math.floor((Number(m) - 1) / 3) * 3 + 1
    return { preset, start: `${y}-${String(q).padStart(2, '0')}-01`, end: today }
  }
  return { preset, start: `${y}-${m}-01`, end: today }
}

export function periodQuery(p: Period): URLSearchParams {
  return new URLSearchParams({ start_date: p.start, end_date: p.end })
}

/** "2026-01" → "ม.ค. 69" */
export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number)
  return new Intl.DateTimeFormat('th-TH', { month: 'short', year: '2-digit', timeZone: 'UTC' }).format(new Date(Date.UTC(y!, m! - 1, 1)))
}
