import type { Metadata } from 'next'
import { texts } from '@/config'
import { SalesDocList } from '@/features/sales/pages'
import { perms, salesApi, salesPage } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.orders.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.orders
  const path = salesPage(companyId, 'orders')
  return (
    <SalesDocList
      companyId={companyId}
      sp={await searchParams}
      kind="order"
      path={path}
      apiPath={salesApi.orders(companyId)}
      heading={t.heading}      createHref={can(ctx.permissions, perms.orderManage) && `${path}/new`}
    />
  )
}
