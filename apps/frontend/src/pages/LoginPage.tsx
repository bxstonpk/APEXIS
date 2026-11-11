import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { Card } from '@/components'
import { appConfig, texts } from '@/config'
import { LoginForm, safeRedirect, useAuth, type Session } from '@/features/auth'
import { AuthLayout } from '@/layouts/AuthLayout'
import { useDocumentTitle } from '@/lib/useDocumentTitle'

/** หน้าเข้าสู่ระบบ — หลังสำเร็จกลับไปหน้าที่ระบุใน ?redirect= (ถ้าไม่มีไปหน้าหลัก) */
export function LoginPage() {
  const t = texts.auth.login
  useDocumentTitle(t.pageTitle)
  const { session, signIn } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const target = safeRedirect(params.get(appConfig.redirectParam))

  if (session) return <Navigate to={target} replace />

  function handleSuccess(s: Session) {
    signIn(s)
    navigate(target, { replace: true })
  }

  return (
    <AuthLayout>
      <Card title={t.heading} description={t.description}>
        <LoginForm onSuccess={handleSuccess} />
      </Card>
    </AuthLayout>
  )
}
