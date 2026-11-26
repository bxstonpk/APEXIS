import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('sales')

/** แอปขาย — ใบเสนอราคา → ใบสั่งขาย → ขายสินค้า / ใบแจ้งหนี้ (ตามประเภทธุรกิจ), มัดจำ, ใบวางบิล, ใบลดหนี้, ลูกค้า, รายงาน, ตั้งค่า (รับเงินที่แอปการเงิน) */
export default async function SalesLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="sales">
      {children}
    </AppShell>
  )
}
