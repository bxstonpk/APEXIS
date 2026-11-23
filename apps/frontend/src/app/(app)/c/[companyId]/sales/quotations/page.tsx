import type { Metadata } from 'next'
import { texts } from '@/config'
import { SalesDocList } from '@/features/sales/pages'
import { perms, salesApi, salesPage } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.quotations.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.quotations
  const path = salesPage(companyId, 'quotations')
  return (
    <SalesDocList
      companyId={companyId}
      sp={await searchParams}
      kind="quotation"
      path={path}
      apiPath={salesApi.quotations(companyId)}
      heading={t.heading}
      description={t.description}
      createHref={can(ctx.permissions, perms.quotationManage) && `${path}/new`}
    />
  )
}
