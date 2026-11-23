import type { Metadata } from 'next'
import { Badge } from '@/components'
import { texts } from '@/config'
import { formatAmount, toString, toUnits } from '@/features/accounting/money'
import { StockDocForm } from '@/features/inventory/DocForm'
import { DOC_QUERY_KEYS, invApi, inventoryPage, invPerms, type StockDoc } from '@/features/inventory/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { dateText, Facts, LinesTable } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.documents.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ต้นทุนต่อหน่วยของบรรทัดขาออก (FIFO) = มูลค่า ÷ จำนวน — คำนวณแบบ BigInt สเกล 10^6 */
function unitCost(value: string, quantity: string): string {
  const q = toUnits(quantity)
  if (q === 0n) return '0'
  return toString((toUnits(value) * 1_000_000n) / (q < 0n ? -q : q))
}

const TONES: Record<string, 'success' | 'neutral' | 'warning'> = { draft: 'neutral', posted: 'success', cancelled: 'warning' }

/** เอกสารสต็อก — ร่าง: แก้ไข / ผ่านเอกสาร / ลบ; ผ่านแล้ว: ดู + ยกเลิก (กลับรายการสต็อกและบัญชี) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.documents
  const path = inventoryPage(companyId, 'documents')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<StockDoc>(invApi.documents(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const manage = can(ctx.permissions, invPerms.documentManage)
  const post = can(ctx.permissions, invPerms.documentPost)
  const actions: DocAction[] = []
  if (d.status === 'draft' && manage) actions.push({ kind: 'delete', label: t.actions.delete, confirm: t.confirm.delete! })
  if (d.status === 'posted' && post) actions.push({ kind: 'patch', status: 'cancelled', label: t.actions.cancel, confirm: t.confirm.cancel, danger: true })

  return (
    <FormPage
      crumb={{ app: texts.apps.inventory.name, label: t.heading, href: listHref }}
      title={d.document_no ?? t.draftTitle}
      description={t.types[d.type] ?? d.type}
      aside={
        <Badge tone={TONES[d.status] ?? 'neutral'} dot>
          {t.statuses[d.status] ?? d.status}
        </Badge>
      }
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${invApi.documents(companyId)}/${d.id}`} listHref={listHref} docNo={d.document_no} actions={actions} />
        {d.status === 'draft' && manage ? (
          <StockDocForm
            apiPath={invApi.documents(companyId)}
            itemsApi={invApi.items(companyId)}
            warehousesApi={invApi.warehouses(companyId)}
            docsPath={path}
            listHref={listHref}
            canPost={post}
            doc={d}
          />
        ) : (
          <>
            <Facts
              items={[
                [t.fields.type, t.types[d.type] ?? d.type],
                [t.fields.date, dateText(d.date)],
                [d.to_warehouse ? t.fields.fromWarehouse : t.fields.warehouse, `${d.warehouse.code} ${d.warehouse.name}`],
                ...(d.to_warehouse ? ([[t.fields.toWarehouse, `${d.to_warehouse.code} ${d.to_warehouse.name}`]] as [string, string][]) : []),
                [t.totalValue, formatAmount(d.total_value)],
                [t.fields.reference, d.reference],
                [t.fields.description, d.description],
              ]}
            />
            <LinesTable
              rows={d.lines.map((l) => ({
                key: l.line_no,
                item: l.item,
                description: [l.description, l.uom_code].filter(Boolean).join(' · '),
                quantity: l.quantity,
                unitPrice: l.unit_cost ?? unitCost(l.value, l.quantity),
                amount: l.value,
              }))}
            />
          </>
        )}
      </div>
    </FormPage>
  )
}
