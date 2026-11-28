import { redirectToFirstMenu } from '@/features/shell/appRoutes'

export default async function PettyCashIndexPage({ params }: { params: Promise<{ companyId: string }> }) {
  return redirectToFirstMenu((await params).companyId, 'petty-cash')
}
