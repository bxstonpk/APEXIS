import { appConfig } from '@/config'
import { errorMessage } from '@/lib/http'
import { pickQuery, type SearchParams } from '@/lib/url'
import { backendGet, type ListMeta } from './backend'

export type ListResult<T> = {
  rows: T[]
  meta: ListMeta
  /** ข้อความ error จาก config (ไม่มี = สำเร็จ) */
  error: string | null
  /** เงื่อนไขจาก URL ที่อนุญาต — ใช้สร้างลิงก์เรียง / แบ่งหน้า / กลับหน้ารายการ */
  query: URLSearchParams
}

/**
 * โหลดหน้ารายการ: เลือก key ที่อนุญาตจาก URL แล้วส่งต่อให้ API ตรงๆ (คิวรี 1 ครั้ง — กรอง / เรียง / แบ่งหน้าใน DB)
 * error ไม่ throw — หน้ารายการแสดงข้อความแทนตาราง
 */
export async function loadList<T>(path: string, params: SearchParams, keys: readonly string[]): Promise<ListResult<T>> {
  const query = pickQuery(params, keys)
  const apiQuery = new URLSearchParams(query)
  if (!apiQuery.has('page_size')) apiQuery.set('page_size', String(appConfig.pageSize))
  const empty: ListMeta = { page: 1, page_size: appConfig.pageSize, total: 0 }
  try {
    const res = await backendGet<T[]>(path, apiQuery)
    return { rows: res.data, meta: res.meta ?? empty, error: null, query }
  } catch (err) {
    return { rows: [], meta: empty, error: errorMessage(err), query }
  }
}

export type Ref = { id: string; code: string; name: string }

/**
 * ข้อมูลอ้างอิงที่ใช้เป็นตัวกรองใน URL (เช่น ?branch_id=) — โหลดเพื่อแสดงชื่อในช่องกรองตอนเปิดหน้า
 * เรียกเฉพาะเมื่อ URL มีตัวกรองนั้น (คิวรีเพิ่ม 1 ครั้งต่อตัวกรองที่ใช้ — เรียกพร้อมกับรายการได้)
 * ไม่พบ / ไม่มีสิทธิ์ = null (ช่องกรองว่าง แต่ URL ยังกรองตาม id เดิม)
 */
export async function loadRef(path: string, id: string): Promise<Ref | null> {
  if (!id) return null
  try {
    const { data } = await backendGet<Ref>(`${path}/${encodeURIComponent(id)}`)
    return { id: data.id, code: data.code, name: data.name }
  } catch {
    return null
  }
}

/** ?back= ของหน้าแก้ไข → URL หน้ารายการพร้อมเงื่อนไขเดิม (กรองเหลือเฉพาะ key ของรายการ) */
export function listHrefFromBack(listPath: string, back: string, keys: readonly string[]): string {
  const q = pickQuery(Object.fromEntries(new URLSearchParams(back)), keys).toString()
  return q ? `${listPath}?${q}` : listPath
}
