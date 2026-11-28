import type { Metadata } from 'next'
import { texts } from '@/config'
import { DepositPrintPage } from '@/features/sales/depositPages'

export const metadata: Metadata = { title: texts.sales.print.deposit }

export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  return <DepositPrintPage companyId={companyId} docId={docId} />
}
