import { appConfig } from '@/config'
import { apiRequest } from '@/lib/http'

export type User = {
  id: string
  email: string
  name: string
  status: string
}

/** ข้อมูลการเข้าสู่ระบบที่แอปเก็บไว้ */
export type Session = {
  accessToken: string
  expiresAt: string // ISO 8601
  user: User
}

type TokenResponse = {
  access_token: string
  token_type: string
  expires_at: string
  user: User
}

/** POST /auth/tokens — ล็อกอินแล้วได้ access token */
export async function login(email: string, password: string, signal?: AbortSignal): Promise<Session> {
  const t = await apiRequest<TokenResponse>(appConfig.api.endpoints.login, {
    method: 'POST',
    body: { email, password },
    signal,
  })
  return { accessToken: t.access_token, expiresAt: t.expires_at, user: t.user }
}
