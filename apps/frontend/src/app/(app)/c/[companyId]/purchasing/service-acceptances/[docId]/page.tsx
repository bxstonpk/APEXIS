import type { Metadata } from 'next'
import { texts } from '@/config'
import { ReceiptViewPage } from '@/features/purchasing/docPages'
import { flows } from '@/features/purchasing/types'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.acceptances.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <ReceiptViewPage companyId={companyId} docId={docId} sp={searchParams} flow={flows.service} />
}
