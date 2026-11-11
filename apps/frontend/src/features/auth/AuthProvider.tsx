import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { Session } from './api'
import { clearSession, isExpired, loadSession, saveSession } from './session'

type AuthContextValue = {
  session: Session | null
  signIn: (session: Session) => void
  signOut: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

/** เก็บสถานะการเข้าสู่ระบบของทั้งแอป — ออกจากระบบอัตโนมัติเมื่อ token หมดอายุ */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(loadSession)

  const signIn = useCallback((s: Session) => {
    saveSession(s)
    setSession(s)
  }, [])

  const signOut = useCallback(() => {
    clearSession()
    setSession(null)
  }, [])

  useEffect(() => {
    if (!session) return
    if (isExpired(session)) {
      signOut()
      return
    }
    const ms = Math.min(Date.parse(session.expiresAt) - Date.now(), 2_147_483_647)
    const timer = setTimeout(signOut, ms)
    return () => clearTimeout(timer)
  }, [session, signOut])

  const value = useMemo(() => ({ session, signIn, signOut }), [session, signIn, signOut])
  return <AuthContext value={value}>{children}</AuthContext>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>')
  return ctx
}
