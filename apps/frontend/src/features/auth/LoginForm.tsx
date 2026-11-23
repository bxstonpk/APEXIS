'use client'

import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, PasswordField, TextField } from '@/components'
import { texts as appTexts, type Texts } from '@/config'
import { errorMessage } from '@/lib/http'
import { login, type SignedIn } from './api'
import { validateLogin, type LoginErrors } from './validation'

export type LoginFormProps = {
  /** เรียกเมื่อเข้าสู่ระบบสำเร็จ (cookie ถูกตั้งแล้ว) — ผู้ใช้ component ตัดสินใจเองว่าจะเปลี่ยนหน้าอย่างไร */
  onSuccess: (result: SignedIn) => void
  /** ข้อความของฟอร์ม — ค่าเริ่มต้นมาจาก config; ส่งมาแทนได้เมื่อ embed ในที่อื่นที่ต้องการข้อความต่างไป */
  texts?: Texts['auth']['login']
  defaultEmail?: string
}

/**
 * ฟอร์มเข้าสู่ระบบ (อีเมล + รหัสผ่าน) — เป็น component อิสระ ไม่ผูกกับ router หรือ layout (client component)
 * จึง embed ได้ทั้งในหน้า login, modal, หรือหน้าอื่นที่ต้องให้เข้าสู่ระบบใหม่
 */
export function LoginForm({ onSuccess, texts: t = appTexts.auth.login, defaultEmail = '' }: LoginFormProps) {
  const [email, setEmail] = useState(defaultEmail)
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<LoginErrors>({})
  const [serverError, setServerError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const emailRef = useRef<HTMLInputElement>(null)
  const passwordRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => () => abortRef.current?.abort(), [])

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (submitting) return
    setServerError(null)

    const found = validateLogin({ email, password })
    setErrors(found)
    if (found.email) return emailRef.current?.focus()
    if (found.password) return passwordRef.current?.focus()

    setSubmitting(true)
    abortRef.current = new AbortController()
    try {
      const result = await login(email.trim(), password, abortRef.current.signal)
      onSuccess(result)
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      setServerError(errorMessage(err))
      setPassword('')
      passwordRef.current?.focus()
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form method="post" onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      {serverError && <Alert tone="error">{serverError}</Alert>}
      <TextField
        ref={emailRef}
        label={t.emailLabel}
        placeholder={t.emailPlaceholder}
        type="email"
        name="email"
        autoComplete="username"
        inputMode="email"
        autoFocus
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        error={errors.email}
        disabled={submitting}
      />
      <PasswordField
        ref={passwordRef}
        label={t.passwordLabel}
        placeholder={t.passwordPlaceholder}
        name="password"
        autoComplete="current-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        error={errors.password}
        disabled={submitting}
        showLabel={t.showPassword}
        hideLabel={t.hidePassword}
      />
      <Button type="submit" fullWidth loading={submitting} loadingText={t.submitting}>
        {t.submit}
      </Button>
    </form>
  )
}
