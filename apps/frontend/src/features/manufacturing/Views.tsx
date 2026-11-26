import Link from 'next/link'
import { Badge } from '@/components'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import { ORDER_TONES, percent, type OrderMaterial, type OrderMovement, type ProductionOrder } from './types'

/** ส่วนแสดงผลใบสั่งผลิต (server) */

export function OrderStatus({ status }: { status: string }) {
  return (
    <Badge tone={ORDER_TONES[status] ?? 'neutral'} dot>
      {texts.manufacturing.statuses[status] ?? status}
    </Badge>
  )
}

const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted whitespace-nowrap'
const num = 'px-2.5 py-2.5 text-right tabular-nums whitespace-nowrap'

/** วัตถุดิบ: ต้องใช้ / เบิก / คืน / ใช้จริง / ต้นทุน + แถบความคืบหน้า (ใช้จริง ÷ ต้องใช้) */
export function MaterialsTable({ materials, showProgress }: { materials: OrderMaterial[]; showProgress: boolean }) {
  const t = texts.manufacturing.orders.materials
  return (
    <section aria-labelledby="mfg-materials" className="flex min-w-0 flex-col gap-3">
      <h2 id="mfg-materials" className="text-[15px] font-semibold">
        {t.title}
      </h2>
      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[36rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle">
              <th scope="col" className={th}>
                {t.item}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.required}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.issued}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.returned}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.net}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.cost}
              </th>
            </tr>
          </thead>
          <tbody>
            {materials.map((m) => {
              const pct = percent(m.net_quantity, m.required_quantity)
              return (
                <tr key={m.line_no} className="border-b border-line align-top last:border-b-0">
                  <td className="px-3 py-2.5">
                    <span className="text-[13px] text-ink-muted">{m.item.code}</span> {m.item.name}
                    {m.item.uom && <span className="ml-1 text-[12px] text-ink-muted">({m.item.uom})</span>}
                    {showProgress && (
                      <span className="mt-1.5 flex items-center gap-2">
                        <span
                          role="progressbar"
                          aria-valuemin={0}
                          aria-valuemax={100}
                          aria-valuenow={pct}
                          aria-label={format(t.progress, { pct })}
                          className="h-1.5 w-24 overflow-hidden rounded-full bg-neutral-soft"
                        >
                          <span className={`block h-full rounded-full ${pct >= 100 ? 'bg-success' : 'bg-primary'}`} style={{ width: `${pct}%` }} />
                        </span>
                        <span className="text-[12px] text-ink-muted tabular-nums">{pct}%</span>
                      </span>
                    )}
                  </td>
                  <td className={num}>{formatAmount(m.required_quantity)}</td>
                  <td className={num}>{formatAmount(m.issued_quantity, { dashZero: true })}</td>
                  <td className={num}>{formatAmount(m.returned_quantity, { dashZero: true })}</td>
                  <td className={`${num} font-medium`}>{formatAmount(m.net_quantity, { dashZero: true })}</td>
                  <td className={num}>{formatAmount(m.cost, { dashZero: true })}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </section>
  )
}

/** การ์ดต้นทุน: วัตถุดิบ + ค่าแปลงสภาพ = รวม, ต่อหน่วย */
export function CostCard({ order }: { order: ProductionOrder }) {
  const t = texts.manufacturing.orders.costs
  const c = order.costs ?? { materials: '0', conversion: order.conversion_cost ?? '0', total: '0', unit_cost: '0' }
  const rows: [string, string, boolean][] = [
    [t.materials, c.materials, false],
    [t.conversion, c.conversion, false],
    [t.total, c.total, true],
  ]
  return (
    <section aria-labelledby="mfg-costs" className="rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card">
      <h2 id="mfg-costs" className="text-[15px] font-semibold">
        {t.title}
      </h2>
      <dl className="mt-3 grid grid-cols-[1fr_auto] gap-y-1.5 text-sm">
        {rows.map(([label, value, strong]) => (
          <div key={label} className={`contents ${strong ? 'font-semibold' : ''}`}>
            <dt className={strong ? 'border-t border-line pt-2' : 'text-ink-muted'}>{label}</dt>
            <dd className={`text-right tabular-nums ${strong ? 'border-t border-line pt-2' : ''}`}>{formatAmount(value)}</dd>
          </div>
        ))}
        <dt className="text-ink-muted">{t.unit}</dt>
        <dd className="text-right tabular-nums">{order.status === 'completed' ? formatAmount(c.unit_cost) : '–'}</dd>
      </dl>
      <p className="mt-3 text-[12px] leading-relaxed text-ink-muted">{t.hint}</p>
    </section>
  )
}

/** ความเคลื่อนไหว: เบิก / คืน / รับสินค้า — ลิงก์เอกสารสต็อก (แอปคลัง) + เลขที่รายการบัญชี */
export function Movements({ companyId, movements }: { companyId: string; movements: OrderMovement[] }) {
  const t = texts.manufacturing.orders.movements
  return (
    <section aria-labelledby="mfg-moves" className="rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card">
      <h2 id="mfg-moves" className="text-[15px] font-semibold">
        {t.title}
      </h2>
      {movements.length === 0 ? (
        <p className="mt-3 text-sm text-ink-muted">{t.empty}</p>
      ) : (
        <ol className="mt-3 flex flex-col">
          {movements.map((m) => (
            <li key={m.id} className="relative border-l-2 border-line pb-4 pl-4 last:pb-0">
              <span aria-hidden className={`absolute top-1 -left-[5px] size-2 rounded-full ${m.kind === 'receipt' ? 'bg-success' : m.kind === 'return' ? 'bg-warning' : 'bg-primary'}`} />
              <p className="flex flex-wrap items-baseline justify-between gap-x-3 text-sm">
                <span className="font-medium">{t.kinds[m.kind] ?? m.kind}</span>
                <span className="tabular-nums">{formatAmount(m.value)}</span>
              </p>
              <p className="text-[12px] text-ink-muted">
                {formatDate(m.date)} ·{' '}
                <Link href={companyPath(companyId, 'inventory', 'documents', m.document_id)} className="font-mono text-primary hover:underline">
                  {m.document_no}
                </Link>
                {' · '}
                {m.entry_no ? format(t.entry, { no: m.entry_no }) : t.noEntry}
              </p>
            </li>
          ))}
        </ol>
      )}
      <p className="mt-3 text-[12px] leading-relaxed text-ink-muted">{t.periodicHint}</p>
    </section>
  )
}
