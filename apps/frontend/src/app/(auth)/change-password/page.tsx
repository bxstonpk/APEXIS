import { KeyRound } from 'lucide-react'
import type { Metadata } from 'next'
import { texts } from '@/config'
import { ChangePasswordForm } from '@/features/auth/ChangePasswordForm'
import { AuthLayout } from '@/layouts/AuthLayout'
import { currentUser } from '@/lib/server/backend'
import { safeRedirect } from '@/lib/server/session'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.auth.changePassword.pageTitle }

/** ตั้งรหัสผ่านใหม่ — proxy.ts พามาที่นี่เมื่อบัญชียังใช้รหัสผ่านชั่วคราว (ต้องเข้าสู่ระบบก่อน) */
export default async function ChangePasswordPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const user = await currentUser()
  const target = safeRedirect(param(await searchParams, 'redirect') || null)
  const t = texts.auth.changePassword
  return (
    <AuthLayout>
      <span className="mb-4 flex size-11 items-center justify-center rounded-full bg-primary-soft text-primary">
        <KeyRound aria-hidden className="size-5" />
      </span>
      <h1 className="text-2xl font-semibold tracking-tight">{t.heading}</h1>
      <p className="mt-1.5 text-sm text-ink-muted">{user.email}</p>
      <p className="mt-3 mb-8 text-sm text-ink-muted">{t.forcedDescription}</p>
      <ChangePasswordForm redirectTo={target} email={user.email} />
    </AuthLayout>
  )
}
