import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('organization')

/** แอปองค์กร — สาขา, หน่วยธุรกิจ, แผนก, ศูนย์ต้นทุน, โครงการ, ปีบัญชี, เลขที่เอกสาร, ข้อมูลบริษัท */
export default async function OrganizationLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="organization">
      {children}
    </AppShell>
  )
}
