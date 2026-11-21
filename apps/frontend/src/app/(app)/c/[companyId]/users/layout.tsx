import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

export const metadata = appMetadata('users')

/** แอปผู้ใช้และสิทธิ์ — สมาชิก, บทบาท */
export default async function UsersLayout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <AppShell companyId={companyId} appKey="users">
      {children}
    </AppShell>
  )
}
