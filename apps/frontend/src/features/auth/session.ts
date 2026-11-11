import { appConfig } from '@/config'
import type { Session } from './api'

/**
 * เก็บ session ใน localStorage เพื่อให้ refresh หน้าแล้วยังอยู่ในระบบ
 * (ทุกการเข้าถึงอยู่ใน try/catch — บาง browser / โหมดส่วนตัวไม่อนุญาต storage)
 */
const key = appConfig.storage.session

export function isExpired(session: Session, now = Date.now()): boolean {
  const exp = Date.parse(session.expiresAt)
  return Number.isNaN(exp) || exp <= now
}

export function loadSession(): Session | null {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const s = JSON.parse(raw) as Session
    if (!s?.accessToken || isExpired(s)) {
      localStorage.removeItem(key)
      return null
    }
    return s
  } catch {
    return null
  }
}

export function saveSession(session: Session): void {
  try {
    localStorage.setItem(key, JSON.stringify(session))
  } catch {
    // storage ใช้ไม่ได้ — session อยู่ในหน่วยความจำจนปิดหน้า
  }
}

export function clearSession(): void {
  try {
    localStorage.removeItem(key)
  } catch {
    // ignore
  }
}
