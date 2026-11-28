import type { Metadata } from 'next'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PcActions, type PcAction } from '@/features/petty-cash/Actions'
import { pcApi, pcPage, pcPerms, VOUCHER_QUERY_KEYS, type Voucher } from '@/features/petty-cash/types'
import { TotalsCard, VoucherStatus } from '@/features/petty-cash/Views'
import { VoucherForm } from '@/features/petty-cash/VoucherForm'
import { dateText, Facts } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.vouchers.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** ใบสำคัญจ่าย — ร่าง: แก้ไข / อนุมัติ / ยกเลิก; อนุมัติแล้ว (ยังไม่อยู่ในคำขอ): ยกเลิก; อื่น ๆ: ดูอย่างเดียว */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.vouchers
  const f = t.fields
  const path = pcPage(companyId, 'vouchers')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), VOUCHER_QUERY_KEYS)
  const res = await loadItem<Voucher>(pcApi.vouchers(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const v = res.item
  const manage = can(ctx.permissions, pcPerms.voucherManage)
  const editing = v.status === 'draft' && manage
  const actions: PcAction[] = []
  if (manage && (v.status === 'draft' || (v.status === 'approved' && !v.replenishment))) {
    actions.push({ path: 'cancel', label: t.actions.cancel, confirm: t.confirm.cancel, danger: true })
  }
  if (v.status === 'draft' && can(ctx.permissions, pcPerms.voucherApprove)) {
    actions.push({ path: 'approve', label: t.actions.approve, confirm: t.confirm.approve, primary: true })
  }
  const hasVat = toUnits(v.vat_rate) > 0n
  return (
    <FormPage
      crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: listHref }}
      title={v.document_no}
      description={v.payee.name}
      aside={<VoucherStatus status={v.status} />}
    >
      <div className="flex flex-col gap-6">
        <PcActions apiPath={`${pcApi.vouchers(companyId)}/${v.id}`} docNo={v.document_no} actions={actions} printHref={`${path}/${v.id}/print`} />
        {editing ? (
          <VoucherForm
            apiPath={pcApi.vouchers(companyId)}
            docsPath={path}
            listHref={listHref}
            fundsApi={pcApi.funds(companyId)}
            accountsApi={pcApi.accounts(companyId)}
            vatRate="7"
            voucher={v}
          />
        ) : (
          <>
            <Facts
              items={[
                [f.date, dateText(v.date)],
                [f.fund, `${v.fund.code} ${v.fund.name}`],
                [f.payee, [v.payee.name, v.payee.tax_id, v.payee.branch_no].filter(Boolean).join(' · ')],
                ...(hasVat && v.tax_invoice
                  ? ([
                      [f.vatRate, `${formatAmount(v.vat_rate)}%`],
                      [f.taxInvoiceNo, `${v.tax_invoice.no} · ${dateText(v.tax_invoice.date)}`],
                    ] as [string, string][])
                  : []),
                [f.description, v.description],
                [f.createdBy, v.created_by.name],
                ...(v.approved_by ? ([[f.approvedBy, `${v.approved_by.name} · ${dateText(v.approved_at)}`]] as [string, string][]) : []),
                ...(v.replenishment
                  ? ([
                      [
                        f.request,
                        <Link key="r" href={`${pcPage(companyId, 'replenishments')}/${v.replenishment.id}`} className="font-mono text-[13px] text-primary hover:underline">
                          {v.replenishment.document_no}
                        </Link>,
                      ],
                    ] as [string, ReactNode][])
                  : []),
              ]}
            />
            <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_18rem]">
              <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
                <table className="w-full min-w-[32rem] border-collapse text-sm">
                  <thead className="text-left text-[13px] text-ink-muted">
                    <tr className="border-b border-line bg-subtle">
                      <th scope="col" className="w-12 px-3 py-2 text-center">#</th>
                      <th scope="col" className="px-3 py-2">{f.account}</th>
                      <th scope="col" className="px-3 py-2">{f.lineDescription}</th>
                      <th scope="col" className="px-3 py-2 text-right">{f.amount}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {v.lines.map((l) => (
                      <tr key={l.line_no} className="border-b border-line align-top last:border-0">
                        <td className="px-3 py-2 text-center tabular-nums text-ink-muted">{l.line_no}</td>
                        <td className="px-3 py-2">
                          <span className="font-mono text-[13px] text-ink-muted">{l.account.code}</span> {l.account.name}
                        </td>
                        <td className="px-3 py-2">{l.description}</td>
                        <td className="px-3 py-2 text-right tabular-nums">{formatAmount(l.amount)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <TotalsCard totals={v.totals} />
            </div>
          </>
        )}
      </div>
    </FormPage>
  )
}
