import { Inbox, Plus, SearchX, Search } from 'lucide-react'
import Form from 'next/form'
import type { ReactNode } from 'react'
import { Alert, Badge, Button, ButtonLink, DataTable, PageHeader, Pagination, TextField, type Column } from '@/components'
import { format, texts } from '@/config'
import type { ListResult } from '@/lib/server/list'
import { withQuery } from '@/lib/url'

/**
 * ส่วนประกอบหน้ารายการ (server) — ใช้ร่วมกันทุกข้อมูลหลัก
 * แม่แบบ: หัวหน้า + ปุ่มสร้าง / แถบค้นหา (GET form → URL) / กรอบตาราง (เรียงด้วยลิงก์) + แบ่งหน้าท้ายกรอบ
 */

export function ListHeader({ title, createHref, count }: { title: string; createHref?: string | false; count?: number }) {
  const countText = count === undefined ? undefined : format(texts.common.count, { n: count.toLocaleString('th-TH') })
  return (
    <PageHeader
      title={title}
      description={countText}
      actions={
        createHref && (
          <ButtonLink href={createHref}>
            <Plus aria-hidden />
            {texts.common.create}
          </ButtonLink>
        )
      }
    />
  )
}

/**
 * แถบค้นหา: ช่อง q (กด Enter ได้) + ตัวกรองเพิ่มเติม (children) — ส่งแบบ GET ลง URL (กลับไปหน้า 1)
 * keep: ค่าที่ต้องคงไว้ (เช่น sort, page_size) ส่งเป็น hidden
 */
export function ListToolbar({ q, keep = {}, children }: { q: string; keep?: Record<string, string>; children?: ReactNode }) {
  return (
    <Form action="" className="mb-3 flex flex-wrap items-end gap-2">
      <TextField
        name="q"
        type="search"
        label={texts.common.search}
        placeholder={texts.common.searchPlaceholder}
        defaultValue={q}
        startAdornment={<Search aria-hidden />}
        className="w-full sm:w-64"
      />
      {children}
      {Object.entries(keep).map(([k, v]) => v && <input key={k} type="hidden" name={k} value={v} />)}
      <Button type="submit" variant="secondary">
        {texts.common.search}
      </Button>
    </Form>
  )
}

/** กรอบตาราง + แบ่งหน้าท้ายกรอบ (หรือข้อความ error) */
export function ListView<T>({
  list,
  path,
  columns,
  rowKey,
  sort,
  createHref,
}: {
  list: ListResult<T>
  path: string
  columns: Column<T>[]
  rowKey: (row: T) => string
  sort: string
  /** มีสิทธิ์สร้าง = แสดงปุ่มสร้างในหน้าว่าง */
  createHref?: string | false
}) {
  if (list.error) return <Alert tone="error">{list.error}</Alert>
  if (list.rows.length === 0) {
    // มีเงื่อนไขค้นหา / กรอง (ไม่นับการเรียง / แบ่งหน้า) = ไม่พบตามเงื่อนไข, ไม่มี = ยังไม่มีข้อมูล
    const filtered = [...list.query.keys()].some((k) => !['sort', 'page', 'page_size'].includes(k))
    return filtered ? (
      <EmptyPanel icon={<SearchX aria-hidden className="size-6" />} title={texts.common.noMatch}>
        <ButtonLink href={path} variant="secondary">
          {texts.common.clearFilters}
        </ButtonLink>
      </EmptyPanel>
    ) : (
      <EmptyPanel icon={<Inbox aria-hidden className="size-6" />} title={texts.common.emptyTitle}>
        {createHref && (
          <ButtonLink href={createHref}>
            <Plus aria-hidden />
            {texts.common.create}
          </ButtonLink>
        )}
      </EmptyPanel>
    )
  }
  return (
    <div className="overflow-hidden rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <DataTable
        bare
        columns={columns}
        rows={list.rows}
        rowKey={rowKey}
        sort={sort}
        sortHref={(s) => withQuery(path, list.query, { sort: s, page: null })}
        emptyText={texts.common.empty}
        sortLabels={{ ascending: texts.common.sortAscending, descending: texts.common.sortDescending }}
      />
      <div className="border-t border-line bg-subtle px-4 py-2.5">
        <Pagination
          page={list.meta.page}
          pageSize={list.meta.page_size}
          total={list.meta.total}
          href={(page) => withQuery(path, list.query, { page })}
          labels={texts.common.pagination}
        />
      </div>
    </div>
  )
}

/** หน้าว่างของรายการ: ไอคอน + ข้อความ + ปุ่มทำต่อ */
function EmptyPanel({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <section className="flex flex-col items-center rounded-(--radius-panel) border border-dashed border-line-strong bg-surface px-6 py-14 text-center">
      <span className="flex size-12 items-center justify-center rounded-full bg-neutral-soft text-ink-muted">{icon}</span>
      <h2 className="mt-4 text-base font-semibold">{title}</h2>
      {children && <div className="mt-5">{children}</div>}
    </section>
  )
}

/** ป้ายสถานะ — labels มาจาก config (ค่าเริ่มต้น active / inactive) */
export function StatusBadge({ status, labels = texts.common.statuses }: { status: string; labels?: Record<string, string> }) {
  const tone = status === 'active' || status === 'open' ? 'success' : status === 'planned' ? 'info' : 'neutral'
  return (
    <Badge tone={tone} dot>
      {labels[status] ?? status}
    </Badge>
  )
}

/** ข้อมูลอ้างอิงในตาราง (ไม่มี = ขีด) */
export function RefText({ value }: { value: { code: string; name: string } | null | undefined }) {
  if (!value) return <span className="text-ink-subtle">–</span>
  return (
    <span className="inline-flex items-baseline gap-1.5">
      <span className="text-[13px] text-ink-subtle">{value.code}</span>
      <span>{value.name}</span>
    </span>
  )
}

/** class ของลิงก์รหัสในแถว (ไปหน้าแก้ไข) */
export const rowLinkClass = 'font-medium text-primary hover:underline'

/** ลิงก์จากแถวไปหน้าแก้ไข พร้อม ?back=<เงื่อนไขของรายการ> */
export function rowHref(listPath: string, id: string, query: URLSearchParams): string {
  const back = query.toString()
  return withQuery(`${listPath}/${id}`, back ? { back } : {})
}
