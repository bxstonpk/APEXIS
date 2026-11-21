/**
 * ส่งออก CSV (pure) — เปิดใน Excel ได้ภาษาไทยถูกต้อง (มี BOM ของ UTF-8 นำหน้า)
 * ทุกช่องครอบด้วย " และ escape " ภายใน; ช่องที่ขึ้นต้นด้วย = + - @ ถูกเติม ' กันสูตร (CSV injection)
 * ยกเว้นตัวเลขติดลบที่เป็นตัวเลขล้วน
 */
export type CsvCell = string | number | null | undefined

const NUMBER = /^-?\d+(\.\d+)?$/

function cell(v: CsvCell): string {
  let s = v === null || v === undefined ? '' : String(v)
  if (/^[=+\-@\t\r]/.test(s) && !NUMBER.test(s)) s = `'${s}`
  return `"${s.replace(/"/g, '""')}"`
}

export function toCsv(rows: readonly (readonly CsvCell[])[]): string {
  return '﻿' + rows.map((r) => r.map(cell).join(',')).join('\r\n') + '\r\n'
}

/** ดาวน์โหลดไฟล์ CSV ใน browser */
export function downloadCsv(filename: string, rows: readonly (readonly CsvCell[])[]): void {
  const url = URL.createObjectURL(new Blob([toCsv(rows)], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
