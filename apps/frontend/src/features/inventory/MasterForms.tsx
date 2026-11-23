'use client'

import { useState, type FormEvent, type ReactNode } from 'react'
import { ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { orNull, useResourceForm } from '@/features/forms/useResourceForm'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { Category, Item, ItemType, Ref, Uom, Warehouse } from './types'

/**
 * ทะเบียนหลักของคลังสินค้า: หน่วยนับ / หมวดสินค้า / คลัง / สินค้า
 * body ตรงกับ backend requests (Master, Category, Warehouse, Item) — id / จุดสั่งซื้อที่ว่างส่ง null
 */

type Copy = { deleteConfirm: string }

/** รหัส + ชื่อ (+ สถานะตอนแก้ไข) — ใช้ร่วมทุกฟอร์ม */
function useMaster(item?: { code: string; name: string; status: string }) {
  const [v, setV] = useState({ code: item?.code ?? '', name: item?.name ?? '', status: item?.status ?? 'active' })
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name'>>({})
  const check = () => {
    const errs: typeof errors = {}
    if (!v.code) errs.code = requiredMessage(texts.inventory.fields.code)
    if (!v.name.trim()) errs.name = requiredMessage(texts.inventory.fields.name)
    setErrors(errs)
    return errs
  }
  return { v, setV, errors, check }
}

function MasterFields({ m, readOnly, isEdit }: { m: ReturnType<typeof useMaster>; readOnly: boolean; isEdit: boolean }) {
  const f = texts.inventory.fields
  return (
    <>
      <TextField
        label={f.code}
        required
        value={m.v.code}
        maxLength={30}
        disabled={readOnly}
        error={m.errors.code}
        onChange={(e) => m.setV((s) => ({ ...s, code: normalizeCode(e.target.value) }))}
      />
      <TextField label={f.name} required value={m.v.name} maxLength={200} disabled={readOnly} error={m.errors.name} onChange={(e) => m.setV((s) => ({ ...s, name: e.target.value }))} />
      {isEdit && <StatusSelect value={m.v.status} disabled={readOnly} onChange={(st) => m.setV((s) => ({ ...s, status: st }))} />}
    </>
  )
}

type FormProps<T> = { apiPath: string; listHref: string; canManage: boolean; item?: T }

function MasterShell({
  form,
  readOnly,
  canManage,
  listHref,
  canDelete,
  onDelete,
  onSubmit,
  children,
}: {
  form: ReturnType<typeof useResourceForm>
  readOnly: boolean
  canManage: boolean
  listHref: string
  canDelete: boolean
  onDelete: () => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
  children: ReactNode
}) {
  return (
    <form noValidate onSubmit={onSubmit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      {children}
      <FormActions listHref={listHref} canManage={canManage} canDelete={canDelete} busy={form.busy} onDelete={onDelete} />
    </form>
  )
}

// ---------------------------------------------------------------- หน่วยนับ

export function UomForm({ apiPath, listHref, canManage, item }: FormProps<Uom>) {
  const t = texts.inventory.uoms
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const m = useMaster(item)
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = m.check()
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: m.v.code, name: m.v.name.trim(), status: m.v.status })
  }
  return (
    <MasterShell form={form} readOnly={readOnly} canManage={canManage} listHref={listHref} canDelete={!!item} onDelete={() => remove(form, t, item)} onSubmit={submit}>
      <FormSection title={texts.inventory.sections.general} description={t.description}>
        <MasterFields m={m} readOnly={readOnly} isEdit={!!item} />
      </FormSection>
    </MasterShell>
  )
}

function remove(form: ReturnType<typeof useResourceForm>, t: Copy, item?: { code: string }) {
  form.remove(format(t.deleteConfirm, { code: item?.code ?? '' }))
}

// ---------------------------------------------------------------- บัญชี (หมวด / สินค้า)

type AccountRefs = { purchase: Ref | null; inventory: Ref | null; revenue: Ref | null }

function AccountFields({ accountsApi, value, onChange, disabled }: { accountsApi: string; value: AccountRefs; onChange: (v: AccountRefs) => void; disabled: boolean }) {
  const f = texts.inventory.fields
  const pick = (k: keyof AccountRefs, label: string, type: string) => (
    <ReferenceSelect
      label={label}
      source={accountsApi}
      params={{ postable: 'true', status: 'active', type }}
      value={value[k]}
      onChange={(r) => onChange({ ...value, [k]: r })}
      disabled={disabled}
      texts={texts.reference}
    />
  )
  return (
    <FormSection title={texts.inventory.sections.accounts} description={f.accountsHint}>
      {pick('purchase', f.purchaseAccount, 'expense')}
      {pick('inventory', f.inventoryAccount, 'asset')}
      {pick('revenue', f.revenueAccount, 'revenue')}
    </FormSection>
  )
}

const accountBody = (a: AccountRefs) => ({
  purchase_account_id: orNull(a.purchase?.id),
  inventory_account_id: orNull(a.inventory?.id),
  revenue_account_id: orNull(a.revenue?.id),
})

// ---------------------------------------------------------------- หมวดสินค้า

export function CategoryForm({ apiPath, accountsApi, listHref, canManage, item, accounts }: FormProps<Category> & { accountsApi: string; accounts?: AccountRefs }) {
  const t = texts.inventory.categories
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const m = useMaster(item)
  const [acc, setAcc] = useState<AccountRefs>(accounts ?? { purchase: null, inventory: null, revenue: null })
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = m.check()
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: m.v.code, name: m.v.name.trim(), status: m.v.status, ...accountBody(acc) })
  }
  return (
    <MasterShell form={form} readOnly={readOnly} canManage={canManage} listHref={listHref} canDelete={!!item} onDelete={() => remove(form, t, item)} onSubmit={submit}>
      <FormSection title={texts.inventory.sections.general} description={t.description}>
        <MasterFields m={m} readOnly={readOnly} isEdit={!!item} />
      </FormSection>
      <AccountFields accountsApi={accountsApi} value={acc} onChange={setAcc} disabled={readOnly} />
    </MasterShell>
  )
}

// ---------------------------------------------------------------- คลัง

export function WarehouseForm({ apiPath, branchesApi, listHref, canManage, item, branch }: FormProps<Warehouse> & { branchesApi: string; branch?: Ref | null }) {
  const t = texts.inventory.warehouses
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const m = useMaster(item)
  const [br, setBr] = useState<Ref | null>(branch ?? null)
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = m.check()
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: m.v.code, name: m.v.name.trim(), status: m.v.status, branch_id: orNull(br?.id) })
  }
  return (
    <MasterShell form={form} readOnly={readOnly} canManage={canManage} listHref={listHref} canDelete={!!item} onDelete={() => remove(form, t, item)} onSubmit={submit}>
      <FormSection title={texts.inventory.sections.general} description={t.description}>
        <MasterFields m={m} readOnly={readOnly} isEdit={!!item} />
        <ReferenceSelect
          label={texts.inventory.fields.branch}
          source={branchesApi}
          params={{ status: 'active' }}
          value={br}
          onChange={setBr}
          disabled={readOnly}
          texts={texts.reference}
        />
      </FormSection>
    </MasterShell>
  )
}

// ---------------------------------------------------------------- สินค้า

export function ItemForm({
  apiPath,
  uomsApi,
  categoriesApi,
  accountsApi,
  listHref,
  canManage,
  item,
  accounts,
}: FormProps<Item> & { uomsApi: string; categoriesApi: string; accountsApi: string; accounts?: AccountRefs }) {
  const t = texts.inventory.items
  const f = texts.inventory.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const m = useMaster(item)
  const [extra, setExtra] = useState({ name_en: item?.name_en ?? '', type: (item?.type ?? 'stock') as ItemType, reorder: item?.reorder_point ?? '' })
  const [uom, setUom] = useState<Ref | null>(item?.uom ?? null)
  const [category, setCategory] = useState<Ref | null>(item?.category ?? null)
  const [acc, setAcc] = useState<AccountRefs>(accounts ?? { purchase: null, inventory: null, revenue: null })
  const [errors, setErrors] = useState<FieldErrors<'uom' | 'reorder'>>({})
  const stock = extra.type === 'stock'

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = m.check()
    const own: typeof errors = {}
    if (!uom) own.uom = requiredMessage(f.uom)
    const reorder = stock && extra.reorder.trim() ? normalizeAmount(extra.reorder) : null
    if (stock && extra.reorder.trim() && (reorder === null || toUnits(reorder) < 0n)) own.reorder = f.reorderHint
    setErrors(own)
    if (readOnly) return
    if (Object.keys(errs).length || Object.keys(own).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: m.v.code,
      name: m.v.name.trim(),
      name_en: extra.name_en.trim(),
      type: extra.type,
      uom_id: uom!.id,
      category_id: orNull(category?.id),
      reorder_point: reorder,
      status: m.v.status,
      ...accountBody(acc),
    })
  }

  return (
    <MasterShell form={form} readOnly={readOnly} canManage={canManage} listHref={listHref} canDelete={!!item} onDelete={() => remove(form, t, item)} onSubmit={submit}>
      <FormSection title={texts.inventory.sections.general} description={t.description}>
        <MasterFields m={m} readOnly={readOnly} isEdit={!!item} />
        <TextField label={f.nameEn} value={extra.name_en} maxLength={200} disabled={readOnly} onChange={(e) => setExtra((s) => ({ ...s, name_en: e.target.value }))} />
        <SelectField
          label={f.type}
          value={extra.type}
          disabled={readOnly}
          options={Object.entries(texts.inventory.itemTypes).map(([value, label]) => ({ value, label }))}
          onChange={(e) => setExtra((s) => ({ ...s, type: e.target.value as ItemType }))}
        />
        <ReferenceSelect
          label={f.uom}
          required
          source={uomsApi}
          params={{ status: 'active' }}
          value={uom}
          onChange={setUom}
          error={errors.uom}
          disabled={readOnly}
          texts={texts.reference}
        />
        <ReferenceSelect label={f.category} source={categoriesApi} params={{ status: 'active' }} value={category} onChange={setCategory} disabled={readOnly} texts={texts.reference} />
      </FormSection>
      {stock && (
        <FormSection title={texts.inventory.sections.stock}>
          <TextField
            label={f.reorderPoint}
            hint={f.reorderHint}
            inputMode="decimal"
            value={extra.reorder}
            disabled={readOnly}
            error={errors.reorder}
            className="[&_input]:text-right [&_input]:tabular-nums"
            onChange={(e) => setExtra((s) => ({ ...s, reorder: e.target.value }))}
          />
        </FormSection>
      )}
      <AccountFields accountsApi={accountsApi} value={acc} onChange={setAcc} disabled={readOnly} />
    </MasterShell>
  )
}

export type { AccountRefs }
