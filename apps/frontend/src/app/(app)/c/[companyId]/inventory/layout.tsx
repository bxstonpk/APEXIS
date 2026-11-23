import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('inventory')

/** แอปคลังสินค้า — ภาพรวม (เมนูอื่นทยอยเปิด) */
export default async function InventoryLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="inventory">
      {children}
    </AppShell>
  )
}
