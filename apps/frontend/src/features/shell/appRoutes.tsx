import type { Metadata } from 'next'
import { redirect } from 'next/navigation'
import { texts } from '@/config'
import { appTexts, companyPath, findApp, visibleApps } from '@/config/apps'
import { getCompanyContext } from '@/lib/server/company'
import { MessagePanel } from './MessagePanel'

/**
 * ส่วนที่ทุกแอปใช้ร่วมกัน (เรียกจาก layout / page ของ /c/[companyId]/[app] และโฟลเดอร์ของแอปที่มีหน้าจริงแล้ว เช่น organization)
 */

/** ชื่อแท็บ: "<หน้า> · <แอป> | APEXIS" */
export function appMetadata(appKey: string): Metadata {
  const app = findApp(appKey)
  if (!app) return {}
  const name = appTexts(app.key).name
  return { title: { default: name, template: `%s · ${name} | ${texts.app.name}` } }
}

/** หน้าแรกของแอป = เมนูแรกที่มีสิทธิ์ (ไม่มีสิทธิ์ให้ AppShell แสดงข้อความแทน) */
export async function redirectToFirstMenu(companyId: string, appKey: string): Promise<null> {
  const ctx = await getCompanyContext(companyId)
  const app = ctx && visibleApps(ctx.permissions).find((a) => a.key === appKey)
  if (app?.menu[0]) redirect(companyPath(companyId, app.key, app.menu[0].path))
  return null
}

/** เมนูที่ยังไม่มีหน้าจริง */
export function ComingSoon() {
  return <MessagePanel title={texts.common.comingSoon} description={texts.common.comingSoonDescription} />
}
