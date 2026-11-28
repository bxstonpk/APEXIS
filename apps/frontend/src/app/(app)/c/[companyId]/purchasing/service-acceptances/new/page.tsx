import type { Metadata } from 'next'
import { texts } from '@/config'
import { PurchaseFulfilPage } from '@/features/purchasing/pages'
import { flows } from '@/features/purchasing/types'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.acceptances.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <PurchaseFulfilPage companyId={companyId} orderId={param(await searchParams, 'order_id')} mode="receipt" flow={flows.service} />
}
