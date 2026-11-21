import type { Metadata } from 'next'
import { Alert } from '@/components'
import { format, texts } from '@/config'
import { CompanySettingsForm, type Company } from '@/features/organization/CompanySettingsForm'
import { StatusBadge } from '@/features/lists/ListParts'
import { COMPANY_MANAGE } from '@/features/organization/resources'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.company.pageTitle }

/** ข้อมูลบริษัท — ต้องมีสิทธิ์ organization.company:read (เมนูซ่อนถ้าไม่มี), แก้ไขต้องมี :manage */
export default async function CompanyPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null

  let company: Company
  try {
    company = (await backendGet<Company>(`/companies/${companyId}`)).data
  } catch (err) {
    return <Alert tone="error">{errorMessage(err)}</Alert>
  }

  return (
    <div className="mx-auto max-w-5xl">
      <header className="mb-8 flex flex-wrap items-center gap-4 border-b border-line pb-6">
        <span aria-hidden className="flex size-12 items-center justify-center rounded-xl bg-primary text-sm font-semibold tracking-wide text-on-primary shadow-tile">
          {company.code.slice(0, 2)}
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[13px] font-medium text-ink-muted">{texts.company.heading}</p>
          <h1 className="truncate text-[22px] leading-tight font-semibold tracking-tight">{company.name}</h1>
          <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-[13px] text-ink-muted">
            <span>{company.code}</span>
            {company.tax_id && (
              <>
                <span aria-hidden>·</span>
                <span className="tabular-nums">{format(texts.companies.taxId, { taxId: company.tax_id })}</span>
              </>
            )}
          </p>
        </div>
        <StatusBadge status={company.status} />
      </header>
      <CompanySettingsForm company={company} canManage={can(ctx.permissions, COMPANY_MANAGE)} />
    </div>
  )
}
