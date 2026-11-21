import { loadRef } from '@/lib/server/list'
import { ledgerApi, type EntryLine, type Ref } from './types'

/** API ที่ฟอร์มรายการบัญชีใช้ (ค้นหาบัญชี / สมุด / มิติวิเคราะห์) */
export function entryFormApi(companyId: string) {
  return {
    entries: ledgerApi.entries(companyId),
    accounts: ledgerApi.accounts(companyId),
    journals: ledgerApi.journals(companyId),
    branches: ledgerApi.branches(companyId),
    departments: ledgerApi.departments(companyId),
    costCenters: ledgerApi.costCenters(companyId),
    projectSites: ledgerApi.projectSites(companyId),
  }
}

/**
 * ชื่อของมิติวิเคราะห์ที่บรรทัดอ้างถึง (server) — API ของรายการส่งเฉพาะ id
 * โหลดเฉพาะ id ที่ไม่ซ้ำ พร้อมกันทั้งหมด (ข้อมูลอยู่ใน module organization จึง JOIN ในคิวรีของ ledger ไม่ได้)
 * รายการส่วนใหญ่ไม่มีมิติ = ไม่มีคำขอเพิ่ม
 */
export async function loadDimensionRefs(companyId: string, lines: readonly EntryLine[]): Promise<Record<string, Ref>> {
  const sources: [keyof EntryLine, string][] = [
    ['branch_id', ledgerApi.branches(companyId)],
    ['department_id', ledgerApi.departments(companyId)],
    ['cost_center_id', ledgerApi.costCenters(companyId)],
    ['project_site_id', ledgerApi.projectSites(companyId)],
  ]
  const wanted = new Map<string, string>()
  for (const l of lines) {
    for (const [key, api] of sources) {
      const id = l[key] as string | null
      if (id) wanted.set(id, api)
    }
  }
  const loaded = await Promise.all([...wanted].map(([id, api]) => loadRef(api, id)))
  return Object.fromEntries(loaded.filter((r): r is Ref => r !== null).map((r) => [r.id, r]))
}
