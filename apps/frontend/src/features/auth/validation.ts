import { texts } from '@/config'

export type LoginValues = { email: string; password: string }
export type LoginErrors = Partial<Record<keyof LoginValues, string>>

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/**
 * ตรวจเบื้องต้นก่อนส่ง (ช่องว่าง / รูปแบบอีเมล) — ข้อความมาจาก config
 * การตรวจจริง (อีเมล / รหัสผ่านถูกต้องหรือไม่) อยู่ที่ backend
 */
export function validateLogin(values: LoginValues, t = texts.auth.validation): LoginErrors {
  const errors: LoginErrors = {}
  const email = values.email.trim()
  if (!email) errors.email = t.emailRequired
  else if (!emailPattern.test(email)) errors.email = t.emailInvalid
  if (!values.password) errors.password = t.passwordRequired
  return errors
}
