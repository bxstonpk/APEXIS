import type { Metadata } from 'next'
import { Alert, ButtonLink, PageHeader, SelectField } from '@/components'
import { texts } from '@/config'
import { NoPermission } from '@/features/organization/pages'
import { AssetManager } from '@/features/print-templates/AssetManager'
import { TemplateList } from '@/features/print-templates/TemplateList'
import { TEMPLATE_DOC_TYPES, isTemplateDocType, type DocumentAsset, type DocumentTemplate } from '@/features/print-templates/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.printTemplates.pageTitle }

const READ = 'organization.document_template:read'
const MANAGE = 'organization.document_template:manage'

/** แม่แบบเอกสาร — แยกตามชนิดเอกสาร (?doc_type= กรอง) + รูปของแม่แบบ; แม่แบบมีไม่กี่ตัวต่อบริษัท จึงโหลดหน้าเดียว 100 รายการ */
export default async function Page({ params, searchParams }: { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, READ)) return <NoPermission />
  const t = texts.printTemplates
  const filter = param(sp, 'doc_type')
  const docType = filter && isTemplateDocType(filter) ? filter : ''
  const base = `/companies/${companyId}`
  const basePath = `/c/${companyId}/organization/templates`
  const q = new URLSearchParams({ page_size: '100' })
  if (docType) q.set('doc_type', docType)
  let items: DocumentTemplate[] = []
  let assets: DocumentAsset[] = []
  let error: string | null = null
  try {
    const [list, files] = await Promise.all([backendGet<DocumentTemplate[]>(`${base}/document-templates`, q), backendGet<DocumentAsset[]>(`${base}/document-assets`)])
    items = list.data
    assets = files.data
  } catch (err) {
    error = errorMessage(err)
  }
  const canManage = can(ctx.permissions, MANAGE)
  return (
    <>
      <PageHeader title={t.heading} />
      <form method="get" className="mb-4 flex flex-wrap items-end gap-2">
        <SelectField
          label={t.fields.docType}
          name="doc_type"
          defaultValue={docType}
          options={[{ value: '', label: texts.common.all }, ...TEMPLATE_DOC_TYPES.map((d) => ({ value: d, label: t.docTypes[d] ?? d }))]}
          className="w-full sm:w-80"
        />
        <button type="submit" className="h-10 cursor-pointer rounded-(--radius-control) border border-line-strong bg-surface px-4 text-sm font-medium hover:bg-subtle focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none">
          {texts.common.filter}
        </button>
        {canManage && (
          <div className="ml-auto">
            <ButtonLink href={`${basePath}/new${docType ? `?doc_type=${docType}` : ''}`}>{t.actions.create}</ButtonLink>
          </div>
        )}
      </form>
      {error ? (
        <Alert tone="error">{error}</Alert>
      ) : (
        <div className="flex flex-col gap-6">
          <TemplateList companyId={companyId} basePath={basePath} docTypes={docType ? [docType] : [...TEMPLATE_DOC_TYPES]} items={items} canManage={canManage} />
          <AssetManager companyId={companyId} items={assets} canManage={canManage} />
        </div>
      )}
    </>
  )
}
