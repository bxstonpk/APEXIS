import { ChevronRight } from 'lucide-react'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, ReferenceSelect, type RefOption } from '@/components'
import { texts } from '@/config'
import { MessagePanel } from '@/features/shell'
import { ApiError, errorMessage } from '@/lib/http'
import { backendGet } from '@/lib/server/backend'
import { isCompanyId } from '@/lib/server/company'

/** ส่วนที่หน้าเพิ่ม / แก้ไขของแอปองค์กรใช้ร่วมกัน (server) */

/** โหลดรายการเดียว — ไม่พบ / ผิดพลาด คืนข้อความแทนหน้า */
export async function loadItem<T>(apiPath: string, id: string, notFound: string): Promise<{ item: T } | { panel: ReactNode }> {
  if (!isCompanyId(id)) return { panel: <MessagePanel title={notFound} /> }
  try {
    return { item: (await backendGet<T>(`${apiPath}/${id}`)).data }
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return { panel: <MessagePanel title={notFound} /> }
    return { panel: <Alert tone="error">{errorMessage(err)}</Alert> }
  }
}

/** label / href = หน้ารายการ; app = ชื่อแอป (ไม่ระบุ = องค์กร) */
export type Crumb = { label: string; href: string; app?: string }

/**
 * กรอบหน้าฟอร์ม / รายละเอียด: เส้นทาง (องค์กร › รายการ › หน้านี้) + หัวข้อ + เนื้อหา
 * เส้นทางช่วยให้รู้ตำแหน่งและกลับไปรายการได้ (หน้าลึก 3 ระดับ)
 */
export function FormPage({
  title,
  description,
  crumb,
  aside,
  children,
}: {
  title: string
  description?: string
  crumb?: Crumb
  aside?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="mx-auto max-w-5xl">
      <header className="mb-8 border-b border-line pb-6">
        {crumb && (
          <nav aria-label={texts.common.breadcrumb} className="mb-2">
            <ol className="flex flex-wrap items-center gap-1 text-[13px] text-ink-muted">
              <li>{crumb.app ?? texts.apps.organization.name}</li>
              <li aria-hidden>
                <ChevronRight className="size-3.5" />
              </li>
              <li>
                <Link href={crumb.href} className="font-medium text-primary hover:underline">
                  {crumb.label}
                </Link>
              </li>
              <li aria-hidden>
                <ChevronRight className="size-3.5" />
              </li>
              <li aria-current="page" className="max-w-60 truncate">
                {title}
              </li>
            </ol>
          </nav>
        )}
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{title}</h1>
            {description && <p className="mt-1 text-sm text-ink-muted">{description}</p>}
          </div>
          {aside}
        </div>
      </header>
      {children}
    </div>
  )
}

export function NoPermission() {
  return <MessagePanel icon="forbidden" title={texts.common.noPermission} />
}

/** ช่องกรองด้วยข้อมูลอ้างอิงในแถบค้นหา (ส่ง id ลง URL ด้วยชื่อ name) */
export function RefFilter({ label, name, source, value }: { label: string; name: string; source: string; value: RefOption | null }) {
  return <ReferenceSelect label={label} name={name} source={source} defaultValue={value} texts={texts.reference} className="w-full sm:w-52" />
}
