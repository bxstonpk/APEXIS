import type { Metadata } from 'next'
import { texts } from '@/config'
import { BillingNoteListPage } from '@/features/sales/billingPages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.billingNotes.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <BillingNoteListPage companyId={companyId} sp={await searchParams} />
}
