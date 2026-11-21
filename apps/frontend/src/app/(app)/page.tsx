import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { appConfig } from '@/config'
import { companyPath } from '@/config/apps'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext, type CompanySummary } from '@/lib/server/company'

/**
 * หน้า / — พาไปที่บริษัทที่ควรเปิด:
 * 1. บริษัทที่เปิดล่าสุด (cookie) ถ้ายังเป็นสมาชิกอยู่
 * 2. เป็นสมาชิกบริษัทเดียว → บริษัทนั้น
 * 3. นอกนั้น → หน้าเลือกบริษัท (รวมกรณียังไม่มีบริษัท: สร้างได้จากหน้านั้น)
 */
export default async function RootPage() {
  const last = (await cookies()).get(appConfig.cookies.company)?.value
  if (last && (await getCompanyContext(last))) redirect(companyPath(last))

  const { data, meta } = await backendGet<CompanySummary[]>('/me/companies', new URLSearchParams({ page_size: '2' }))
  if (meta?.total === 1 && data[0]) redirect(companyPath(data[0].id))
  redirect(appConfig.routes.companies)
}
