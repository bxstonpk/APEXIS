import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function AccountingIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'accounting')
}
