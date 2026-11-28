import { redirectToDocuments } from '@/features/purchasing/pages'
import type { SearchParams } from '@/lib/url'

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการใบสั่งจ้างย้ายไปรวมที่ "เอกสารซื้อ" (?type=work_order) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  redirectToDocuments(companyId, 'work_order', await searchParams)
}
