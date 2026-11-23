import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function InventoryIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'inventory')
}
