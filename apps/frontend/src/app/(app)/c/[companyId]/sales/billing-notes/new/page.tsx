import type { Metadata } from 'next'
import { texts } from '@/config'
import { BillingNoteNewPage } from '@/features/sales/billingPages'

export const metadata: Metadata = { title: texts.sales.billingNotes.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  return <BillingNoteNewPage companyId={companyId} />
}
