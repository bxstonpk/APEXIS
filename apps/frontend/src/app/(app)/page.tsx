import type { Metadata } from 'next'
import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { Card } from '@/components'
import { appConfig, format, texts } from '@/config'
import { LogoutButton } from '@/features/auth'
import { AuthLayout } from '@/layouts/AuthLayout'
import { SESSION_COOKIE, decodeSession } from '@/lib/server/session'

export const metadata: Metadata = { title: texts.home.pageTitle }

/** หน้าหลักชั่วคราวหลังเข้าสู่ระบบ — จะถูกแทนด้วยโครงหน้าระบบ (เมนู / เลือกบริษัท) ในขั้นถัดไป */
export default async function HomePage() {
  // proxy.ts กันหน้านี้แล้ว — ตรวจซ้ำเผื่อ cookie หมดอายุระหว่างทาง
  const session = decodeSession((await cookies()).get(SESSION_COOKIE)?.value)
  if (!session) redirect(appConfig.routes.login)
  const { user } = session

  return (
    <AuthLayout>
      <Card title={format(texts.home.welcome, { name: user.name })} description={format(texts.home.description, { email: user.email })}>
        <LogoutButton />
      </Card>
    </AuthLayout>
  )
}
