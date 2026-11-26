'use client'

import { Undo2 } from 'lucide-react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormBar } from '@/features/forms/FormParts'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { requiredMessage } from '@/lib/validation'
import { formatAmount, sum } from './money'
import type { Entry, Ref } from './types'

/**
 * รายการที่ผ่านแล้ว / กลับรายการแล้ว / ไม่มีสิทธิ์แก้ — แสดงอย่างเดียว
 * กลับรายการ (สิทธิ์ ledger.entry:post): เปิดแผงเลือกวันที่ + คำอธิบาย แล้วยืนยัน
 */
export function EntryView({
  entry,
  apiPath,
  listHref,
  entriesPath,
  canPost,
  dimRefs,
}: {
  entry: Entry
  apiPath: string
  listHref: string
  entriesPath: string
  canPost: boolean
  dimRefs: Record<string, Ref>
}) {
  const t = texts.entries
  const tl = t.lines
  const router = useRouter()
  const [reversing, setReversing] = useState(false)
  const [rev, setRev] = useState({ date: today(), description: '' })
  const [dateError, setDateError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const dims = (l: Entry['lines'][number]) =>
    (
      [
        [tl.branch, l.branch_id],
        [tl.department, l.department_id],
        [tl.costCenter, l.cost_center_id],
        [tl.projectSite, l.project_site_id],
      ] as const
    ).filter(([, id]) => id)

  async function reverse(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!rev.date) return setDateError(requiredMessage(t.actions.reverseDate))
    setDateError(undefined)
    setBusy(true)
    setError(null)
    try {
      const created = await apiRequest<Entry>(`${apiPath}/${entry.id}/reversals`, { method: 'POST', body: { date: rev.date, description: rev.description.trim() } })
      showToast(t.toast.reversed)
      router.push(`${entriesPath}/${created.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      {entry.status === 'reversed' && <Alert>{t.reversedNotice}</Alert>}

      <dl className="grid gap-x-8 gap-y-3 rounded-(--radius-panel) border border-line bg-surface p-5 text-sm shadow-card sm:grid-cols-2 lg:grid-cols-3">
        {(
          [
            [t.meta.entryNo, <span className="font-mono tabular-nums">{entry.entry_no ?? t.noNumber}</span>],
            [t.fields.date, formatDate(entry.date)],
            [t.fields.journal, `${entry.journal.code} · ${entry.journal.name}`],
            [t.fields.reference, entry.reference || '–'],
            [t.fields.description, entry.description || '–'],
            ...(entry.reversal_of_id
              ? [[t.meta.reversalOf, <Link className="text-primary hover:underline" href={`${entriesPath}/${entry.reversal_of_id}`}>{t.meta.openEntry}</Link>] as const]
              : []),
            ...(entry.reversed_by_id
              ? [[t.meta.reversedBy, <Link className="text-primary hover:underline" href={`${entriesPath}/${entry.reversed_by_id}`}>{t.meta.openEntry}</Link>] as const]
              : []),
          ] as const
        ).map(([label, value]) => (
          <div key={label} className="min-w-0">
            <dt className="text-[13px] text-ink-muted">{label}</dt>
            <dd className="mt-0.5 break-words">{value}</dd>
          </div>
        ))}
      </dl>

      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[40rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
              <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                #
              </th>
              <th scope="col" className="px-3 py-2.5 font-medium">
                {tl.account}
              </th>
              <th scope="col" className="w-40 px-3 py-2.5 text-right font-medium">
                {tl.debit}
              </th>
              <th scope="col" className="w-40 px-3 py-2.5 text-right font-medium">
                {tl.credit}
              </th>
            </tr>
          </thead>
          <tbody>
            {entry.lines.map((l) => (
              <tr key={l.line_no} className="border-b border-line align-top">
                <td className="px-3 py-2.5 text-center text-[13px] text-ink-subtle tabular-nums">{l.line_no}</td>
                <td className="px-3 py-2.5">
                  <span className="font-mono text-[13px] text-ink-muted">{l.account.code}</span> {l.account.name}
                  {l.description && <span className="block text-[13px] text-ink-muted">{l.description}</span>}
                  {dims(l).length > 0 && (
                    <span className="mt-1 flex flex-wrap gap-1.5">
                      {dims(l).map(([label, id]) => {
                        const r = dimRefs[id!]
                        return (
                          <span key={label} className="rounded-full bg-neutral-soft px-2 py-0.5 text-xs text-ink-muted">
                            {label}: {r ? `${r.code} ${r.name}` : id}
                          </span>
                        )
                      })}
                    </span>
                  )}
                </td>
                <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(l.debit, { dashZero: true })}</td>
                <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(l.credit, { dashZero: true })}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="bg-subtle font-semibold">
              <td colSpan={2} className="px-3 py-3 text-right text-[13px] text-ink-muted">
                {tl.total}
              </td>
              <td className="px-3 py-3 text-right tabular-nums">{formatAmount(sum(entry.lines.map((l) => l.debit)))}</td>
              <td className="px-3 py-3 text-right tabular-nums">{formatAmount(sum(entry.lines.map((l) => l.credit)))}</td>
            </tr>
          </tfoot>
        </table>
      </div>

      {reversing && (
        <form noValidate onSubmit={reverse} className="rounded-(--radius-panel) border border-warning/40 bg-warning-soft/50 p-5">
          <h2 className="flex items-center gap-2 text-[15px] font-semibold">
            <Undo2 aria-hidden className="size-4" />
            {t.actions.reverseTitle}
          </h2>          <div className="mt-4 grid gap-4 sm:grid-cols-[12rem_1fr]">
            <TextField label={t.actions.reverseDate} required type="date" value={rev.date} error={dateError} onChange={(e) => setRev((r) => ({ ...r, date: e.target.value }))} />
            <TextField
              label={t.actions.reverseDescription}
              value={rev.description}
              maxLength={500}
              placeholder={format('{no}', { no: entry.entry_no ?? '' })}
              onChange={(e) => setRev((r) => ({ ...r, description: e.target.value }))}
            />
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="secondary" disabled={busy} onClick={() => setReversing(false)}>
              {texts.common.cancel}
            </Button>
            <Button type="submit" variant="danger" loading={busy} loadingText={t.actions.reversing}>
              {t.actions.reverseConfirm}
            </Button>
          </div>
        </form>
      )}

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.back}
          </ButtonLink>
          {canPost && entry.status === 'posted' && !reversing && (
            <Button variant="secondary" onClick={() => setReversing(true)}>
              <Undo2 aria-hidden />
              {t.actions.reverse}
            </Button>
          )}
        </div>
      </FormBar>
    </div>
  )
}
