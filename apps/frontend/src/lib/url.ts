/** ค่า searchParams ของ Next (page props) */
export type SearchParams = Record<string, string | string[] | undefined>

/** ค่าแรกของ query parameter (ไม่มี = '') */
export function param(params: SearchParams, key: string): string {
  const v = params[key]
  return (Array.isArray(v) ? v[0] : v) ?? ''
}

/** เลือกเฉพาะ key ที่อนุญาตและมีค่า — ใช้ส่งต่อเงื่อนไขค้นหาจาก URL ไป API ตรงๆ */
export function pickQuery(params: SearchParams, keys: readonly string[]): URLSearchParams {
  const out = new URLSearchParams()
  for (const key of keys) {
    const v = param(params, key).trim()
    if (v) out.set(key, v)
  }
  return out
}

/**
 * path + query ใหม่จาก query ปัจจุบัน โดยแก้บาง key (null / '' = ลบ key)
 * เช่น withQuery('/x', {q: 'a', page: '3'}, {page: null}) = '/x?q=a'
 */
export function withQuery(path: string, current: URLSearchParams | Record<string, string>, patch: Record<string, string | number | null> = {}): string {
  const q = new URLSearchParams(current)
  for (const [key, value] of Object.entries(patch)) {
    if (value === null || value === '') q.delete(key)
    else q.set(key, String(value))
  }
  const s = q.toString()
  return s ? `${path}?${s}` : path
}
