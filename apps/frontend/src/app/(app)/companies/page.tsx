import { ChevronRight, Search } from 'lucide-react'
import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import { Alert, Badge, Button, ButtonLink, PageHeader, Pagination, TextField } from '@/components'
import { appConfig, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { TopBar } from '@/features/shell'
import { errorMessage } from '@/lib/http'
import { backendGet, currentUser, type ListMeta } from '@/lib/server/backend'
import type { CompanySummary } from '@/lib/server/company'
import { param, pickQuery, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.companies.pageTitle }

const QUERY_KEYS = ['q', 'page'] as const

/** เลือกบริษัท — รายการบริษัทที่เป็นสมาชิก ค้นหาผ่าน ?q= (กด Back กลับมาได้เงื่อนไขเดิม) */
export default async function CompaniesPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const params = await searchParams
  const query = pickQuery(params, QUERY_KEYS)
  const user = await currentUser()
  const t = texts.companies

  let rows: CompanySummary[] = []
  let meta: ListMeta = { page: 1, page_size: appConfig.pageSize, total: 0 }
  let error: string | null = null
  try {
    const res = await backendGet<CompanySummary[]>('/me/companies', withPageSize(query))
    rows = res.data
    meta = res.meta ?? meta
  } catch (err) {
    error = errorMessage(err)
  }
  const q = param(params, 'q')
  const path = appConfig.routes.companies

  return (
    <div className="flex min-h-full flex-col">
      <TopBar userName={user.name} />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-8">
        <PageHeader
          title={t.heading}
          description={t.description}
          actions={<ButtonLink href={appConfig.routes.newCompany}>{t.create}</ButtonLink>}
        />

        <Form action="" className="mb-4 flex gap-2">
          <TextField
            name="q"
            type="search"
            label={texts.common.search}
            placeholder={t.searchPlaceholder}
            defaultValue={q}
            className="flex-1 [&>label]:sr-only"
          />
          <Button type="submit" variant="secondary" aria-label={texts.common.search} className="self-end px-4">
            <Search aria-hidden className="size-5" />
          </Button>
        </Form>

        {error && <Alert tone="error">{error}</Alert>}

        {!error && rows.length === 0 && (
          <p className="rounded-xl border border-line bg-surface px-4 py-10 text-center text-ink-muted">
            {q ? texts.common.empty : t.empty}
          </p>
        )}

        <ul className="flex flex-col gap-2">
          {rows.map((c) => (
            <li key={c.id}>
              <Link
                href={companyPath(c.id)}
                className="flex items-center gap-4 rounded-xl border border-line bg-surface px-4 py-3 hover:border-primary"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">{c.name}</span>
                  <span className="text-sm text-ink-muted">{c.code}</span>
                </span>
                {c.status !== 'active' && <Badge>{t.inactive}</Badge>}
                <ChevronRight aria-hidden className="size-5 text-ink-muted" />
              </Link>
            </li>
          ))}
        </ul>

        {meta.total > meta.page_size && (
          <Pagination
            page={meta.page}
            pageSize={meta.page_size}
            total={meta.total}
            href={(page) => withQuery(path, query, { page })}
            labels={texts.common.pagination}
          />
        )}
      </main>
    </div>
  )
}

function withPageSize(query: URLSearchParams): URLSearchParams {
  const q = new URLSearchParams(query)
  q.set('page_size', String(appConfig.pageSize))
  return q
}
