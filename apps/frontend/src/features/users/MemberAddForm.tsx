'use client'

import { Copy, Dices } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormActions, FormSection } from '@/features/forms/FormParts'
import { ApiError, apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage, type FieldErrors } from '@/lib/validation'
import { RoleChecklist } from './RoleChecklist'
import type { Role } from './types'

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
// ตัดตัวอักษรที่สับสนง่าย (0 O o 1 l I) — ผู้ดูแลต้องบอกต่อให้ผู้ใช้
const ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'

/** รหัสผ่านชั่วคราวแบบสุ่ม (crypto) 12 ตัว */
export function generateTempPassword(length = 12): string {
  const bytes = new Uint32Array(length)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => ALPHABET[b % ALPHABET.length]).join('')
}

type Field = 'email' | 'roles' | 'name' | 'password'

/**
 * เพิ่มสมาชิก: อีเมล + บทบาท
 * - มีบัญชีแล้ว → เพิ่มทันที
 * - ไม่พบบัญชี (404 user_not_found) → เปิดส่วน "สร้างบัญชีใหม่" (ชื่อ + รหัสผ่านชั่วคราว) แล้วส่งอีกครั้งพร้อม new_user
 *   ผู้ใช้ต้องเปลี่ยนรหัสผ่านเมื่อเข้าสู่ระบบครั้งแรก
 */
export function MemberAddForm({ apiPath, listHref, roles }: { apiPath: string; listHref: string; roles: Role[] }) {
  const t = texts.members
  const router = useRouter()
  const nameRef = useRef<HTMLInputElement>(null)
  const [email, setEmail] = useState('')
  const [roleIds, setRoleIds] = useState<Set<string>>(new Set())
  const [newAccount, setNewAccount] = useState<{ email: string } | null>(null)
  const [account, setAccount] = useState({ name: '', password: '' })
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  function changeEmail(value: string) {
    setEmail(value)
    // เปลี่ยนอีเมลแล้ว — ต้องตรวจใหม่ว่ามีบัญชีหรือไม่
    if (newAccount && value.trim().toLowerCase() !== newAccount.email) setNewAccount(null)
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = e.currentTarget
    const errs: FieldErrors<Field> = {}
    const mail = email.trim().toLowerCase()
    if (!mail) errs.email = requiredMessage(t.fields.email)
    else if (!EMAIL.test(mail)) errs.email = texts.validation.email
    if (roleIds.size === 0) errs.roles = t.fields.rolesRequired
    if (newAccount) {
      if (!account.name.trim()) errs.name = requiredMessage(t.newAccount.name)
      if (account.password.length < 8) errs.password = texts.auth.changePassword.tooShort
    }
    setErrors(errs)
    setFormError(null)
    if (Object.keys(errs).length) return focusFirstInvalid(form)

    setBusy(true)
    try {
      await apiRequest(apiPath, {
        method: 'POST',
        body: {
          email: mail,
          role_ids: [...roleIds],
          ...(newAccount ? { new_user: { name: account.name.trim(), password: account.password } } : {}),
        },
      })
      showToast(format(t.added, { name: newAccount ? account.name.trim() : mail }))
      router.push(listHref)
      router.refresh()
    } catch (err) {
      setBusy(false)
      if (err instanceof ApiError && err.code === 'user_not_found' && !newAccount) {
        setNewAccount({ email: mail })
        requestAnimationFrame(() => nameRef.current?.focus())
        return
      }
      setFormError(errorMessage(err))
    }
  }

  async function copyPassword() {
    try {
      await navigator.clipboard.writeText(account.password)
      showToast(t.newAccount.copied)
    } catch {
      // clipboard ถูกปิด — ผู้ใช้คัดลอกจากช่องเองได้
    }
  }

  return (
    <form method="post" noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}

      <FormSection title={t.sections.account} description={t.sections.accountHint}>
        <TextField
          label={t.fields.email}
          required
          type="email"
          autoComplete="off"
          hint={t.fields.emailHint}
          value={email}
          maxLength={320}
          error={errors.email}
          onChange={(e) => changeEmail(e.target.value)}
          className="sm:col-span-2"
        />
        {newAccount && (
          <>
            <div className="sm:col-span-2">
              <Alert>{format(t.newAccount.notFound, { email: newAccount.email })}</Alert>
            </div>
            <TextField
              ref={nameRef}
              label={t.newAccount.name}
              required
              autoComplete="off"
              value={account.name}
              maxLength={200}
              error={errors.name}
              onChange={(e) => setAccount((a) => ({ ...a, name: e.target.value }))}
            />
            <div className="flex flex-col gap-2">
              <TextField
                label={t.newAccount.password}
                required
                autoComplete="new-password"
                hint={t.newAccount.passwordHint}
                value={account.password}
                maxLength={72}
                error={errors.password}
                className="font-mono"
                onChange={(e) => setAccount((a) => ({ ...a, password: e.target.value }))}
              />
              <div className="flex gap-2">
                <Button size="sm" variant="secondary" onClick={() => setAccount((a) => ({ ...a, password: generateTempPassword() }))}>
                  <Dices aria-hidden />
                  {t.newAccount.generate}
                </Button>
                <Button size="sm" variant="ghost" disabled={!account.password} onClick={copyPassword}>
                  <Copy aria-hidden />
                  {t.newAccount.copy}
                </Button>
              </div>
            </div>
          </>
        )}
      </FormSection>

      <FormSection title={t.sections.roles} description={t.sections.rolesHint}>
        <RoleChecklist roles={roles} selected={roleIds} onChange={setRoleIds} error={errors.roles} />
      </FormSection>

      <FormActions listHref={listHref} canManage canDelete={false} busy={busy ? 'save' : null} />
    </form>
  )
}
