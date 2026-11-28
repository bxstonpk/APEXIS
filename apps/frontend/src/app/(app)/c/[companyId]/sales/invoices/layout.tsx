import type { ReactNode } from 'react'
import { MenuGuard } from '@/features/shell/MenuGuard'

/** ขายสินค้า ใช้เฉพาะธุรกิจซื้อขาย / ผลิต — เปิดจาก URL ตรงในประเภทอื่นแสดงข้อความแทน */
export default async function Layout({ params, children }: { params: Promise<{ companyId: string }>; children: ReactNode }) {
  const { companyId } = await params
  return (
    <MenuGuard companyId={companyId} appKey="sales" menu="invoices">
      {children}
    </MenuGuard>
  )
}
