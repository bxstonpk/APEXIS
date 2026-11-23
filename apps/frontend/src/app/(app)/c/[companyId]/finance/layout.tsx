import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('finance')

/** แอปการเงิน — รับเงิน, จ่ายเงิน, โอนเงิน, บัญชีเงินสด / ธนาคาร, รายงานภาษี, ตั้งค่า */
export default async function FinanceLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="finance">
      {children}
    </AppShell>
  )
}
