import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { texts } from '@/config'
import { CompaniesSkeleton, LauncherSkeleton, PageSkeleton, type SkeletonKind } from './PageSkeleton'

describe('PageSkeleton', () => {
  it.each<SkeletonKind>(['list', 'dashboard', 'report', 'form', 'doc'])('%s: บอก screen reader ว่ากำลังโหลด และซ่อนชิ้น skeleton', (kind) => {
    const { container } = render(<PageSkeleton kind={kind} />)
    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('aria-busy', 'true')
    expect(status).toHaveTextContent(texts.common.loading)
    const pieces = container.querySelectorAll('[data-slot="skeleton"]')
    expect(pieces.length).toBeGreaterThan(5)
    for (const p of pieces) expect(p.closest('[aria-hidden="true"]')).not.toBeNull()
  })

  it('หน้ารวมแอป / หน้าเลือกบริษัทมี skeleton ของตัวเอง', () => {
    render(
      <>
        <LauncherSkeleton />
        <CompaniesSkeleton />
      </>,
    )
    expect(screen.getAllByRole('status')).toHaveLength(2)
  })
})
