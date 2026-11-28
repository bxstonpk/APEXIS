import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('petty-cash')

/** แอปเงินสดย่อย — ทุกประเภทธุรกิจ */
export default async function PettyCashLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="petty-cash">
      {children}
    </AppShell>
  )
}
