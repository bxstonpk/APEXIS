import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('purchasing')

/** แอปจัดซื้อ — ใบสั่งซื้อ → รับสินค้า → ใบแจ้งหนี้ซื้อ, ผู้ขาย, รายงาน, ตั้งค่า (จ่ายเงินที่แอปการเงิน) */
export default async function PurchasingLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="purchasing">
      {children}
    </AppShell>
  )
}
