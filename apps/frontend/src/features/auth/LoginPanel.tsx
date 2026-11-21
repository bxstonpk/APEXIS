'use client'

import { useRouter } from 'next/navigation'
import { LoginForm } from './LoginForm'

/** ฟอร์มเข้าสู่ระบบของหน้า /login — สำเร็จแล้วไปหน้าเดิม (redirect ผ่านการตรวจ safeRedirect จาก server แล้ว) */
export function LoginPanel({ redirectTo }: { redirectTo: string }) {
  const router = useRouter()
  return (
    <LoginForm
      onSuccess={() => {
        router.replace(redirectTo)
        router.refresh()
      }}
    />
  )
}
