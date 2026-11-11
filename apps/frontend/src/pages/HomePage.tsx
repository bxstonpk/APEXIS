import { Button, Card } from '@/components'
import { format, texts } from '@/config'
import { useAuth } from '@/features/auth'
import { AuthLayout } from '@/layouts/AuthLayout'
import { useDocumentTitle } from '@/lib/useDocumentTitle'

/** หน้าหลักชั่วคราวหลังเข้าสู่ระบบ — จะถูกแทนด้วย layout ของแอปเมื่อมีเมนู / หน้าอื่น */
export function HomePage() {
  useDocumentTitle(texts.home.pageTitle)
  const { session, signOut } = useAuth()
  if (!session) return null
  const { user } = session

  return (
    <AuthLayout>
      <Card
        title={format(texts.home.welcome, { name: user.name })}
        description={format(texts.home.description, { email: user.email })}
      >
        <Button variant="secondary" fullWidth onClick={signOut}>
          {texts.auth.logout}
        </Button>
      </Card>
    </AuthLayout>
  )
}
