import type { ReactNode } from 'react'
import { ButtonLink } from '@/components'
import { appConfig, texts } from '@/config'
import { appTexts, companyPath, findApp, visibleApps } from '@/config/apps'
import { getCompanyContext } from '@/lib/server/company'
import { AppNav } from './AppNav'
import { CloseAppButton } from './CloseAppButton'
import { MessagePanel } from './MessagePanel'

/**
 * โครงหน้าของแอป (แท็บที่เปิดจากหน้ารวมแอป):
 * [✕ ปิด] [ไอคอน + ชื่อแอป] [เมนูของแอปตามสิทธิ์ ...] [ชื่อบริษัท]
 * ไม่มีสิทธิ์แอปนี้ / ไม่ใช่สมาชิกบริษัท → แสดงข้อความแทนเนื้อหา
 */
export async function AppShell({ companyId, appKey, children }: { companyId: string; appKey: string; children: ReactNode }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) {
    return (
      <MessagePanel
        title={texts.companies.unavailable}
        description={texts.companies.unavailableDescription}
        action={<ButtonLink href={appConfig.routes.companies}>{texts.companies.heading}</ButtonLink>}
      />
    )
  }

  const launcher = companyPath(companyId)
  const def = findApp(appKey)
  const app = def && visibleApps(ctx.permissions).find((a) => a.key === def.key)
  if (!def || !app) {
    return (
      <MessagePanel
        icon={def ? 'forbidden' : 'info'}
        title={def ? texts.common.noPermission : texts.common.notFound}
        action={<ButtonLink href={launcher}>{texts.launcher.pageTitle}</ButtonLink>}
      />
    )
  }

  const t = appTexts(app.key)
  const Icon = app.icon
  const items = app.menu.map((m) => ({ href: companyPath(companyId, app.key, m.path), label: t.menu[m.path] ?? m.path }))

  return (
    <div className="flex min-h-full flex-col">
      <header className="sticky top-0 z-20 bg-nav text-white print:hidden">
        <div className="mx-auto flex h-12 max-w-7xl items-center gap-1 px-2 sm:px-4">
          <CloseAppButton launcherHref={launcher} label={texts.shell.close} />
          <span aria-hidden className="mx-1.5 h-5 w-px bg-white/15" />
          <div className="flex shrink-0 items-center gap-2 pr-3">
            <span className={`flex size-6 items-center justify-center rounded-[5px] ${app.tone}`}>
              <Icon aria-hidden className="size-3.5" strokeWidth={2.25} />
            </span>
            <span className="text-[15px] font-semibold">{t.name}</span>
          </div>
          <AppNav items={items} label={texts.shell.menu} />
          <span className="ml-auto hidden max-w-60 shrink-0 truncate pl-4 text-[13px] text-white/60 lg:inline">{ctx.company.name}</span>
        </div>
      </header>
      <main id="main" tabIndex={-1} className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 outline-none sm:py-8 print:max-w-none print:p-0">
        {children}
      </main>
    </div>
  )
}
