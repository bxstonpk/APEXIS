import { describe, expect, it } from 'vitest'
import { visibleApps } from '@/config/apps'
import { can, covers } from './permissions'

describe('covers (กติกาเดียวกับ backend)', () => {
  it.each([
    ['*', 'sales.order:read', true],
    ['sales.order:read', 'sales.order:read', true],
    ['sales.order:read', 'sales.order:manage', false],
    ['sales.order:*', 'sales.order:approve', true],
    ['sales.*:read', 'sales.customer:read', true],
    ['sales.*:read', 'purchasing.order:read', false],
    ['sales.*:read', 'sales.customer:manage', false],
    ['sales.order:read', '*', false],
  ])('%s ครอบ %s = %s', (granted, required, expected) => {
    expect(covers(granted, required)).toBe(expected)
  })

  it('can = มีสิทธิ์ใดสิทธิ์หนึ่งครอบ', () => {
    expect(can(['ledger.report:read', 'sales.*:read'], 'sales.order:read')).toBe(true)
    expect(can([], 'sales.order:read')).toBe(false)
  })
})

describe('visibleApps', () => {
  it('owner (*) เห็นทุกแอปและทุกเมนู', () => {
    const apps = visibleApps(['*'])
    expect(apps.map((a) => a.key)).toEqual(['sales', 'purchasing', 'inventory', 'accounting', 'finance', 'organization', 'users'])
    expect(apps.find((a) => a.key === 'organization')!.menu[0]!.path).toBe('branches')
  })

  it('เหลือเฉพาะเมนูที่มีสิทธิ์ และซ่อนแอปที่ไม่เหลือเมนู', () => {
    const apps = visibleApps(['sales.customer:read', 'sales.report:read'])
    expect(apps).toHaveLength(1)
    expect(apps[0]!.key).toBe('sales')
    expect(apps[0]!.menu.map((m) => m.path)).toEqual(['dashboard', 'customers', 'reports']) // ภาพรวมใช้สิทธิ์รายงาน
  })

  it('สิทธิ์ manage อย่างเดียวไม่ทำให้เห็นเมนู (ต้องมี read)', () => {
    expect(visibleApps(['organization.branch:manage'])).toEqual([])
  })
})
