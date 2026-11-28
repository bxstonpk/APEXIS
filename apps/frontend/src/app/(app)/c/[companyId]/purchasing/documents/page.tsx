import type { Metadata } from 'next'
import { texts } from '@/config'
import { PurchaseDocumentList } from '@/features/purchasing/pages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.documents.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** เอกสารซื้อ — รายการรวมใบสั่งซื้อ / ใบรับสินค้า / ซื้อสินค้า / ใบสั่งจ้าง / ตรวจรับงาน / ซื้อบริการ (?type=) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <PurchaseDocumentList companyId={companyId} sp={await searchParams} />
}
