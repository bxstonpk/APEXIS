import type { Metadata } from 'next'
import { texts } from '@/config'
import { CreditNoteList } from '@/features/credit-notes/pages'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.creditNotes.purchasing.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบลดหนี้ — สร้างจากหน้าใบกำกับภาษีเดิม (ปุ่ม "บันทึกใบลดหนี้") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  return <CreditNoteList mode="purchasing" companyId={companyId} sp={await searchParams} />
}
