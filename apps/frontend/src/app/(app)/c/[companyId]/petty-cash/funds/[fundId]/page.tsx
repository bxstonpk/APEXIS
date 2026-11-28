import type { Metadata } from 'next'
import { Badge, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { FormPage, loadItem } from '@/features/organization/pages'
import { FundForm } from '@/features/petty-cash/FundForm'
import { loadCustodians } from '@/features/petty-cash/server'
import { FUND_QUERY_KEYS, pcApi, pcPage, pcPerms, type Fund } from '@/features/petty-cash/types'
import { Facts } from '@/features/sales/Views'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.funds.pageTitle }

type Props = { params: Promise<{ companyId: string; fundId: string }>; searchParams: Promise<SearchParams> }

/** กองทุน — ยอดกองทุน + ปุ่มขอตั้งวงเงิน / เพิ่มวงเงิน / เบิกชดเชย + ฟอร์มแก้ไข */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, fundId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.funds
  const f = t.fields
  const listHref = listHrefFromBack(pcPage(companyId, 'funds'), param(await searchParams, 'back'), FUND_QUERY_KEYS)
  const res = await loadItem<Fund>(pcApi.funds(companyId), fundId, t.notFound)
  if ('panel' in res) return res.panel
  const x = res.item
  const canManage = can(ctx.permissions, pcPerms.fundManage)
  const canRequest = can(ctx.permissions, pcPerms.requestPost) && x.status === 'active'
  const newRequest = (kind: string) => `${pcPage(companyId, 'replenishments')}/new?kind=${kind}&fund_id=${x.id}`
  return (
    <FormPage crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: x.code })}
      description={x.name}
      aside={x.funded ? undefined : <Badge tone="warning">{f.notFunded}</Badge>}
    >
      <div className="flex flex-col gap-6">
        {canRequest && (
          <div className="flex flex-wrap items-center gap-2 rounded-(--radius-panel) border border-line bg-surface px-4 py-3 shadow-card">
            <div className="ml-auto flex flex-wrap gap-2">
              {x.funded ? (
                <>
                  <ButtonLink href={newRequest('increase')} variant="secondary">
                    {t.actions.increase}
                  </ButtonLink>
                  <ButtonLink href={newRequest('replenish')}>{t.actions.replenish}</ButtonLink>
                </>
              ) : (
                <ButtonLink href={newRequest('establish')}>{t.actions.establish}</ButtonLink>
              )}
            </div>
          </div>
        )}
        <Facts
          items={[
            [f.imprest, <span key="i" className="tabular-nums">{formatAmount(x.imprest_amount)}</span>],
            [f.cashOnHand, <span key="c" className="font-semibold tabular-nums">{formatAmount(x.cash_on_hand)}</span>],
            [f.pending, <span key="p" className="tabular-nums">{formatAmount(x.pending_replenishment)}</span>],
            [f.requested, <span key="r" className="tabular-nums">{formatAmount(x.requested)}</span>],
          ]}
        />
        <FundForm
          apiPath={pcApi.funds(companyId)}
          listHref={listHref}
          accountsApi={pcApi.accounts(companyId)}
          members={canManage ? await loadCustodians(companyId) : []}
          canManage={canManage}
          fund={x}
        />
      </div>
    </FormPage>
  )
}
