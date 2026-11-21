import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function UsersIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'users')
}
