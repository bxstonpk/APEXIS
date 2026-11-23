import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { texts } from '@/config'
import { changePercent, KpiCard } from './parts'
import { monthLabel, resolvePeriod } from './period'

describe('resolvePeriod', () => {
  const today = '2026-11-23'
  it('ค่าเริ่มต้น = ต้นเดือนถึงวันนี้; ไตรมาส / ปีนับจากต้นไตรมาส / ต้นปี', () => {
    expect(resolvePeriod({}, today)).toEqual({ preset: 'month', start: '2026-11-01', end: today })
    expect(resolvePeriod({ period: 'quarter' }, today)).toEqual({ preset: 'quarter', start: '2026-10-01', end: today })
    expect(resolvePeriod({ period: 'year' }, today)).toEqual({ preset: 'year', start: '2026-01-01', end: today })
  })
  it('กำหนดเองต้องมีวันที่ถูกต้องและเริ่ม <= สิ้นสุด ไม่งั้นกลับเป็นเดือนนี้', () => {
    expect(resolvePeriod({ period: 'custom', start_date: '2026-01-01', end_date: '2026-03-31' }, today)).toEqual({ preset: 'custom', start: '2026-01-01', end: '2026-03-31' })
    expect(resolvePeriod({ period: 'custom', start_date: '2026-05-01', end_date: '2026-03-31' }, today).preset).toBe('month')
    expect(resolvePeriod({ period: 'weird' }, today).preset).toBe('month')
  })
  it('ชื่อเดือนแบบไทยย่อ', () => {
    expect(monthLabel('2026-01')).toBe('ม.ค. 69')
  })
})

describe('changePercent / KpiCard', () => {
  it('คำนวณ % จากช่วงก่อน และช่วงก่อนเป็น 0 = null', () => {
    expect(changePercent('150', '100')).toBe(50)
    expect(changePercent('50', '100')).toBe(-50)
    expect(changePercent('10', '0')).toBeNull()
  })
  it('บอกทิศทางด้วยข้อความ (ไม่ใช่สีอย่างเดียว) และเป็นลิงก์เมื่อมี href', () => {
    render(<KpiCard label="ยอดขาย" value="1,500.00" delta={{ current: '1500', previous: '1000' }} href="/x" />)
    expect(screen.getByText('เพิ่มขึ้น 50% จากช่วงก่อน')).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/x')
  })
  it('ค่าใช้จ่ายเพิ่มขึ้น (upIsGood=false) ยังบอกว่าเพิ่มขึ้น แต่ไม่ใช้สีดี', () => {
    render(<KpiCard label="ค่าใช้จ่าย" value="2" delta={{ current: '2', previous: '1', upIsGood: false }} />)
    const p = screen.getByText('เพิ่มขึ้น 100% จากช่วงก่อน')
    expect(p.className).toContain('text-danger')
    expect(texts.dashboard.compare.noPrevious).toBeTruthy()
  })
})
