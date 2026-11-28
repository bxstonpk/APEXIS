import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('manufacturing')

/** แอปการผลิต — เฉพาะบริษัทประเภทธุรกิจผลิตสินค้า (AppShell แสดงข้อความแทนเมื่อประเภทไม่ตรง) */
export default async function ManufacturingLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="manufacturing">
      {children}
    </AppShell>
  )
}
