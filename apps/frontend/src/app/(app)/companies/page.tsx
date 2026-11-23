import { ArrowRight, Building2, Plus, Search } from 'lucide-react'
import type { Metadata } from 'next'
import { cookies } from 'next/headers'
import Form from 'next/form'
import Link from 'next/link'
import { Alert, Badge, Button, ButtonLink, Pagination, TextField } from '@/components'
import { appConfig, format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { TopBar } from '@/features/shell'
import { cx } from '@/lib/cx'
import { errorMessage } from '@/lib/http'
import { backendGet, currentUser, type ListMeta } from '@/lib/server/backend'
import type { CompanySummary } from '@/lib/server/company'
import { param, pickQuery, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.companies.pageTitle }

const QUERY_KEYS = ['q', 'page'] as const

/** สีประจำบริษัท (คงที่ตามรหัส) — ใช้ชุดสีเดียวกับไอคอนแอป */
const AVATAR_TONES = ['bg-app-purchasing', 'bg-app-accounting', 'bg-app-organization', 'bg-app-finance', 'bg-app-inventory', 'bg-app-sales'] as const

function avatarTone(code: string): string {
  let h = 0
  for (const ch of code) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  return AVATAR_TONES[h % AVATAR_TONES.length]!
}

/**
 * เลือกบริษัท — การ์ดบริษัทที่เป็นสมาชิก (บริษัทที่ใช้ล่าสุดขึ้นก่อน) + การ์ดสร้างบริษัท
 * ค้นหาผ่าน ?q= (กด Back กลับมาได้เงื่อนไขเดิม)
 */
export default async function CompaniesPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const params = await searchParams
  const query = pickQuery(params, QUERY_KEYS)
  const [user, store] = await Promise.all([currentUser(), cookies()])
  const lastUsed = store.get(appConfig.cookies.company)?.value
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
  // บริษัทที่ใช้ล่าสุดขึ้นก่อน (ในหน้าปัจจุบัน) — ลำดับอื่นตามที่ API เรียงมา
  rows = [...rows].sort((a, b) => Number(b.id === lastUsed) - Number(a.id === lastUsed))

  const q = param(params, 'q')
  const path = appConfig.routes.companies
  const hasCompanies = meta.total > 0 || q !== ''

  return (
    <div className="flex min-h-full flex-col bg-[radial-gradient(70rem_26rem_at_50%_-8rem,var(--color-primary-soft),transparent)]">
      <TopBar userName={user.name} />
      <main id="main" tabIndex={-1} className="mx-auto w-full max-w-4xl flex-1 px-4 pt-12 pb-16 outline-none sm:pt-16">
        <header className="mb-8 flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
          <div>
            <p className="text-[13px] font-medium text-ink-muted">{format(texts.launcher.greeting, { name: user.name })}</p>
            <h1 className="mt-1 text-[26px] leading-tight font-semibold tracking-tight">{t.heading}</h1>
            <p className="mt-1.5 text-sm text-ink-muted">
              {t.description}
              {meta.total > 0 && <span className="text-ink-subtle"> · {format(t.count, { n: meta.total.toLocaleString('th-TH') })}</span>}
            </p>
          </div>
          {hasCompanies && (
            <Form action="" className="flex w-full items-end gap-2 sm:w-auto">
              <TextField
                name="q"
                type="search"
                label={t.searchLabel}
                placeholder={t.searchPlaceholder}
                defaultValue={q}
                startAdornment={<Search aria-hidden />}
                className="flex-1 sm:w-72"
              />
              <Button type="submit" variant="secondary">
                {texts.common.search}
              </Button>
            </Form>
          )}
        </header>

        {error && <Alert tone="error">{error}</Alert>}

        {!error && !hasCompanies && <EmptyState />}

        {!error && hasCompanies && (
          <>
            {rows.length === 0 && (
              <p className="mb-3 text-sm text-ink-muted">
                {format(t.noMatch, { q })}{' '}
                <Link href={path} className="font-medium text-primary hover:underline">
                  {t.clearSearch}
                </Link>
              </p>
            )}
            <ul className="grid gap-3 sm:grid-cols-2">
              {rows.map((c) => (
                <li key={c.id}>
                  <CompanyCard company={c} lastUsed={c.id === lastUsed} />
                </li>
              ))}
              <li>
                <Link
                  href={appConfig.routes.newCompany}
                  className="group flex h-full min-h-[88px] items-center gap-3.5 rounded-(--radius-panel) border border-dashed border-line-strong px-4 py-3.5 text-ink-muted transition-colors hover:border-primary hover:bg-surface hover:text-primary"
                >
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-dashed border-current">
                    <Plus aria-hidden className="size-5" />
                  </span>
                  <span>
                    <span className="block text-sm font-medium">{t.create}</span>
                    <span className="text-[13px] text-ink-subtle">{t.createHint}</span>
                  </span>
                </Link>
              </li>
            </ul>
          </>
        )}

        {meta.total > meta.page_size && (
          <div className="mt-6">
            <Pagination
              page={meta.page}
              pageSize={meta.page_size}
              total={meta.total}
              href={(page) => withQuery(path, query, { page })}
              labels={texts.common.pagination}
            />
          </div>
        )}
      </main>
    </div>
  )
}

function CompanyCard({ company: c, lastUsed }: { company: CompanySummary; lastUsed: boolean }) {
  const t = texts.companies
  const inactive = c.status !== 'active'
  return (
    <Link
      href={companyPath(c.id)}
      aria-label={format(t.open, { name: c.name })}
      className={cx(
        'group relative flex h-full min-h-[88px] items-center gap-3.5 rounded-(--radius-panel) border bg-surface px-4 py-3.5 shadow-card transition-[border-color,box-shadow]',
        'hover:border-primary/40 hover:shadow-pop focus-visible:ring-3 focus-visible:ring-primary/25 focus-visible:outline-none',
        lastUsed ? 'border-primary/35' : 'border-line',
      )}
    >
      <span
        aria-hidden
        className={cx(
          'flex size-10 shrink-0 items-center justify-center rounded-lg text-[13px] font-semibold tracking-wide text-white shadow-tile',
          inactive ? 'bg-ink-subtle' : avatarTone(c.code),
        )}
      >
        {c.code.slice(0, 2)}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2">
          <span className="truncate text-[15px] font-medium">{c.name}</span>
          {lastUsed && (
            <Badge tone="info" dot>
              {t.lastUsed}
            </Badge>
          )}
        </span>
        <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-[13px] text-ink-muted">
          <span>{c.code}</span>
          {c.tax_id && (
            <>
              <span aria-hidden className="text-line-strong">
                ·
              </span>
              <span className="tabular-nums">{format(t.taxId, { taxId: c.tax_id })}</span>
            </>
          )}
        </span>
        {inactive && (
          <span className="mt-1.5 block">
            <Badge>{t.inactive}</Badge>
          </span>
        )}
      </span>
      <ArrowRight aria-hidden className="size-4 shrink-0 -translate-x-1 text-ink-subtle opacity-0 transition-[opacity,transform] group-hover:translate-x-0 group-hover:text-primary group-hover:opacity-100" />
    </Link>
  )
}

function EmptyState() {
  const t = texts.companies
  return (
    <section className="flex flex-col items-center rounded-(--radius-panel) border border-dashed border-line-strong bg-surface px-6 py-14 text-center">
      <span className="flex size-12 items-center justify-center rounded-full bg-primary-soft text-primary">
        <Building2 aria-hidden className="size-6" />
      </span>
      <h2 className="mt-4 text-lg font-semibold">{t.emptyTitle}</h2>
      <p className="mt-1.5 max-w-sm text-sm text-ink-muted">{t.empty}</p>
      <ButtonLink href={appConfig.routes.newCompany} className="mt-6">
        <Plus aria-hidden />
        {t.create}
      </ButtonLink>
    </section>
  )
}

function withPageSize(query: URLSearchParams): URLSearchParams {
  const q = new URLSearchParams(query)
  q.set('page_size', String(appConfig.pageSize))
  return q
}
