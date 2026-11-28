import type { Metadata } from 'next'
import Link from 'next/link'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { RequestForm } from '@/features/petty-cash/RequestForm'
import { pcApi, pcPage, pcPerms, type Fund, type RequestKind } from '@/features/petty-cash/types'
import { cn } from '@/lib/utils'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.requests.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const KINDS: RequestKind[] = ['replenish', 'increase', 'establish']

/** สร้างคำขอ — ประเภทและกองทุนตั้งต้นจาก URL (?kind=&fund_id=) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, pcPerms.requestPost)) return <NoPermission />
  const t = texts.pettyCash.requests
  const path = pcPage(companyId, 'replenishments')
  const raw = param(sp, 'kind')
  const kind: RequestKind = KINDS.includes(raw as RequestKind) ? (raw as RequestKind) : 'replenish'
  const fundId = param(sp, 'fund_id')
  const fund = fundId
    ? await backendGet<Fund>(`${pcApi.funds(companyId)}/${encodeURIComponent(fundId)}`).then(
        (r) => r.data,
        () => null,
      )
    : null
  const initialFund = fund ? { id: fund.id, code: fund.code, name: fund.name, imprest_amount: fund.imprest_amount } : null
  const kindHref = (k: RequestKind) => `${path}/new?kind=${k}${fundId ? `&fund_id=${encodeURIComponent(fundId)}` : ''}`
  return (
    <FormPage crumb={{ app: texts.apps['petty-cash'].name, label: t.heading, href: path }} title={t.newTitle}>
      <div className="flex flex-col gap-6">
        <nav aria-label={t.fields.kind} className="flex w-fit flex-wrap gap-1 rounded-(--radius-panel) border border-line bg-surface p-1 shadow-card">
          {KINDS.map((k) => (
            <Link
              key={k}
              href={kindHref(k)}
              aria-current={k === kind ? 'page' : undefined}
              className={cn(
                'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                k === kind ? 'bg-primary text-white' : 'text-ink-muted hover:bg-subtle hover:text-ink',
              )}
            >
              {texts.pettyCash.requestKinds[k]}
            </Link>
          ))}
        </nav>
        <RequestForm
          key={`${kind}-${fundId}`}
          apiPath={pcApi.requests(companyId)}
          docsPath={path}
          listHref={path}
          fundsApi={pcApi.funds(companyId)}
          vouchersApi={pcApi.vouchers(companyId)}
          kind={kind}
          initialFund={initialFund}
        />
      </div>
    </FormPage>
  )
}
