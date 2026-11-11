import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { appConfig } from '@/config'
import { useAuth } from './AuthProvider'

/**
 * ครอบหน้าที่ต้องเข้าสู่ระบบ — ยังไม่เข้าสู่ระบบจะไปหน้า login พร้อม ?redirect=<หน้าเดิม รวม URL query>
 * หลังเข้าสู่ระบบจึงกลับมาหน้าเดิม (รวมเงื่อนไขค้นหาเดิม) ได้
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { session } = useAuth()
  const location = useLocation()
  if (!session) {
    const params = new URLSearchParams({ [appConfig.redirectParam]: location.pathname + location.search })
    return <Navigate to={`${appConfig.routes.login}?${params}`} replace />
  }
  return children
}

/** รับเฉพาะ path ภายในแอป (กัน open redirect ไปเว็บอื่น เช่น //evil.com หรือ https://...) */
export function safeRedirect(target: string | null): string {
  if (!target || !target.startsWith('/') || target.startsWith('//') || target.startsWith('/\\')) {
    return appConfig.routes.home
  }
  return target
}
