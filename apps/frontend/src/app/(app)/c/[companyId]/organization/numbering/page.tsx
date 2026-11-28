import type { Metadata } from 'next'
import { Alert, PageHeader } from '@/components'
import { texts } from '@/config'
import { NumberingTable } from '@/features/organization/NumberingTable'
import { NUMBERING_MANAGE, type DocumentNumbering } from '@/features/organization/resources'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.numbering.pageTitle }

/** เลขที่เอกสาร — ทุกชนิดเอกสารในคิวรีเดียว (จำนวนชนิดคงที่ ไม่ต้องแบ่งหน้า) */
export default async function NumberingPage({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const apiPath = `/companies/${companyId}/document-numberings`

  let items: DocumentNumbering[] = []
  let error: string | null = null
  try {
    items = (await backendGet<DocumentNumbering[]>(apiPath)).data
  } catch (err) {
    error = errorMessage(err)
  }

  return (
    <>
      <PageHeader title={texts.numbering.heading} />
      {error ? <Alert tone="error">{error}</Alert> : <NumberingTable apiPath={apiPath} items={items} canManage={can(ctx.permissions, NUMBERING_MANAGE)} />}
    </>
  )
}
