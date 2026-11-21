'use client'

import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { LogOut } from 'lucide-react'
import { Button, type ButtonSize, type ButtonVariant } from '@/components'
import { appConfig, texts } from '@/config'
import { logout } from './api'

/** ปุ่มออกจากระบบ — ลบ cookie ที่ Next แล้วไปหน้า login (ลบไม่สำเร็จก็ยังพาไปหน้า login) */
export function LogoutButton({
  label = texts.auth.logout,
  fullWidth = true,
  size = 'md',
  variant = 'secondary',
}: {
  label?: string
  fullWidth?: boolean
  size?: ButtonSize
  variant?: ButtonVariant
}) {
  const router = useRouter()
  const [busy, setBusy] = useState(false)

  async function handleClick() {
    setBusy(true)
    try {
      await logout()
    } catch {
      // ไม่มี session แล้วก็ถือว่าออกจากระบบ
    } finally {
      router.replace(appConfig.routes.login)
      router.refresh()
    }
  }

  return (
    <Button variant={variant} size={size} fullWidth={fullWidth} loading={busy} onClick={handleClick}>
      <LogOut aria-hidden />
      {label}
    </Button>
  )
}
