import type { Metadata } from 'next'
import { texts } from '@/config'
import { DepositViewPage } from '@/features/sales/depositPages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deposits.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <DepositViewPage companyId={companyId} docId={docId} sp={await searchParams} />
}
