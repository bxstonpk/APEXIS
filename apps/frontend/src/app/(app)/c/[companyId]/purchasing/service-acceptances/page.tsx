import type { Metadata } from 'next'
import { texts } from '@/config'
import { ReceiptList } from '@/features/purchasing/pages'
import { flows } from '@/features/purchasing/types'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.acceptances.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบตรวจรับงาน — สร้างจากหน้าใบสั่งจ้างที่อนุมัติแล้ว (ปุ่ม "ตรวจรับงาน") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  return <ReceiptList companyId={companyId} sp={await searchParams} flow={flows.service} />
}
