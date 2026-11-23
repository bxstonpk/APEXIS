import type { ReactNode } from 'react'
import { texts } from '@/config'

/** ตารางรายงาน (พิมพ์ได้) — หัวคอลัมน์ข้อความ + ตัวเลขชิดขวา, ไม่มีแถว = ข้อความ, แถวรวมท้ายตาราง */
const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted print:py-1'

export function ReportTable({ minWidth, head, numHead, empty, foot, children }: { minWidth: string; head: string[]; numHead: string[]; empty: boolean; foot?: ReactNode; children: ReactNode }) {
  return (
    <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card print:border-0 print:shadow-none">
      <table className="w-full border-collapse text-sm print:text-[11px]" style={{ minWidth }}>
        <thead>
          <tr className="border-b border-line bg-subtle print:bg-transparent">
            {head.map((h) => (
              <th key={h} scope="col" className={th}>
                {h}
              </th>
            ))}
            {numHead.map((h) => (
              <th key={h} scope="col" className={`${th} text-right`}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {empty ? (
            <tr>
              <td colSpan={head.length + numHead.length} className="px-3 py-10 text-center text-ink-muted">
                {texts.sales.reports.noRows}
              </td>
            </tr>
          ) : (
            children
          )}
        </tbody>
        {foot && !empty && <tfoot>{foot}</tfoot>}
      </table>
    </div>
  )
}
