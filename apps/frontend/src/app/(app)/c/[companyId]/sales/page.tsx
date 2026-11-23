import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function SalesIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'sales')
}
