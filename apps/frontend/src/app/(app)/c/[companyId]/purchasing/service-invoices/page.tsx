import { redirectToDocuments } from '@/features/purchasing/pages'
import type { SearchParams } from '@/lib/url'

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการซื้อบริการย้ายไปรวมที่ "เอกสารซื้อ" (?type=service_invoice) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  redirectToDocuments(companyId, 'service_invoice', await searchParams)
}
