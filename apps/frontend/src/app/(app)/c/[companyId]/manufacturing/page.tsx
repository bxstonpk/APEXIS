import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function ManufacturingIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'manufacturing')
}
