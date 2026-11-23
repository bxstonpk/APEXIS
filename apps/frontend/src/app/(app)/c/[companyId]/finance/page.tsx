import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function FinanceIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'finance')
}
