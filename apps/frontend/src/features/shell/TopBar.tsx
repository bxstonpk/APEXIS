import { ArrowLeftRight } from 'lucide-react'
import Link from 'next/link'
import { appConfig, brand, texts } from '@/config'
import { LogoutButton } from '@/features/auth'

export type TopBarProps = {
  userName: string
  /** บริษัทปัจจุบัน — ไม่มี = หน้าเลือกบริษัท */
  companyName?: string
}

/** แถบบนของหน้ารวมแอป: โลโก้, บริษัท + เปลี่ยนบริษัท, ผู้ใช้, ออกจากระบบ */
export function TopBar({ userName, companyName }: TopBarProps) {
  return (
    <header className="border-b border-line bg-surface">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-4 px-4">
        {/* eslint-disable-next-line @next/next/no-img-element -- โลโก้ WebP ที่ย่อไว้แล้ว */}
        <img src={brand.logo.src} alt={texts.app.logoAlt} width={brand.logo.width} height={brand.logo.height} className="h-9 w-auto" />
        {companyName !== undefined && (
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate font-semibold">{companyName}</span>
            <Link
              href={appConfig.routes.companies}
              title={texts.companies.switch}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm text-primary hover:bg-info-soft"
            >
              <ArrowLeftRight aria-hidden className="size-4" />
              <span className="hidden sm:inline">{texts.companies.switch}</span>
            </Link>
          </div>
        )}
        <div className="ml-auto flex items-center gap-3">
          <span className="hidden text-sm text-ink-muted sm:inline">{userName}</span>
          <LogoutButton fullWidth={false} size="sm" />
        </div>
      </div>
    </header>
  )
}
