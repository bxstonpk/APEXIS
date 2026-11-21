import type { Metadata } from 'next'
import { ButtonLink } from '@/components'
import { appConfig, format, texts } from '@/config'
import { appTexts, appWindowName, companyPath, visibleApps } from '@/config/apps'
import { AppTile, MessagePanel, TopBar } from '@/features/shell'
import { currentUser } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.launcher.pageTitle }

/**
 * หน้ารวมแอปของบริษัท — ไอคอนแอปกลางจอ (เฉพาะแอปที่มีสิทธิ์)
 * กดไอคอน = เปิดแอปในแท็บใหม่ที่เมนูแรกที่มีสิทธิ์; ในแท็บนั้นกด ✕ เพื่อปิดกลับมาที่นี่
 */
export default async function LauncherPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const [user, ctx] = await Promise.all([currentUser(), getCompanyContext(companyId)])

  if (!ctx) {
    return (
      <div className="flex min-h-full flex-col">
        <TopBar userName={user.name} />
        <MessagePanel
          title={texts.companies.unavailable}
          description={texts.companies.unavailableDescription}
          action={<ButtonLink href={appConfig.routes.companies}>{texts.companies.heading}</ButtonLink>}
        />
      </div>
    )
  }

  const available = visibleApps(ctx.permissions)
  return (
    <div className="flex min-h-full flex-col">
      <TopBar userName={user.name} companyName={ctx.company.name || ctx.company.code} />
      <main className="flex flex-1 items-center justify-center px-4 py-10">
        {available.length === 0 ? (
          <p className="max-w-md text-center text-ink-muted">{texts.launcher.empty}</p>
        ) : (
          <ul className="grid grid-cols-3 gap-x-4 gap-y-8 sm:grid-cols-4 sm:gap-x-10">
            {available.map((app) => {
              const name = appTexts(app.key).name
              const Icon = app.icon
              return (
                <li key={app.key}>
                  <AppTile
                    href={companyPath(companyId, app.key, app.menu[0]!.path)}
                    windowName={appWindowName(companyId, app.key)}
                    label={name}
                    title={format(texts.launcher.openHint, { name })}
                    tone={app.tone}
                    icon={<Icon aria-hidden className="size-8 sm:size-10" strokeWidth={1.75} />}
                  />
                </li>
              )
            })}
          </ul>
        )}
      </main>
    </div>
  )
}
