import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function AppIndexPage({ params }: { params: Promise<{ companyId: string; app: string }> }) {
  const { companyId, app } = await params
  return redirectToFirstMenu(companyId, app)
}
