const dateFormat = new Intl.DateTimeFormat('th-TH', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })

/** วันที่จาก API (YYYY-MM-DD) เป็นรูปแบบไทย เช่น "1 ม.ค. 2570" — ไม่มีค่า = "" */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(`${iso.slice(0, 10)}T00:00:00Z`)
  return Number.isNaN(d.getTime()) ? iso : dateFormat.format(d)
}

/** วันนี้ในรูปแบบ YYYY-MM-DD (ตามเวลาเครื่อง) */
export function today(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
