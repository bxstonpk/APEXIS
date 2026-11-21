import type { Metadata } from 'next'
import { Badge } from '@/components'
import { texts } from '@/config'
import { EntryForm } from '@/features/accounting/EntryForm'
import { EntryView } from '@/features/accounting/EntryView'
import { entryFormApi, loadDimensionRefs } from '@/features/accounting/pages'
import { ENTRY_QUERY_KEYS, ledgerApi, ledgerPage, perms, type Entry } from '@/features/accounting/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.entries.pageTitle }

type Props = { params: Promise<{ companyId: string; entryId: string }>; searchParams: Promise<SearchParams> }

const statusTone = { draft: 'warning', posted: 'success', reversed: 'neutral' } as const

/** ร่าง + มีสิทธิ์แก้ = ฟอร์ม; ผ่านแล้ว / กลับรายการแล้ว / ดูอย่างเดียว = หน้าแสดงรายการ (กลับรายการได้) */
export default async function EntryPage({ params, searchParams }: Props) {
  const { companyId, entryId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.entries
  const path = ledgerPage.entries(companyId)
  const listHref = listHrefFromBack(path, param(await searchParams, 'back'), ENTRY_QUERY_KEYS)
  const res = await loadItem<Entry>(ledgerApi.entries(companyId), entryId, t.notFound)
  if ('panel' in res) return res.panel
  const e = res.item
  const dimRefs = await loadDimensionRefs(companyId, e.lines)
  const editable = e.status === 'draft' && can(ctx.permissions, perms.entryManage)
  const canPost = can(ctx.permissions, perms.entryPost)
  return (
    <FormPage
      crumb={{ app: texts.apps.accounting.name, label: t.heading, href: listHref }}
      title={e.entry_no ?? t.draftTitle}
      description={e.description || undefined}
      aside={<Badge tone={statusTone[e.status]} dot>{t.statuses[e.status] ?? e.status}</Badge>}
    >
      {editable ? (
        <EntryForm key={e.id} companyApi={entryFormApi(companyId)} listHref={listHref} entriesPath={path} canPost={canPost} entry={e} dimRefs={dimRefs} />
      ) : (
        <EntryView key={e.id} entry={e} apiPath={ledgerApi.entries(companyId)} listHref={listHref} entriesPath={path} canPost={canPost} dimRefs={dimRefs} />
      )}
    </FormPage>
  )
}
