import type { ReactNode } from 'react'
import { MenuGuard } from '@/features/shell/MenuGuard'

/** ใบแจ้งหนี้ค่าบริการ ใช้เฉพาะธุรกิจบริการ / ก่อสร้าง — เปิดจาก URL ตรงในประเภทอื่นแสดงข้อความแทน */
export default async function Layout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <MenuGuard companyId={companyId} appKey="sales" menu="service-invoices">
      {children}
    </MenuGuard>
  )
}
