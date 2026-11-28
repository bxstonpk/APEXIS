import type { Metadata } from 'next'
import { texts } from '@/config'
import { BillingNotePrintPage } from '@/features/sales/billingPages'

export const metadata: Metadata = { title: texts.sales.print.billingNote }

export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  return <BillingNotePrintPage companyId={companyId} docId={docId} />
}
