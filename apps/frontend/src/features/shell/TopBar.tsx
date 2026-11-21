import { Building2, ChevronsUpDown } from 'lucide-react'
import Link from 'next/link'
import { appConfig, brand, texts } from '@/config'
import { LogoutButton } from '@/features/auth'

export type TopBarProps = {
  userName: string
  /** บริษัทปัจจุบัน — ไม่มี = หน้าเลือกบริษัท */
  companyName?: string
}

/** อักษรย่อของชื่อ (ใช้ในวงกลมผู้ใช้ / บริษัท) */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  return (parts.length > 1 ? parts[0]![0]! + parts[1]![0]! : name.trim().slice(0, 2)).toUpperCase()
}

/** แถบบนของหน้ารวมแอป / หน้าเลือกบริษัท: โลโก้ · บริษัท (กดเพื่อเปลี่ยน) ........ ผู้ใช้ · ออกจากระบบ */
export function TopBar({ userName, companyName }: TopBarProps) {
  return (
    <header className="sticky top-0 z-20 border-b border-line bg-surface/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4">
        <Link href={appConfig.routes.home} className="shrink-0">
          {/* eslint-disable-next-line @next/next/no-img-element -- โลโก้ WebP ที่ย่อไว้แล้ว */}
          <img src={brand.logo.src} alt={texts.app.logoAlt} width={brand.logo.width} height={brand.logo.height} className="h-8 w-auto" />
        </Link>
        {companyName !== undefined && (
          <>
            <span aria-hidden className="h-6 w-px bg-line" />
            <Link
              href={appConfig.routes.companies}
              title={texts.companies.switch}
              className="group flex min-w-0 items-center gap-2 rounded-(--radius-control) px-2 py-1.5 hover:bg-neutral-soft"
            >
              <Building2 aria-hidden className="size-4 shrink-0 text-ink-subtle" />
              <span className="truncate text-sm font-medium">{companyName}</span>
              <ChevronsUpDown aria-hidden className="size-3.5 shrink-0 text-ink-subtle group-hover:text-ink" />
              <span className="sr-only">{texts.companies.switch}</span>
            </Link>
          </>
        )}
        <div className="ml-auto flex items-center gap-2">
          <span className="hidden items-center gap-2 text-sm sm:flex">
            <span aria-hidden className="flex size-7 items-center justify-center rounded-full bg-primary-soft text-[11px] font-semibold text-primary">
              {initials(userName)}
            </span>
            <span className="text-ink-muted">{userName}</span>
          </span>
          <LogoutButton fullWidth={false} size="sm" variant="ghost" />
        </div>
      </div>
    </header>
  )
}
