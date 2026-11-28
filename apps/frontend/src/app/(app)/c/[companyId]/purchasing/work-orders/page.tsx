import type { Metadata } from 'next'
import { texts } from '@/config'
import { OrderList } from '@/features/purchasing/pages'
import { flows, purchasingPage } from '@/features/purchasing/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.workOrders.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const flow = flows.service
  const createHref = can(ctx.permissions, flow.perms.orderManage) && `${purchasingPage(companyId, flow.menu.orders)}/new`
  return <OrderList companyId={companyId} sp={await searchParams} createHref={createHref} flow={flow} />
}
