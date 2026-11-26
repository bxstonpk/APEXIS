import type { Metadata } from 'next'
import { texts } from '@/config'
import { CreditNoteNewPage } from '@/features/credit-notes/pages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.creditNotes.purchasing.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <CreditNoteNewPage mode="purchasing" companyId={companyId} invoiceId={param(await searchParams, 'invoice_id')} />
}
