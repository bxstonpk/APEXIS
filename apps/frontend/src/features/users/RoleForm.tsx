'use client'

import { useMemo, useState, type FormEvent } from 'react'
import { Badge, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormActions, FormNotices, FormSection } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { focusFirstInvalid, requiredMessage, type FieldErrors } from '@/lib/validation'
import { buildMatrix, expandGranted, toList } from './matrix'
import { PermissionMatrix } from './PermissionMatrix'
import type { PermissionEntry, Role } from './types'

const CODE = /^[a-z][a-z0-9_-]{1,49}$/

/**
 * สร้าง / แก้ไขบทบาท — ข้อมูลบทบาท + ตารางสิทธิ์
 * - บทบาทของระบบ (owner) แสดงแบบอ่านอย่างเดียว ทุกช่องถูกเลือก
 * - สิทธิ์แบบ wildcard ของบทบาทเดิม (เช่น sales.order:*) ถูกแสดงเป็นช่องที่ครอบคลุม และบันทึกกลับเป็นรายการเต็ม
 */
export function RoleForm({
  apiPath,
  listHref,
  catalog,
  role,
  canManage,
}: {
  apiPath: string
  listHref: string
  catalog: PermissionEntry[]
  role?: Role
  canManage: boolean
}) {
  const t = texts.roles
  const system = role?.is_system ?? false
  const readOnly = !canManage || system
  const groups = useMemo(() => buildMatrix(catalog), [catalog])
  const form = useResourceForm({ apiPath, id: role?.id, listHref })
  const [v, setV] = useState({ code: role?.code ?? '', name: role?.name ?? '', description: role?.description ?? '' })
  const [selected, setSelected] = useState<Set<string>>(() => expandGranted(role?.permissions ?? [], catalog))
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'permissions'>>({})

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    const errs: typeof errors = {}
    if (!role) {
      if (!v.code) errs.code = requiredMessage(t.fields.code)
      else if (!CODE.test(v.code)) errs.code = t.invalidCode
    }
    if (!v.name.trim()) errs.name = requiredMessage(t.fields.name)
    if (selected.size === 0) errs.permissions = t.noPermission
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    const permissions = toList(selected, catalog)
    form.save(
      role
        ? { name: v.name.trim(), description: v.description.trim(), permissions }
        : { code: v.code, name: v.name.trim(), description: v.description.trim(), permissions },
    )
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={!canManage} />

      <FormSection title={t.sections.general} aside={system ? <Badge tone="info">{t.system}</Badge> : undefined}>
        <TextField
          label={t.fields.code}
          required={!role}
          value={v.code}
          maxLength={50}
          autoComplete="off"
          disabled={!!role || readOnly}
          error={errors.code}
          onChange={(e) => setV((s) => ({ ...s, code: e.target.value.toLowerCase().replace(/\s+/g, '-') }))}
        />
        <TextField label={t.fields.name} required value={v.name} maxLength={100} disabled={readOnly} error={errors.name} onChange={(e) => setV((s) => ({ ...s, name: e.target.value }))} />
        <TextField
          label={t.fields.description}
          value={v.description}
          maxLength={500}
          disabled={readOnly}
          className="sm:col-span-2"
          onChange={(e) => setV((s) => ({ ...s, description: e.target.value }))}
        />
      </FormSection>

      <section className="grid gap-4 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-10">
        <div>
          <h2 className="text-[15px] font-semibold">{t.sections.permissions}</h2>          <p className="mt-3 text-[13px] font-medium text-ink tabular-nums" aria-live="polite">
            {format(t.permissionCount, { n: selected.size })}
          </p>
          {errors.permissions && (
            <p role="alert" className="mt-2 text-[13px] text-danger">
              {errors.permissions}
            </p>
          )}
        </div>
        <PermissionMatrix groups={groups} selected={selected} onChange={setSelected} disabled={readOnly} />
      </section>

      <FormActions
        listHref={listHref}
        canManage={canManage && !system}
        canDelete={!!role && !system}
        busy={form.busy}
        onDelete={() => form.remove(format(t.deleteConfirm, { name: role?.name ?? '' }))}
      />
    </form>
  )
}
