import type { Metadata } from 'next'
import { texts } from '@/config'
import { OrderNewPage } from '@/features/purchasing/docPages'
import { flows } from '@/features/purchasing/types'

export const metadata: Metadata = { title: texts.purchasing.orders.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  return <OrderNewPage companyId={companyId} flow={flows.goods} />
}
