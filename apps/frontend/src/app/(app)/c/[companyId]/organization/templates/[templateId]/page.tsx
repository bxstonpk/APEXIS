import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { FormPage, loadItem, NoPermission } from '@/features/organization/pages'
import { TemplateEditor } from '@/features/print-templates/TemplateEditor'
import type { DocumentTemplate } from '@/features/print-templates/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.printTemplates.pageTitle }

/** แก้แม่แบบ — ชนิดเอกสารเปลี่ยนไม่ได้; ไม่มีสิทธิ์ manage = ดูอย่างเดียว */
export default async function Page({ params }: { params: Promise<{ companyId: string; templateId: string }> }) {
  const { companyId, templateId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, 'organization.document_template:read')) return <NoPermission />
  const t = texts.printTemplates
  const res = await loadItem<DocumentTemplate>(`/companies/${companyId}/document-templates`, templateId, t.notFound)
  if ('panel' in res) return res.panel
  const x = res.item
  const listHref = `/c/${companyId}/organization/templates`
  return (
    <FormPage crumb={{ app: texts.apps.organization.name, label: t.heading, href: listHref }} title={format(t.editTitle, { name: x.name })} description={t.docTypes[x.doc_type] ?? x.doc_type}>
      <TemplateEditor
        companyId={companyId}
        listHref={listHref}
        canManage={can(ctx.permissions, 'organization.document_template:manage')}
        initial={{ id: x.id, docType: x.doc_type, name: x.name, html: x.html ?? '', paper: x.paper, status: x.status }}
      />
    </FormPage>
  )
}
