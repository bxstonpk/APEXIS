'use client'

import { CircleCheck, CircleAlert, Plus, SlidersHorizontal, Trash2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { Fragment, useRef, useState, type FormEvent } from 'react'
import { Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormBar, FormNotices, FormSection } from '@/features/forms/FormParts'
import { cx } from '@/lib/cx'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { formatAmount, normalizeAmount, sum, toUnits } from './money'
import type { Entry, Ref } from './types'

/**
 * ฟอร์มรายการบัญชี (สร้าง / แก้ร่าง)
 *
 * - ตารางบรรทัด: บัญชี, คำอธิบาย, เดบิต, เครดิต — กรอกด้านหนึ่งแล้วอีกด้านถูกล้าง (บรรทัดละด้านเดียว)
 * - ยอดรวม / ผลต่างคำนวณสดด้วย BigInt (ไม่มีเศษ float) — สมดุลแสดงทั้งไอคอนและข้อความ (ไม่ใช้สีอย่างเดียว)
 * - มิติวิเคราะห์ (สาขา / แผนก / ศูนย์ต้นทุน / โครงการ) ซ่อนไว้ในแต่ละบรรทัด กด "มิติวิเคราะห์" เพื่อเปิด
 * - บันทึกร่างได้แม้ยังไม่สมดุล; "บันทึกและผ่านรายการ" ต้องสมดุล (backend ตรวจซ้ำ + ออกเลขที่)
 */

type Dims = { branch: Ref | null; department: Ref | null; cost_center: Ref | null; project_site: Ref | null }
const DIM_KEYS = ['branch', 'department', 'cost_center', 'project_site'] as const

type Line = { key: number; account: Ref | null; description: string; debit: string; credit: string; dims: Dims; open: boolean }
type LineErrors = Partial<Record<'account' | 'amount', string>>

const noDims: Dims = { branch: null, department: null, cost_center: null, project_site: null }

export type EntryFormProps = {
  companyApi: { entries: string; accounts: string; journals: string; branches: string; departments: string; costCenters: string; projectSites: string }
  listHref: string
  /** path ของหน้ารายการบัญชี (หน้ารายละเอียด = {entriesPath}/{id}) */
  entriesPath: string
  canPost: boolean
  /** ร่างที่แก้ไข (ไม่มี = สร้างใหม่) */
  entry?: Entry
  /** ชื่อของมิติวิเคราะห์ที่บรรทัดของร่างอ้างถึง (id → ref) */
  dimRefs?: Record<string, Ref>
  /** สมุดเริ่มต้นของรายการใหม่ */
  defaultJournal?: Ref | null
}

let nextKey = 1
function newLine(partial: Partial<Line> = {}): Line {
  return { key: nextKey++, account: null, description: '', debit: '', credit: '', dims: noDims, open: false, ...partial }
}

function linesFrom(entry: Entry | undefined, refs: Record<string, Ref>): Line[] {
  if (!entry) return [newLine(), newLine()]
  const ref = (id: string | null) => (id ? (refs[id] ?? { id, code: '', name: id }) : null)
  return entry.lines.map((l) =>
    newLine({
      account: l.account,
      description: l.description,
      debit: toUnits(l.debit) === 0n ? '' : l.debit,
      credit: toUnits(l.credit) === 0n ? '' : l.credit,
      dims: { branch: ref(l.branch_id), department: ref(l.department_id), cost_center: ref(l.cost_center_id), project_site: ref(l.project_site_id) },
    }),
  )
}

const dimCount = (d: Dims) => DIM_KEYS.filter((k) => d[k]).length

export function EntryForm({ companyApi, listHref, entriesPath, canPost, entry, dimRefs = {}, defaultJournal = null }: EntryFormProps) {
  const t = texts.entries
  const tl = t.lines
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [journal, setJournal] = useState<Ref | null>(entry?.journal ?? defaultJournal)
  const [head, setHead] = useState({ date: entry?.date ?? today(), reference: entry?.reference ?? '', description: entry?.description ?? '' })
  const [lines, setLines] = useState<Line[]>(() => linesFrom(entry, dimRefs))
  const [errors, setErrors] = useState<{ journal?: string; date?: string; lines?: string; byLine: Record<number, LineErrors> }>({ byLine: {} })
  const [busy, setBusy] = useState<'draft' | 'post' | 'delete' | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const totalDebit = sum(lines.map((l) => l.debit))
  const totalCredit = sum(lines.map((l) => l.credit))
  const diff = totalDebit - totalCredit
  const balanced = diff === 0n && totalDebit > 0n

  const update = (key: number, patch: Partial<Line>) => setLines((ls) => ls.map((l) => (l.key === key ? { ...l, ...patch } : l)))
  const setAmount = (key: number, side: 'debit' | 'credit', raw: string) =>
    update(key, side === 'debit' ? { debit: raw, ...(raw ? { credit: '' } : {}) } : { credit: raw, ...(raw ? { debit: '' } : {}) })
  const tidy = (key: number, side: 'debit' | 'credit', raw: string) => {
    const n = normalizeAmount(raw)
    if (n !== null && n !== raw) update(key, { [side]: n === '0' ? '' : n })
  }

  function validate(post: boolean) {
    const byLine: Record<number, LineErrors> = {}
    const used = lines.filter((l) => l.account || l.debit || l.credit || l.description.trim())
    let withAmount = 0
    for (const l of used) {
      const e: LineErrors = {}
      if (!l.account) e.account = tl.accountRequired
      const d = normalizeAmount(l.debit)
      const c = normalizeAmount(l.credit)
      if (d === null || c === null || toUnits(d) < 0n || toUnits(c) < 0n) e.amount = tl.invalidAmount
      else if (toUnits(d) > 0n && toUnits(c) > 0n) e.amount = tl.bothSides
      else if (toUnits(d) === 0n && toUnits(c) === 0n) e.amount = tl.amountRequired
      else withAmount++
      if (e.account || e.amount) byLine[l.key] = e
    }
    const out = {
      journal: journal ? undefined : requiredMessage(t.fields.journal),
      date: head.date ? undefined : requiredMessage(t.fields.date),
      lines: withAmount < 2 ? tl.minLines : post && !balanced ? format(tl.unbalanced, { amount: formatAmount(diff < 0n ? -diff : diff) }) : undefined,
      byLine,
    }
    setErrors(out)
    const ok = !out.journal && !out.date && !out.lines && Object.keys(byLine).length === 0
    return { ok, used }
  }

  async function submit(post: boolean) {
    setFormError(null)
    const { ok, used } = validate(post)
    if (!ok) return focusFirstInvalid(formRef.current)
    const body = {
      journal_id: journal!.id,
      date: head.date,
      reference: head.reference.trim(),
      description: head.description.trim(),
      lines: used.map((l) => ({
        account_id: l.account!.id,
        description: l.description.trim(),
        debit: normalizeAmount(l.debit) || '0',
        credit: normalizeAmount(l.credit) || '0',
        branch_id: l.dims.branch?.id ?? null,
        department_id: l.dims.department?.id ?? null,
        cost_center_id: l.dims.cost_center?.id ?? null,
        project_site_id: l.dims.project_site?.id ?? null,
      })),
    }
    setBusy(post ? 'post' : 'draft')
    try {
      let saved: Entry
      if (!entry) {
        saved = await apiRequest<Entry>(companyApi.entries, { method: 'POST', body: { ...body, status: post ? 'posted' : 'draft' } })
      } else {
        saved = await apiRequest<Entry>(`${companyApi.entries}/${entry.id}`, { method: 'PUT', body })
        if (post) saved = await apiRequest<Entry>(`${companyApi.entries}/${entry.id}`, { method: 'PATCH', body: { status: 'posted' } })
      }
      showToast(post ? format(t.toast.posted, { no: saved.entry_no ?? '' }) : t.toast.draftSaved)
      router.push(`${entriesPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(null)
    }
  }

  async function remove() {
    if (!entry || !window.confirm(t.actions.deleteConfirm)) return
    setBusy('delete')
    try {
      await apiRequest(`${companyApi.entries}/${entry.id}`, { method: 'DELETE' })
      showToast(texts.common.toast.deleted)
      router.push(listHref)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(null)
    }
  }

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    void submit(false)
  }

  const dimSources = { branch: companyApi.branches, department: companyApi.departments, cost_center: companyApi.costCenters, project_site: companyApi.projectSites }
  const dimLabels = { branch: tl.branch, department: tl.department, cost_center: tl.costCenter, project_site: tl.projectSite }

  return (
    <form ref={formRef} noValidate onSubmit={onSubmit} className="flex flex-col gap-6">
      <FormNotices formError={formError} readOnly={false} />
      <FormSection title={t.sections.header}>
        <ReferenceSelect
          label={t.fields.journal}
          required
          source={companyApi.journals}
          params={{ status: 'active' }}
          value={journal}
          onChange={setJournal}
          error={errors.journal}
          texts={texts.reference}
        />
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField
          label={t.fields.reference}
          hint={t.referenceHint}
          value={head.reference}
          maxLength={100}
          onChange={(e) => setHead((h) => ({ ...h, reference: e.target.value }))}
        />
        <TextField label={t.fields.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <section aria-labelledby="entry-lines" className="flex flex-col gap-3">
        <h2 id="entry-lines" className="text-[15px] font-semibold">
          {t.sections.lines}
        </h2>
        <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[52rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="w-10 px-3 py-2.5 text-center font-medium">
                  #
                </th>
                <th scope="col" className="w-[32%] px-2 py-2.5 font-medium">
                  {tl.account}
                </th>
                <th scope="col" className="px-2 py-2.5 font-medium">
                  {tl.description}
                </th>
                <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                  {tl.debit}
                </th>
                <th scope="col" className="w-36 px-2 py-2.5 text-right font-medium">
                  {tl.credit}
                </th>
                <th scope="col" className="w-24 px-2 py-2.5">
                  <span className="sr-only">{tl.dimensions}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {lines.map((l, i) => {
                const n = i + 1
                const e = errors.byLine[l.key] ?? {}
                const count = dimCount(l.dims)
                const dimsId = `dims-${l.key}`
                return (
                  <Fragment key={l.key}>
                    <tr className={cx('align-top', !l.open && 'border-b border-line')}>
                      <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{n}</td>
                      <td className="px-2 py-2">
                        <ReferenceSelect
                          label={`${tl.account} ${format(tl.line, { n })}`}
                          hideLabel
                          source={companyApi.accounts}
                          params={{ postable: 'true', status: 'active' }}
                          value={l.account}
                          onChange={(a) => update(l.key, { account: a })}
                          error={e.account}
                          texts={texts.reference}
                        />
                      </td>
                      <td className="px-2 py-2">
                        <TextField
                          label={`${tl.description} ${format(tl.line, { n })}`}
                          hideLabel
                          value={l.description}
                          maxLength={500}
                          onChange={(ev) => update(l.key, { description: ev.target.value })}
                        />
                      </td>
                      {(['debit', 'credit'] as const).map((side) => (
                        <td key={side} className="px-2 py-2">
                          <TextField
                            label={`${side === 'debit' ? tl.debit : tl.credit} ${format(tl.line, { n })}`}
                            hideLabel
                            inputMode="decimal"
                            autoComplete="off"
                            value={l[side]}
                            error={side === 'debit' ? e.amount : undefined}
                            className="[&_input]:text-right [&_input]:tabular-nums"
                            onChange={(ev) => setAmount(l.key, side, ev.target.value)}
                            onBlur={(ev) => tidy(l.key, side, ev.target.value)}
                          />
                        </td>
                      ))}
                      <td className="px-2 py-2">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            aria-expanded={l.open}
                            aria-controls={dimsId}
                            title={l.open ? tl.hideDimensions : tl.showDimensions}
                            aria-label={`${l.open ? tl.hideDimensions : tl.showDimensions} ${format(tl.line, { n })}`}
                            onClick={() => update(l.key, { open: !l.open })}
                            className={cx(
                              'relative flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) hover:bg-neutral-soft focus-visible:outline-2 focus-visible:outline-primary',
                              l.open || count ? 'text-primary' : 'text-ink-subtle',
                            )}
                          >
                            <SlidersHorizontal aria-hidden className="size-4" />
                            {count > 0 && (
                              <span className="absolute top-1 right-1 flex size-4 items-center justify-center rounded-full bg-primary text-[10px] font-semibold text-white">
                                {count}
                              </span>
                            )}
                          </button>
                          <button
                            type="button"
                            aria-label={format(tl.remove, { n })}
                            disabled={lines.length <= 2}
                            onClick={() => setLines((ls) => ls.filter((x) => x.key !== l.key))}
                            className="flex size-10 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-subtle hover:bg-danger-soft hover:text-danger focus-visible:outline-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ink-subtle"
                          >
                            <Trash2 aria-hidden className="size-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                    {l.open && (
                      <tr id={dimsId} className="border-b border-line bg-subtle/60">
                        <td />
                        <td colSpan={5} className="px-2 pt-1 pb-3">
                          <p className="mb-2 text-[13px] font-medium text-ink-muted">{format(tl.dimensionCount, { n: count })}</p>
                          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                            {DIM_KEYS.map((k) => (
                              <ReferenceSelect
                                key={k}
                                label={dimLabels[k]}
                                source={dimSources[k]}
                                params={{ status: 'active' }}
                                value={l.dims[k]}
                                onChange={(r) => update(l.key, { dims: { ...l.dims, [k]: r } })}
                                texts={texts.reference}
                              />
                            ))}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
            <tfoot>
              <tr className="bg-subtle font-semibold">
                <td colSpan={2} className="px-3 py-3">
                  <Button size="sm" variant="secondary" onClick={() => setLines((ls) => [...ls, newLine()])}>
                    <Plus aria-hidden />
                    {tl.add}
                  </Button>
                </td>
                <td className="px-2 py-3 text-right text-[13px] text-ink-muted">{tl.total}</td>
                <td className="px-5 py-3 text-right tabular-nums">{formatAmount(totalDebit)}</td>
                <td className="px-5 py-3 text-right tabular-nums">{formatAmount(totalCredit)}</td>
                <td />
              </tr>
            </tfoot>
          </table>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-3 text-sm" aria-live="polite">
          {balanced ? (
            <span className="inline-flex items-center gap-1.5 font-medium text-success">
              <CircleCheck aria-hidden className="size-4" />
              {tl.balanced}
            </span>
          ) : (
            totalDebit + totalCredit > 0n && (
              <span className="inline-flex items-center gap-1.5 font-medium text-warning">
                <CircleAlert aria-hidden className="size-4" />
                {format(tl.unbalanced, { amount: formatAmount(diff < 0n ? -diff : diff) })}
              </span>
            )
          )}
        </div>
        {errors.lines && (
          <p role="alert" className="text-right text-[13px] text-danger">
            {errors.lines}
          </p>
        )}
      </section>

      <FormBar>
        {entry && (
          <Button variant="danger" loading={busy === 'delete'} loadingText={texts.common.deleting} disabled={busy !== null} onClick={remove}>
            <Trash2 aria-hidden />
            {texts.common.delete}
          </Button>
        )}
        <div className="ml-auto flex flex-wrap gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" variant={canPost ? 'secondary' : 'primary'} loading={busy === 'draft'} loadingText={texts.common.saving} disabled={busy !== null}>
            {t.actions.saveDraft}
          </Button>
          {canPost && (
            <Button
              loading={busy === 'post'}
              loadingText={t.actions.posting}
              disabled={busy !== null}
              onClick={() => {
                if (window.confirm(t.actions.postConfirm)) void submit(true)
              }}
            >
              {t.actions.saveAndPost}
            </Button>
          )}
        </div>
      </FormBar>
    </form>
  )
}
