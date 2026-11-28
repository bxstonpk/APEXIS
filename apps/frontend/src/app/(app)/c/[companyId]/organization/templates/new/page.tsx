import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { builtinTemplate } from '@/features/print-templates/builtin'
import { TemplateEditor } from '@/features/print-templates/TemplateEditor'
import { isTemplateDocType } from '@/features/print-templates/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.printTemplates.newTitle }

/** สร้างแม่แบบ — ?doc_type= เลือกชนิดเอกสาร, ?from=builtin = เริ่มจากแม่แบบมาตรฐาน (ไม่ส่ง = เริ่มจากแม่แบบมาตรฐานเช่นกัน แล้วนำเข้าไฟล์ได้) */
export default async function Page({ params, searchParams }: { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'organization.document_template:manage')) return <NoPermission />
  const t = texts.printTemplates
  const raw = param(sp, 'doc_type')
  const docType = raw && isTemplateDocType(raw) ? raw : 'sales.invoice'
  const builtin = builtinTemplate(docType)
  const listHref = `/c/${companyId}/organization/templates`
  return (
    <FormPage crumb={{ app: texts.apps.organization.name, label: t.heading, href: listHref }} title={t.newTitle}>
      <TemplateEditor
        companyId={companyId}
        listHref={listHref}
        canManage
        initial={{ docType, name: param(sp, 'from') === 'builtin' ? `${t.docTypes[docType] ?? docType}` : '', html: builtin.html, paper: builtin.paper, status: 'active' }}
      />
    </FormPage>
  )
}
