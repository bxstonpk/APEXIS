'use client'

import { useState, type FormEvent } from 'react'
import { Alert, Button, PasswordField } from '@/components'
import { appConfig, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage, type FieldErrors } from '@/lib/validation'

type Field = 'old' | 'next' | 'confirm'
const MIN = 8

/**
 * ตั้งรหัสผ่านใหม่ (บัญชีที่ผู้ดูแลสร้างให้ใช้รหัสชั่วคราว) — สำเร็จแล้วไปหน้าเดิม (?redirect=) หรือหน้าหลัก
 * autocomplete current-password / new-password ให้ password manager เติมและบันทึกรหัสใหม่ได้ (วางรหัสผ่านได้)
 */
export function ChangePasswordForm({ redirectTo, email }: { redirectTo: string; email: string }) {
  const t = texts.auth.changePassword
  const [v, setV] = useState({ old: '', next: '', confirm: '' })
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const show = { showLabel: texts.auth.login.showPassword, hideLabel: texts.auth.login.hidePassword }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: FieldErrors<Field> = {}
    if (!v.old) errs.old = requiredMessage(t.oldPassword)
    if (!v.next) errs.next = requiredMessage(t.newPassword)
    else if (v.next.length < MIN) errs.next = t.tooShort
    else if (v.next === v.old) errs.next = t.sameAsOld
    if (!errs.next && v.confirm !== v.next) errs.confirm = t.mismatch
    setErrors(errs)
    setFormError(null)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    setBusy(true)
    try {
      await apiRequest(appConfig.api.sessionPassword, { method: 'PUT', baseUrl: '', body: { old_password: v.old, new_password: v.next } })
      showToast(t.success)
      // โหลดหน้าใหม่ทั้งหน้า: router ของ Next จำไว้ว่าหน้าอื่น redirect มาที่นี่ (ก่อนเปลี่ยนรหัส) — ต้องล้างแคชนั้น
      window.location.assign(redirectTo)
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form method="post" noValidate onSubmit={submit} className="flex flex-col gap-5">
      {formError && <Alert tone="error">{formError}</Alert>}
      {/* ชื่อผู้ใช้ (ซ่อน) ให้ password manager รู้ว่ารหัสใหม่เป็นของบัญชีไหน */}
      <input type="email" name="username" autoComplete="username" value={email} readOnly hidden />
      <PasswordField
        label={t.oldPassword}
        required
        autoComplete="current-password"
        value={v.old}
        error={errors.old}
        onChange={(e) => setV((s) => ({ ...s, old: e.target.value }))}
        {...show}
      />
      <PasswordField
        label={t.newPassword}
        required        autoComplete="new-password"
        value={v.next}
        error={errors.next}
        onChange={(e) => setV((s) => ({ ...s, next: e.target.value }))}
        {...show}
      />
      <PasswordField
        label={t.confirmPassword}
        required
        autoComplete="new-password"
        value={v.confirm}
        error={errors.confirm}
        onChange={(e) => setV((s) => ({ ...s, confirm: e.target.value }))}
        {...show}
      />
      <Button type="submit" fullWidth loading={busy} loadingText={t.submitting} className="mt-1">
        {t.submit}
      </Button>
    </form>
  )
}
