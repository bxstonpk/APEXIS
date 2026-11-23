import type { Metadata } from 'next'
import { Suspense } from 'react'
import { ButtonLink } from '@/components'
import { appConfig, format, texts } from '@/config'
import { appTexts, appWindowName, companyPath, visibleApps } from '@/config/apps'
import { LauncherSkeleton } from '@/features/loading/PageSkeleton'
import { AppTile, MessagePanel, TopBar } from '@/features/shell'
import { currentUser } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.launcher.pageTitle }

/**
 * หน้ารวมแอปของบริษัท — ไอคอนแอปกลางจอ (เฉพาะแอปที่มีสิทธิ์)
 * กดไอคอน = เปิดแอปในแท็บใหม่ที่เมนูแรกที่มีสิทธิ์; ในแท็บนั้นกด ✕ เพื่อปิดกลับมาที่นี่
 * แถบบนแสดงทันที ส่วนไอคอนแอปรอสิทธิ์จาก backend (skeleton ระหว่างรอ)
 */
export default async function LauncherPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const user = await currentUser() // จาก cookie — ไม่เรียก backend
  return (
    <Suspense
      fallback={
        <div className="flex min-h-full flex-col">
          <TopBar userName={user.name} />
          <main id="main" tabIndex={-1} className="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center px-4 pt-16 pb-16 sm:pt-24">
            <LauncherSkeleton />
          </main>
        </div>
      }
    >
      <Launcher companyId={companyId} user={user} />
    </Suspense>
  )
}

/** ส่วนที่ต้องรอ backend (บริษัท + สิทธิ์) — ระหว่างรอแสดง skeleton ของไอคอนแอป */
async function Launcher({ companyId, user }: { companyId: string; user: { name: string } }) {
  const ctx = await getCompanyContext(companyId)

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
  const today = new Intl.DateTimeFormat('th-TH', { dateStyle: 'full' }).format(new Date())
  return (
    <div className="flex min-h-full flex-col bg-[radial-gradient(70rem_28rem_at_50%_-6rem,var(--color-primary-soft),transparent)]">
      <TopBar userName={user.name} companyName={ctx.company.name || ctx.company.code} />
      <main id="main" tabIndex={-1} className="mx-auto flex w-full max-w-3xl flex-1 flex-col items-center px-4 pt-16 pb-16 sm:pt-24">
        <p className="text-[13px] text-ink-muted">{today}</p>
        <h1 className="mt-1 text-center text-2xl font-semibold tracking-tight">{format(texts.launcher.greeting, { name: user.name })}</h1>
        {available.length === 0 ? (
          <p className="mt-10 max-w-md text-center text-ink-muted">{texts.launcher.empty}</p>
        ) : (
          <ul className="mt-12 grid grid-cols-3 gap-x-2 gap-y-6 sm:grid-cols-4 sm:gap-x-6 sm:gap-y-8">
            {available.map((app) => {
              const name = appTexts(app.key).name
              const Icon = app.icon
              return (
                <li key={app.key} className="flex justify-center">
                  <AppTile
                    href={companyPath(companyId, app.key, app.menu[0]!.path)}
                    windowName={appWindowName(companyId, app.key)}
                    label={name}
                    title={format(texts.launcher.openHint, { name })}
                    tone={app.tone}
                    icon={<Icon aria-hidden className="size-8 sm:size-9" strokeWidth={1.6} />}
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
