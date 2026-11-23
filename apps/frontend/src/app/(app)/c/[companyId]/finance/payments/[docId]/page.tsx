import type { Metadata } from 'next'
import { Badge } from '@/components'
import { texts } from '@/config'
import { companyPath } from '@/config/apps'
import { MoneyDocView } from '@/features/finance/DocView'
import { DOC_QUERY_KEYS, financeApi, financePage, perms, type Payment } from '@/features/finance/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.payments.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance
  const listHref = listHrefFromBack(financePage(companyId, 'payments'), param(await searchParams, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Payment>(financeApi.payments(companyId), docId, texts.finance.payments.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const entryHref = d.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${d.entry_id}` : null
  return (
    <FormPage
      crumb={{ app: texts.apps.finance.name, label: texts.finance.payments.heading, href: listHref }}
      title={d.document_no}
      description={d.party.name}
      aside={<Badge tone={d.status === 'posted' ? 'success' : 'neutral'} dot>{t.statuses[d.status] ?? d.status}</Badge>}
    >
      <MoneyDocView
        mode="payment"
        doc={d}
        apiPath={`${financeApi.payments(companyId)}/${d.id}`}
        listHref={listHref}
        entryHref={entryHref}
        canCancel={can(ctx.permissions, perms.paymentPost)}
      />
    </FormPage>
  )
}
