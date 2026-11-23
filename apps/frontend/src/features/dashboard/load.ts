import { today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet } from '@/lib/server/backend'
import type { SearchParams } from '@/lib/url'
import { periodQuery, resolvePeriod, type Period } from './period'

/** โหลดข้อมูลภาพรวม 1 คำขอ (backend รวมด้วย aggregate query) — error ไม่ throw, หน้าแสดงข้อความแทน */
export async function loadDashboard<T>(path: string, sp: SearchParams): Promise<{ period: Period; data: T | null; error: string | null }> {
  const period = resolvePeriod(sp, today())
  try {
    const { data } = await backendGet<T>(path, periodQuery(period))
    return { period, data, error: null }
  } catch (err) {
    return { period, data: null, error: errorMessage(err) }
  }
}
