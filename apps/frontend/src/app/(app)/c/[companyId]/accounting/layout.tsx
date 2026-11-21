import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('accounting')

/** แอปบัญชี — รายการบัญชี, ผังบัญชี, สมุดรายวัน, รายงาน */
export default async function AccountingLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="accounting">
      {children}
    </AppShell>
  )
}
