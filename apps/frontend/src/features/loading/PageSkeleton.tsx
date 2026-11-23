import type { ReactNode } from 'react'
import { Skeleton } from '@/components/ui/skeleton'
import { texts } from '@/config'
import { cn } from '@/lib/utils'

/**
 * Skeleton ระหว่างโหลดหน้า — ใช้ใน loading.tsx ทุกแอป (แสดงทุกครั้ง ไม่ว่าจะเร็วหรือช้า)
 *
 * - โครงเดียวกับหน้าจริง (หัวเรื่อง / ตัวกรอง / ตาราง / การ์ด / ฟอร์ม) เพื่อไม่ให้เนื้อหากระโดดเมื่อโหลดเสร็จ
 * - role="status" + aria-busy + ข้อความ sr-only ให้ screen reader รู้ว่ากำลังโหลด; ชิ้น skeleton ซ่อนจาก a11y tree
 * - animate-pulse ปิดเมื่อผู้ใช้ตั้ง reduced motion (อยู่ใน components/ui/skeleton)
 * - เป็น server component ล้วน ไม่เพิ่ม JS ฝั่ง browser
 */
export type SkeletonKind = 'list' | 'dashboard' | 'report' | 'form' | 'doc'

export function PageSkeleton({ kind = 'list' }: { kind?: SkeletonKind }) {
  return (
    <Busy className="flex flex-col gap-6">
      {kind === 'list' && <ListBody />}
      {kind === 'dashboard' && <DashboardBody />}
      {kind === 'report' && <ReportBody />}
      {kind === 'form' && <FormBody />}
      {kind === 'doc' && <DocBody />}
    </Busy>
  )
}

function Busy({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div role="status" aria-busy="true" aria-live="polite" className={className}>
      <span className="sr-only">{texts.common.loading}</span>
      <div aria-hidden className="contents">
        {children}
      </div>
    </div>
  )
}

/** ชิ้น skeleton — ขนาดกำหนดด้วย className */
function S({ className }: { className: string }) {
  return <Skeleton className={className} />
}

function PageHeader({ action = true, wide = false }: { action?: boolean; wide?: boolean }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
      <div className="flex min-w-0 flex-col gap-2">
        <S className="h-3.5 w-32" />
        <S className="h-7 w-56 max-w-full" />
        <S className="h-4 w-80 max-w-full" />
      </div>
      {action && <S className={cn('h-10 rounded-(--radius-control)', wide ? 'w-full sm:w-96' : 'w-28')} />}
    </div>
  )
}

function Panel({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn('rounded-(--radius-panel) border border-line bg-surface shadow-card', className)}>{children}</div>
}

function FilterBar({ fields = 2 }: { fields?: number }) {
  return (
    <Panel className="flex flex-wrap items-end gap-2 p-4">
      {Array.from({ length: fields }, (_, i) => (
        <div key={i} className="flex w-full flex-col gap-1.5 sm:w-56">
          <S className="h-3.5 w-20" />
          <S className="h-10 w-full rounded-(--radius-control)" />
        </div>
      ))}
      <S className="h-10 w-24 rounded-(--radius-control)" />
    </Panel>
  )
}

/** ความกว้างของเซลล์ต่างกันต่อแถว ให้ดูเป็นข้อมูลจริง ไม่ใช่แถบเท่ากันหมด */
const CELL_WIDTHS = ['w-24', 'w-40', 'w-32', 'w-48', 'w-28', 'w-36']

function Table({ rows = 8, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <Panel className="overflow-hidden">
      <div className="flex gap-6 border-b border-line bg-subtle px-4 py-3">
        {Array.from({ length: cols }, (_, c) => (
          <S key={c} className={cn('h-3.5', c === 0 ? 'w-20' : 'w-16', c >= 3 && 'ml-auto hidden sm:block', c > 3 && 'ml-0')} />
        ))}
      </div>
      {Array.from({ length: rows }, (_, r) => (
        <div key={r} className="flex items-center gap-6 border-b border-line px-4 py-3.5 last:border-b-0">
          {Array.from({ length: cols }, (_, c) => (
            <S
              key={c}
              className={cn('h-4 shrink-0', c === 0 ? 'w-20' : CELL_WIDTHS[(r + c) % CELL_WIDTHS.length], c >= 3 && 'hidden sm:block', c === 3 && 'ml-auto', c > 3 && 'w-20')}
            />
          ))}
        </div>
      ))}
    </Panel>
  )
}

function ListBody() {
  return (
    <>
      <PageHeader />
      <FilterBar />
      <Table />
    </>
  )
}

function ReportBody() {
  return (
    <>
      <PageHeader />
      <FilterBar fields={3} />
      <Table rows={12} cols={6} />
    </>
  )
}

function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <Panel className={cn('flex flex-col gap-3 p-4', className)}>{children}</Panel>
}

function DashboardBody() {
  return (
    <>
      <PageHeader wide />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Card key={i}>
            <S className="h-3.5 w-28" />
            <S className="h-7 w-36" />
            <S className="h-3.5 w-24" />
          </Card>
        ))}
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        <Card className="lg:col-span-3">
          <S className="h-4 w-40" />
          <S className="h-3.5 w-56 max-w-full" />
          <div className="mt-2 flex h-64 items-end gap-2">
            {[45, 60, 38, 72, 55, 80, 66, 48, 70, 58, 84, 62].map((h, i) => (
              <Skeleton key={i} className="flex-1 rounded-b-none" style={{ height: `${h}%` }} />
            ))}
          </div>
        </Card>
        <Card className="lg:col-span-2">
          <S className="h-4 w-36" />
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="flex flex-col gap-1.5 pt-1">
              <div className="flex justify-between gap-3">
                <S className={cn('h-3.5', CELL_WIDTHS[i % CELL_WIDTHS.length])} />
                <S className="h-3.5 w-16" />
              </div>
              <S className="h-1.5 w-full" />
            </div>
          ))}
        </Card>
      </div>
    </>
  )
}

function FormSection({ fields }: { fields: number }) {
  return (
    <div className="grid gap-4 border-b border-line pb-8 last:border-b-0 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-10">
      <div className="flex flex-col gap-2">
        <S className="h-4 w-32" />
        <S className="h-3.5 w-48 max-w-full" />
      </div>
      <Panel className="grid gap-x-5 gap-y-4 p-5 sm:grid-cols-2">
        {Array.from({ length: fields }, (_, i) => (
          <div key={i} className="flex flex-col gap-1.5">
            <S className={cn('h-3.5', i % 2 ? 'w-24' : 'w-32')} />
            <S className="h-10 w-full rounded-(--radius-control)" />
          </div>
        ))}
      </Panel>
    </div>
  )
}

function FormBody() {
  return (
    <>
      <PageHeader action={false} />
      <div className="flex flex-col gap-8">
        <FormSection fields={6} />
        <FormSection fields={4} />
      </div>
      <div className="flex justify-end gap-2 border-t border-line pt-4">
        <S className="h-10 w-24 rounded-(--radius-control)" />
        <S className="h-10 w-28 rounded-(--radius-control)" />
      </div>
    </>
  )
}

function DocBody() {
  return (
    <>
      <div className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
        <div className="flex min-w-0 flex-col gap-2">
          <S className="h-3.5 w-32" />
          <div className="flex items-center gap-2">
            <S className="h-7 w-48" />
            <S className="h-6 w-16 rounded-full" />
          </div>
        </div>
        <div className="flex gap-2">
          <S className="h-10 w-24 rounded-(--radius-control)" />
          <S className="h-10 w-28 rounded-(--radius-control)" />
        </div>
      </div>
      <Panel className="grid gap-x-8 gap-y-4 p-5 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }, (_, i) => (
          <div key={i} className="flex flex-col gap-1.5">
            <S className="h-3.5 w-20" />
            <S className={cn('h-4', CELL_WIDTHS[(i + 2) % CELL_WIDTHS.length])} />
          </div>
        ))}
      </Panel>
      <Table rows={4} cols={5} />
      <div className="flex flex-col items-end gap-2">
        <S className="h-4 w-56" />
        <S className="h-4 w-48" />
        <S className="h-5 w-60" />
      </div>
    </>
  )
}

/** หน้ารวมแอป — ไอคอนแอปกลางจอ */
export function LauncherSkeleton() {
  return (
    <Busy className="flex w-full flex-col items-center">
      <S className="h-3.5 w-40" />
      <S className="mt-2 h-8 w-72 max-w-full" />
      <div className="mt-12 grid grid-cols-3 gap-x-2 gap-y-6 sm:grid-cols-4 sm:gap-x-6 sm:gap-y-8">
        {Array.from({ length: 8 }, (_, i) => (
          <div key={i} className="flex w-24 flex-col items-center gap-2.5 p-2 sm:w-28">
            <S className="size-16 rounded-[18px] sm:size-[72px]" />
            <S className="h-3.5 w-16" />
          </div>
        ))}
      </div>
    </Busy>
  )
}

/** เมนูของแอปบนแถบสีเข้ม (ใช้ตอนโครงแอปยังรอสิทธิ์ผู้ใช้) */
export function NavSkeleton() {
  return (
    <div aria-hidden className="flex min-w-0 flex-1 items-center gap-2 overflow-hidden px-1">
      {['w-16', 'w-20', 'w-24', 'w-16', 'w-20'].map((w, i) => (
        <span key={i} className={cn('h-4 shrink-0 animate-pulse rounded-md bg-white/15 motion-reduce:animate-none', w, i > 2 && 'hidden md:block')} />
      ))}
    </div>
  )
}

/** หน้าเลือกบริษัท — หัวเรื่อง + ช่องค้นหา + การ์ดบริษัท */
export function CompaniesSkeleton() {
  return (
    <Busy className="flex flex-col">
      <div className="mb-8 flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
        <div className="flex flex-col gap-2">
          <S className="h-3.5 w-40" />
          <S className="h-8 w-56" />
          <S className="h-4 w-64 max-w-full" />
        </div>
        <S className="h-10 w-full rounded-(--radius-control) sm:w-80" />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        {Array.from({ length: 4 }, (_, i) => (
          <Panel key={i} className="flex min-h-[88px] items-center gap-3.5 px-4 py-3.5">
            <S className="size-10 shrink-0 rounded-lg" />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <S className={cn('h-4', CELL_WIDTHS[(i + 1) % CELL_WIDTHS.length])} />
              <S className="h-3.5 w-24" />
            </div>
          </Panel>
        ))}
      </div>
    </Busy>
  )
}
