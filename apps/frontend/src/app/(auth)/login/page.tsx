import type { Metadata } from 'next'
import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { appConfig, texts } from '@/config'
import { LoginPanel } from '@/features/auth'
import { AuthLayout } from '@/layouts/AuthLayout'
import { SESSION_COOKIE, decodeSession, safeRedirect } from '@/lib/server/session'

export const metadata: Metadata = { title: texts.auth.login.pageTitle }

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> }

/** หน้าเข้าสู่ระบบ — เข้าสู่ระบบอยู่แล้วไปหน้าเดิมทันที; สำเร็จแล้วกลับไปหน้าที่ระบุใน ?redirect= (ถ้าไม่มีไปหน้าหลัก) */
export default async function LoginPage({ searchParams }: Props) {
  const raw = (await searchParams)[appConfig.redirectParam]
  const target = safeRedirect(typeof raw === 'string' ? raw : null)
  if (decodeSession((await cookies()).get(SESSION_COOKIE)?.value)) redirect(target)

  const t = texts.auth.login
  return (
    <AuthLayout>
      <h1 className="mb-8 text-2xl font-semibold tracking-tight">{t.heading}</h1>
      <LoginPanel redirectTo={target} />
    </AuthLayout>
  )
}
