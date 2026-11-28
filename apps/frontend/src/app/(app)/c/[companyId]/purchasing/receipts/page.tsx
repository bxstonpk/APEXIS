import { redirectToDocuments } from '@/features/purchasing/pages'
import type { SearchParams } from '@/lib/url'

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการใบรับสินค้าย้ายไปรวมที่ "เอกสารซื้อ" (?type=receipt) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  redirectToDocuments(companyId, 'receipt', await searchParams)
}
