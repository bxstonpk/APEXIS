import type { ReactNode } from 'react'
import { ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { companyPath, menuAvailableFor } from '@/config/apps'
import { businessTypeName } from '@/config/businessTypes'
import { getCompanyContext } from '@/lib/server/company'
import { MessagePanel } from './MessagePanel'

/**
 * กันเมนูที่ไม่ใช้กับประเภทธุรกิจของบริษัท (เปิดจาก URL ตรง) — ใช้ใน layout.tsx ของเมนูนั้น
 * เช่น บริษัทบริการเปิด /sales/invoices (ขายสินค้า) → แสดงข้อความพร้อมปุ่มกลับหน้าแรกของแอป
 */
export async function MenuGuard({ companyId, appKey, menu, children }: { companyId: string; appKey: string; menu: string; children: ReactNode }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const bt = ctx.company.business_type
  if (menuAvailableFor(appKey, menu, bt)) return <>{children}</>
  return (
    <MessagePanel
      title={format(texts.businessTypes.menuUnavailable, { type: businessTypeName(bt) })}      action={<ButtonLink href={companyPath(companyId, appKey)}>{texts.businessTypes.backToApp}</ButtonLink>}
    />
  )
}
