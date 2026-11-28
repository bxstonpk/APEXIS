import type { Metadata } from 'next'
import { texts } from '@/config'
import { DepositListPage } from '@/features/sales/depositPages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deposits.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <DepositListPage companyId={companyId} sp={await searchParams} />
}
