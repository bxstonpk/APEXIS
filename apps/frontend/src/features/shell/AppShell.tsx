import { Suspense, type ReactNode } from 'react'
import { ButtonLink } from '@/components'
import { appConfig, format, texts } from '@/config'
import { appAvailableFor, appTexts, companyPath, findApp, visibleApps, type AppDefinition } from '@/config/apps'
import { businessTypeName } from '@/config/businessTypes'
import { NavSkeleton, PageSkeleton } from '@/features/loading/PageSkeleton'
import { getCompanyContext } from '@/lib/server/company'
import { AppNav } from './AppNav'
import { CloseAppButton } from './CloseAppButton'
import { MessagePanel } from './MessagePanel'

/**
 * โครงหน้าของแอป (แท็บที่เปิดจากหน้ารวมแอป):
 * [✕ ปิด] [ไอคอน + ชื่อแอป] [เมนูของแอปตามสิทธิ์ ...] [ชื่อบริษัท]
 * ไม่มีสิทธิ์แอปนี้ / ไม่ใช่สมาชิกบริษัท / แอปไม่ใช้กับประเภทธุรกิจของบริษัท (เปิดจาก URL ตรง) → แสดงข้อความแทนเนื้อหา
 *
 * ระหว่างรอสิทธิ์ของผู้ใช้ (เปิดแอปในแท็บใหม่) แสดงแถบแอป + skeleton ทันที — ชื่อ / ไอคอนแอปรู้จาก appKey ไม่ต้องรอ backend
 * การเปลี่ยนหน้าภายในแอปใช้ loading.tsx ของแต่ละแอป (แถบแอปคงอยู่)
 */
export function AppShell({ companyId, appKey, children }: { companyId: string; appKey: string; children: ReactNode }) {
  const def = findApp(appKey)
  return (
    <Suspense
      fallback={
        <Frame companyId={companyId} app={def} nav={<NavSkeleton />}>
          <PageSkeleton />
        </Frame>
      }
    >
      <AppShellContent companyId={companyId} appKey={appKey}>
        {children}
      </AppShellContent>
    </Suspense>
  )
}

async function AppShellContent({ companyId, appKey, children }: { companyId: string; appKey: string; children: ReactNode }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) {
    return (
      <MessagePanel
        title={texts.companies.unavailable}
        action={<ButtonLink href={appConfig.routes.companies}>{texts.companies.heading}</ButtonLink>}
      />
    )
  }

  const launcher = companyPath(companyId)
  const def = findApp(appKey)
  const businessType = ctx.company.business_type
  if (def && !appAvailableFor(def, businessType)) {
    return (
      <MessagePanel
        title={format(texts.businessTypes.appUnavailable, { type: businessTypeName(businessType) })}        action={<ButtonLink href={launcher}>{texts.launcher.pageTitle}</ButtonLink>}
      />
    )
  }
  const app = def && visibleApps(ctx.permissions, businessType).find((a) => a.key === def.key)
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
  const items = app.menu.map((m) => ({ href: companyPath(companyId, app.key, m.path), label: t.menu[m.path] ?? m.path }))
  return (
    <Frame
      companyId={companyId}
      app={app}
      nav={<AppNav items={items} label={texts.shell.menu} />}
      company={<span className="ml-auto hidden max-w-60 shrink-0 truncate pl-4 text-[13px] text-white/60 lg:inline">{ctx.company.name}</span>}
    >
      {children}
    </Frame>
  )
}

/** แถบแอปสีเข้ม + พื้นที่เนื้อหา — ใช้ร่วมกันระหว่างหน้าจริงกับ skeleton เพื่อไม่ให้ layout กระโดด */
function Frame({ companyId, app, nav, company, children }: { companyId: string; app: AppDefinition | undefined; nav: ReactNode; company?: ReactNode; children: ReactNode }) {
  const Icon = app?.icon
  return (
    <div className="flex min-h-full flex-col">
      <header className="sticky top-0 z-20 bg-nav text-white print:hidden">
        <div className="mx-auto flex h-12 max-w-7xl items-center gap-1 px-2 sm:px-4">
          <CloseAppButton launcherHref={companyPath(companyId)} label={texts.shell.close} />
          <span aria-hidden className="mx-1.5 h-5 w-px bg-white/15" />
          {app && Icon && (
            <div className="flex shrink-0 items-center gap-2 pr-3">
              <span className={`flex size-6 items-center justify-center rounded-[5px] ${app.tone}`}>
                <Icon aria-hidden className="size-3.5" strokeWidth={2.25} />
              </span>
              <span className="text-[15px] font-semibold">{appTexts(app.key).name}</span>
            </div>
          )}
          {nav}
          {company}
        </div>
      </header>
      <main id="main" tabIndex={-1} className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 outline-none sm:py-8 print:max-w-none print:p-0">
        {children}
      </main>
    </div>
  )
}
