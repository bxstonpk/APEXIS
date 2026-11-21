'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, TextField } from '@/components'
import { appConfig, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { apiRequest, errorMessage } from '@/lib/http'
import { normalizeCode, patterns, requiredMessage, type FieldErrors } from '@/lib/validation'

type Field = 'code' | 'name' | 'taxId'

/** ฟอร์มสร้างบริษัท (POST /companies) — รหัส + ชื่อ + เลขผู้เสียภาษี; รายละเอียดอื่นแก้ภายหลัง */
export function CompanyForm() {
  const router = useRouter()
  const t = texts.companies.new
  const [values, setValues] = useState({ code: '', name: '', taxId: '' })
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  function validate(): FieldErrors<Field> {
    const e: FieldErrors<Field> = {}
    if (!values.code) e.code = requiredMessage(t.code)
    else if (!patterns.code.test(values.code)) e.code = texts.validation.code
    if (!values.name.trim()) e.name = requiredMessage(t.name)
    return e
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const e = validate()
    setErrors(e)
    setFormError(null)
    if (Object.keys(e).length > 0) return
    setBusy(true)
    try {
      const company = await apiRequest<{ id: string }>('/companies', {
        method: 'POST',
        body: { code: values.code, name: values.name.trim(), tax_id: values.taxId.trim() },
      })
      router.push(companyPath(company.id))
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
      {formError && <Alert tone="error">{formError}</Alert>}
      <TextField
        label={t.code}
        hint={t.codeHint}
        value={values.code}
        maxLength={20}
        autoComplete="off"
        error={errors.code}
        onChange={(e) => setValues((v) => ({ ...v, code: normalizeCode(e.target.value) }))}
      />
      <TextField
        label={t.name}
        value={values.name}
        maxLength={200}
        error={errors.name}
        onChange={(e) => setValues((v) => ({ ...v, name: e.target.value }))}
      />
      <TextField
        label={t.taxId}
        value={values.taxId}
        maxLength={20}
        inputMode="numeric"
        onChange={(e) => setValues((v) => ({ ...v, taxId: e.target.value }))}
      />
      <div className="mt-2 flex justify-end gap-2">
        <ButtonLink href={appConfig.routes.companies} variant="secondary">
          {texts.common.cancel}
        </ButtonLink>
        <Button type="submit" loading={busy} loadingText={texts.common.saving}>
          {t.submit}
        </Button>
      </div>
    </form>
  )
}
