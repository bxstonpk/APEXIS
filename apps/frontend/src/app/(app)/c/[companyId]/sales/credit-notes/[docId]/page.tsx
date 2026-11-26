import type { Metadata } from 'next'
import { texts } from '@/config'
import { CreditNoteViewPage } from '@/features/credit-notes/pages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.creditNotes.sales.pageTitle }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <CreditNoteViewPage mode="sales" companyId={companyId} docId={docId} sp={await searchParams} />
}
