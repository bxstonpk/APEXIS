import type { Metadata } from 'next'
import { texts } from '@/config'
import { DepositNewPage } from '@/features/sales/depositPages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deposits.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <DepositNewPage companyId={companyId} orderId={param(await searchParams, 'order_id')} />
}
