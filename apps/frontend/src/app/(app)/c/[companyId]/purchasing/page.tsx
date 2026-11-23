import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function PurchasingIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'purchasing')
}
