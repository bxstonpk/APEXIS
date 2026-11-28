'use client'

import { useRef, useState, type FormEvent } from 'react'
import { ReferenceSelect, SelectField, TextField } from '@/components'
import { texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { focusFirstInvalid, normalizeCode, requiredMessage } from '@/lib/validation'
import type { Fund, Ref, UserRef } from './types'

/**
 * กองทุนเงินสดย่อย — body ตรงกับ POST / PUT /petty-cash-funds
 * วงเงินแก้ได้จนกว่าการเงินจ่ายใบขอตั้งวงเงิน (หลังตั้งแล้ว backend คงค่าเดิม — เพิ่มผ่านใบขอเพิ่มวงเงิน)
 */
export function FundForm({
  apiPath,
  listHref,
  accountsApi,
  members,
  canManage,
  fund,
}: {
  apiPath: string
  listHref: string
  accountsApi: string
  /** ผู้ถือเงินที่เลือกได้ (สมาชิกของบริษัท) */
  members: UserRef[]
  canManage: boolean
  fund?: Fund
}) {
  const t = texts.pettyCash.funds
  const f = t.fields
  const readOnly = !canManage
  const formRef = useRef<HTMLFormElement>(null)
  const form = useResourceForm({ apiPath, id: fund?.id, listHref })
  const [v, setV] = useState({
    code: fund?.code ?? '',
    name: fund?.name ?? '',
    custodian: fund?.custodian.id ?? '',
    imprest: fund?.imprest_amount ?? '',
    status: fund?.status ?? 'active',
  })
  const [account, setAccount] = useState<Ref | null>(fund?.gl_account ?? null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const options = [...members]
  if (fund && !options.some((m) => m.id === fund.custodian.id)) options.push(fund.custodian)

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    const errs: Record<string, string> = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (!v.custodian) errs.custodian = t.errors.custodianRequired
    const imprest = normalizeAmount(v.imprest)
    if (!imprest || toUnits(imprest) <= 0n) errs.imprest = t.errors.imprestInvalid
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    void form.save({
      code: v.code,
      name: v.name.trim(),
      custodian_user_id: v.custodian,
      gl_account_id: account?.id ?? null,
      imprest_amount: imprest,
      status: v.status,
    })
  }

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.general}>
        <TextField
          label={f.code}
          required
          value={v.code}
          maxLength={20}
          disabled={readOnly || !!fund}
          error={errors.code}
          onChange={(e) => setV((s) => ({ ...s, code: normalizeCode(e.target.value) }))}
        />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => setV((s) => ({ ...s, name: e.target.value }))} />
        <SelectField
          label={f.custodian}
          required
          value={v.custodian}
          disabled={readOnly}
          error={errors.custodian}
          options={[{ value: '', label: texts.common.none }, ...options.map((m) => ({ value: m.id, label: m.name || m.id }))]}
          onChange={(e) => setV((s) => ({ ...s, custodian: e.target.value }))}
        />
        <TextField
          label={f.imprest}
          required
          inputMode="decimal"
          value={v.imprest}
          disabled={readOnly || !!fund?.funded}
          error={errors.imprest}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => setV((s) => ({ ...s, imprest: e.target.value }))}
        />
        <ReferenceSelect
          label={f.glAccount}
          source={accountsApi}
          params={{ postable: 'true', status: 'active', type: 'asset' }}
          value={account}
          onChange={setAccount}
          disabled={readOnly}
          texts={texts.reference}
        />
        {fund && <StatusSelect value={v.status} disabled={readOnly} onChange={(status) => setV((s) => ({ ...s, status: status as Fund['status'] }))} />}
      </FormSection>
      <FormActions listHref={listHref} canManage={canManage} canDelete={false} busy={form.busy} />
    </form>
  )
}
