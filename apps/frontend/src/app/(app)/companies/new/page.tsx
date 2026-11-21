import type { Metadata } from 'next'
import { Card } from '@/components'
import { texts } from '@/config'
import { CompanyForm } from '@/features/companies/CompanyForm'
import { TopBar } from '@/features/shell'
import { currentUser } from '@/lib/server/backend'

export const metadata: Metadata = { title: texts.companies.new.pageTitle }

/** สร้างบริษัท — ผู้สร้างเป็น owner; สร้างเสร็จเข้าหน้ารวมแอปของบริษัทใหม่ */
export default async function NewCompanyPage() {
  const user = await currentUser()
  const t = texts.companies.new
  return (
    <div className="flex min-h-full flex-col">
      <TopBar userName={user.name} />
      <main className="mx-auto w-full max-w-lg flex-1 px-4 py-8">
        <Card title={t.heading} description={t.description}>
          <CompanyForm />
        </Card>
      </main>
    </div>
  )
}
