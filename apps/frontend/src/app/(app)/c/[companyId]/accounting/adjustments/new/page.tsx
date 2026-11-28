import type { Metadata } from 'next'
import { texts } from '@/config'
import { AdjustmentForm } from '@/features/accounting/AdjustmentForm'
import { ledgerApi, ledgerPage, perms, type AdjustmentTemplate, type LedgerSettings, type YearEnd } from '@/features/accounting/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.adjustments.newTitle }

/** บันทึกรายการปรับปรุง — แบบ / ปีที่ยังไม่ปิด (งวดปรับปรุง) / สมุดตั้งต้นโหลดพร้อมกัน */
export default async function NewAdjustmentPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.entryPost)) return <NoPermission />
  const path = ledgerPage.adjustments(companyId)
  const [templates, years, settings] = await Promise.all([
    backendGet<AdjustmentTemplate[]>(ledgerApi.adjustmentTemplates(companyId)).then((r) => r.data, () => []),
    backendGet<YearEnd[]>(ledgerApi.yearEnds(companyId)).then((r) => r.data, () => []),
    backendGet<LedgerSettings>(ledgerApi.ledgerSettings(companyId)).then((r) => r.data, () => null),
  ])
  const openYears = years.filter((y) => y.status === 'open').map((y) => ({ fiscal_year: y.fiscal_year, end_date: y.end_date }))
  return (
    <FormPage crumb={{ app: texts.apps.accounting.name, label: texts.adjustments.heading, href: path }} title={texts.adjustments.newTitle}>
      <AdjustmentForm
        api={{ adjustments: ledgerApi.adjustments(companyId), accounts: ledgerApi.accounts(companyId), journals: ledgerApi.journals(companyId) }}
        entriesPath={ledgerPage.entries(companyId)}
        listHref={path}
        templates={templates}
        openYears={openYears}
        journal={settings?.closing_journal ?? null}
      />
    </FormPage>
  )
}
