import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PcActions, type PcAction } from '@/features/petty-cash/Actions'
import { pcApi, pcPage, pcPerms, REQUEST_QUERY_KEYS, type Replenishment } from '@/features/petty-cash/types'
import { RequestStatus } from '@/features/petty-cash/Views'
import { VoucherTable } from '@/features/petty-cash/VoucherTable'
import { dateText, Facts } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.requests.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** คำขอ — รอการเงินจ่าย: ยกเลิกได้; จ่ายแล้ว: แสดงเลขที่เอกสารจ่ายของการเงิน (ไม่ลิงก์ข้ามแอป) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.requests
  const f = t.fields
  const path = pcPage(companyId, 'replenishments')
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), REQUEST_QUERY_KEYS)
  const res = await loadItem<Replenishment>(pcApi.requests(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const r = res.item
  const actions: PcAction[] =
    r.status === 'requested' && can(ctx.permissions, pcPerms.requestPost) ? [{ path: 'cancel', label: t.actions.cancel, confirm: t.confirm.cancel, danger: true }] : []
  const vouchersPath = pcPage(companyId, 'vouchers')
  return (
    <FormPage
      crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: listHref }}
      title={r.document_no}
      description={`${texts.pettyCash.requestKinds[r.kind] ?? r.kind} · ${r.fund.code} ${r.fund.name}`}
      aside={<RequestStatus status={r.status} />}
    >
      <div className="flex flex-col gap-6">
        <PcActions apiPath={`${pcApi.requests(companyId)}/${r.id}`} docNo={r.document_no} actions={actions} printHref={`${path}/${r.id}/print`} />
        <Facts
          items={[
            [f.date, dateText(r.date)],
            [f.kind, texts.pettyCash.requestKinds[r.kind] ?? r.kind],
            [f.fund, `${r.fund.code} ${r.fund.name}`],
            [f.amount, <span key="a" className="font-semibold tabular-nums">{formatAmount(r.amount)}</span>],
            [f.description, r.description],
            ...(r.payment
              ? ([
                  [f.payment, <span key="p" className="font-mono text-[13px]">{r.payment.document_no}</span>],
                  [f.paidDate, dateText(r.payment.date)],
                ] as [string, ReactNode][])
              : []),
          ]}
        />
        {r.kind === 'replenish' && <VoucherTable rows={r.vouchers} href={(id) => `${vouchersPath}/${id}`} total={r.amount} />}
      </div>
    </FormPage>
  )
}
