'use client'

import { useId, useState, type FormEvent } from 'react'
import { ReferenceSelect, SelectField, TextField } from '@/components'
import { errorClass, FieldLabel, fieldFrameClass, hintClass, inputTextClass } from '@/components/TextField/TextField'
import { format, texts } from '@/config'
import { FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { cx } from '@/lib/cx'
import { today } from '@/lib/format'
import { DEFAULT_TEMPLATE, previewNumber } from '@/lib/numbering'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import { ACCOUNT_TYPES, type Account, type AccountType, type Journal, type Ref } from './types'

/** รูปแบบรหัสเดียวกับ backend ledger value_objects (ตรวจช่วยผู้ใช้ — backend ตรวจซ้ำ) */
const ACCOUNT_CODE = /^[0-9A-Z][0-9A-Z._-]{0,19}$/
const JOURNAL_CODE = /^[A-Z][A-Z0-9]{0,9}$/

type CommonProps = { apiPath: string; listHref: string; canManage: boolean }

/** ช่องข้อความหลายบรรทัดในกรอบเดียวกับช่องอื่น */
function TextArea({
  label,
  hint,
  error,
  value,
  onChange,
  disabled,
  maxLength,
  className,
}: {
  label: string
  hint?: string
  error?: string
  value: string
  onChange: (v: string) => void
  disabled?: boolean
  maxLength?: number
  className?: string
}) {
  const id = useId()
  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      <FieldLabel htmlFor={id} label={label} />
      <div className={cx(fieldFrameClass(error, disabled), 'h-auto')}>
        <textarea
          id={id}
          rows={3}
          value={value}
          maxLength={maxLength}
          disabled={disabled}
          aria-describedby={hint ? `${id}-hint` : undefined}
          aria-invalid={error ? true : undefined}
          onChange={(e) => onChange(e.target.value)}
          className={cx(inputTextClass, 'w-full resize-y bg-transparent px-3 py-2 leading-relaxed outline-none disabled:cursor-not-allowed disabled:text-ink-muted')}
        />
      </div>
      {hint && !error && (
        <p id={`${id}-hint`} className={hintClass}>
          {hint}
        </p>
      )}
      {error && <p className={errorClass}>{error}</p>}
    </div>
  )
}

// ---------------------------------------------------------------- บัญชี

export function AccountForm({ apiPath, listHref, canManage, item }: CommonProps & { item?: Account }) {
  const t = texts.accounts
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    name_en: item?.name_en ?? '',
    type: (item?.type ?? 'asset') as AccountType,
    postable: item?.postable ?? true,
    status: item?.status ?? 'active',
    description: item?.description ?? '',
  })
  const [parent, setParent] = useState<Ref | null>(item?.parent ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name'>>({})
  const set = <K extends keyof typeof v>(k: K, val: (typeof v)[K]) => setV((s) => ({ ...s, [k]: val }))

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: FieldErrors<'code' | 'name'> = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    else if (!ACCOUNT_CODE.test(v.code)) errs.code = t.codeHint
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      name_en: v.name_en.trim(),
      type: v.type,
      parent_id: parent?.id ?? null,
      postable: v.postable,
      status: v.status,
      description: v.description.trim(),
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.general} description={t.sections.generalHint}>
        <TextField
          label={f.code}
          required
          hint={t.codeHint}
          value={v.code}
          maxLength={20}
          autoComplete="off"
          disabled={readOnly}
          error={errors.code}
          onChange={(e) => set('code', normalizeCode(e.target.value))}
        />
        {item ? <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s as 'active' | 'inactive')} /> : <span className="hidden sm:block" />}
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <TextField label={f.nameEn} value={v.name_en} maxLength={200} disabled={readOnly} onChange={(e) => set('name_en', e.target.value)} />
        <TextArea
          label={f.description}
          hint={t.descriptionHint}
          value={v.description}
          maxLength={500}
          disabled={readOnly}
          className="sm:col-span-2"
          onChange={(val) => set('description', val)}
        />
      </FormSection>
      <FormSection title={t.sections.structure} description={t.sections.structureHint}>
        <div className="flex flex-col gap-1.5">
          <SelectField
            label={f.type}
            required
            value={v.type}
            disabled={readOnly}
            options={ACCOUNT_TYPES.map((x) => ({ value: x, label: t.types[x] ?? x }))}
            onChange={(e) => {
              set('type', e.target.value as AccountType)
              setParent(null) // บัญชีคุมต้องหมวดเดียวกัน
            }}
          />
          {t.typeHints[v.type] && <p className={hintClass}>{t.typeHints[v.type]}</p>}
        </div>
        <SelectField
          label={f.kind}
          value={v.postable ? 'postable' : 'header'}
          disabled={readOnly}
          options={[
            { value: 'postable', label: t.kinds.postable },
            { value: 'header', label: t.kinds.header },
          ]}
          onChange={(e) => set('postable', e.target.value === 'postable')}
        />
        <ReferenceSelect
          key={v.type}
          label={f.parent}
          hint={t.parentHint}
          source={apiPath}
          params={{ postable: 'false', type: v.type }}
          value={parent}
          onChange={setParent}
          excludeId={item?.id}
          disabled={readOnly}
          texts={texts.reference}
          className="sm:col-span-2"
        />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(t.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}

// ---------------------------------------------------------------- สมุดรายวัน

export function JournalForm({ apiPath, listHref, canManage, item }: CommonProps & { item?: Journal }) {
  const t = texts.journals
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    template: item?.number_template ?? '',
    status: item?.status ?? 'active',
  })
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'template'>>({})
  const set = <K extends keyof typeof v>(k: K, val: (typeof v)[K]) => setV((s) => ({ ...s, [k]: val }))

  const d = new Date(`${today()}T00:00:00`)
  const preview = previewNumber(v.code || 'JV', v.template || DEFAULT_TEMPLATE, { date: d, fiscalYear: d.getFullYear() })

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: FieldErrors<'code' | 'name' | 'template'> = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    else if (!JOURNAL_CODE.test(v.code)) errs.code = t.invalidCode
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (!preview.ok) errs.template = preview.error
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: v.code, name: v.name.trim(), number_template: v.template.trim(), status: v.status })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.general} description={t.sections.generalHint}>
        <TextField
          label={f.code}
          required
          hint={t.codeHint}
          value={v.code}
          maxLength={10}
          autoComplete="off"
          disabled={readOnly}
          error={errors.code}
          onChange={(e) => set('code', normalizeCode(e.target.value))}
        />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s as 'active' | 'inactive')} />}
      </FormSection>
      <FormSection title={t.sections.numbering} description={t.sections.numberingHint}>
        <TextField
          label={f.template}
          hint={format(t.templateHint, { template: DEFAULT_TEMPLATE })}
          placeholder={DEFAULT_TEMPLATE}
          value={v.template}
          maxLength={60}
          autoComplete="off"
          disabled={readOnly}
          error={errors.template ?? (preview.ok ? undefined : preview.error)}
          onChange={(e) => set('template', e.target.value)}
        />
        <div className="flex flex-col gap-1.5">
          <span className="text-[13px] font-medium">{t.preview}</span>
          <output aria-live="polite" className="flex h-10 items-center rounded-(--radius-control) bg-subtle px-3 font-mono text-sm tabular-nums">
            {preview.ok ? preview.example : '–'}
          </output>
        </div>
        <details className="text-[13px] text-ink-muted sm:col-span-2">
          <summary className="cursor-pointer font-medium text-primary">{texts.numbering.helpTitle}</summary>
          <dl className="mt-2 grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1">
            {texts.numbering.help.map(([k, desc]) => (
              <div key={k} className="contents">
                <dt className="font-mono text-ink">{k}</dt>
                <dd>{desc}</dd>
              </div>
            ))}
          </dl>
          <p className="mt-2">{texts.numbering.resetHint}</p>
        </details>
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(t.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}
