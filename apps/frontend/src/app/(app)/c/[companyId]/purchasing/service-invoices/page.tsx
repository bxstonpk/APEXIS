import type { Metadata } from 'next'
import { texts } from '@/config'
import { InvoiceList } from '@/features/purchasing/pages'
import { flows, purchasingPage } from '@/features/purchasing/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.serviceInvoices.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ซื้อบริการ — บันทึกจากใบสั่งจ้างที่ตรวจรับงานแล้ว หรือซื้อตรง (ปุ่มสร้าง) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const flow = flows.service
  const createHref = can(ctx.permissions, flow.perms.invoicePost) && `${purchasingPage(companyId, flow.menu.invoices)}/new`
  return <InvoiceList companyId={companyId} sp={await searchParams} flow={flow} createHref={createHref} />
}
