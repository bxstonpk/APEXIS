import type { Metadata } from 'next'
import { texts } from '@/config'
import { BillingNoteViewPage } from '@/features/sales/billingPages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.billingNotes.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <BillingNoteViewPage companyId={companyId} docId={docId} sp={await searchParams} />
}
