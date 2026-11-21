import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { AppShell } from '@/features/shell'
import { appMetadata } from '@/features/shell/appRoutes'

type Params = Promise<{ companyId: string; app: string }>

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  return appMetadata((await params).app)
}

/** แท็บของแอปที่ยังไม่มีโฟลเดอร์ของตัวเอง — แอปที่มีหน้าจริงแล้วมีโฟลเดอร์ชื่อแอป (เช่น organization/) ซึ่ง Next เลือกก่อน */
export default async function AppLayout({ params, children }: { params: Params; children: ReactNode }) {
  const { companyId, app } = await params
  return (
    <AppShell companyId={companyId} appKey={app}>
      {children}
    </AppShell>
  )
}
