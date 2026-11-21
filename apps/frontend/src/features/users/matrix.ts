import { covers } from '@/lib/permissions'
import type { PermissionEntry } from './types'

/**
 * โครงตารางสิทธิ์ (pure — ทดสอบง่าย): จัดกลุ่มตาม module → แถว resource → คอลัมน์ action
 * ลำดับ module / resource ตามลำดับที่ backend ลงทะเบียน; action เรียง read ก่อน แล้วตามลำดับที่พบ
 */
export type MatrixRow = { resource: string; key: string; cells: Record<string, string | undefined> }
export type MatrixGroup = { module: string; actions: string[]; rows: MatrixRow[]; codes: string[] }

const ACTION_ORDER = ['read', 'manage', 'approve', 'post', 'close']

export function buildMatrix(catalog: readonly PermissionEntry[]): MatrixGroup[] {
  const groups = new Map<string, MatrixGroup>()
  for (const p of catalog) {
    let g = groups.get(p.module)
    if (!g) {
      g = { module: p.module, actions: [], rows: [], codes: [] }
      groups.set(p.module, g)
    }
    if (!g.actions.includes(p.action)) g.actions.push(p.action)
    let row = g.rows.find((r) => r.resource === p.resource)
    if (!row) {
      row = { resource: p.resource, key: `${p.module}.${p.resource}`, cells: {} }
      g.rows.push(row)
    }
    row.cells[p.action] = p.code
    g.codes.push(p.code)
  }
  for (const g of groups.values()) {
    g.actions.sort((a, b) => rank(a) - rank(b))
  }
  return [...groups.values()]
}

function rank(action: string): number {
  const i = ACTION_ORDER.indexOf(action)
  return i === -1 ? ACTION_ORDER.length : i
}

/** สิทธิ์ของ role (อาจมี wildcard เช่น "*", "sales.order:*") → รหัสในรายการที่ครอบคลุม */
export function expandGranted(granted: readonly string[], catalog: readonly PermissionEntry[]): Set<string> {
  return new Set(catalog.filter((p) => granted.some((g) => covers(g, p.code))).map((p) => p.code))
}

/**
 * เลือก / ยกเลิกช่องเดียว พร้อมกฎ "ต้องเห็นก่อนจึงทำได้":
 * - เลือก action อื่น → เลือก read ของ resource เดียวกันด้วย
 * - ยกเลิก read → ยกเลิกทุก action ของ resource นั้น
 */
export function toggle(selected: ReadonlySet<string>, row: MatrixRow, action: string, on: boolean): Set<string> {
  const next = new Set(selected)
  const code = row.cells[action]
  if (!code) return next
  if (on) {
    next.add(code)
    if (action !== 'read' && row.cells.read) next.add(row.cells.read)
  } else if (action === 'read') {
    for (const c of Object.values(row.cells)) if (c) next.delete(c)
  } else {
    next.delete(code)
  }
  return next
}

/** เลือก / ยกเลิกทั้ง module */
export function toggleGroup(selected: ReadonlySet<string>, group: MatrixGroup, on: boolean): Set<string> {
  const next = new Set(selected)
  for (const c of group.codes) {
    if (on) next.add(c)
    else next.delete(c)
  }
  return next
}

/** สิทธิ์ที่บันทึก: เรียงตามลำดับรายการสิทธิ์ (ผลเหมือนเดิมทุกครั้ง) */
export function toList(selected: ReadonlySet<string>, catalog: readonly PermissionEntry[]): string[] {
  return catalog.map((p) => p.code).filter((c) => selected.has(c))
}
