import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AppTile } from './AppTile'
import { CloseAppButton } from './CloseAppButton'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

describe('AppTile', () => {
  it('กดแล้วเปิดแท็บใหม่ด้วยชื่อแท็บของแอป', async () => {
    const win = { focus: vi.fn() }
    const open = vi.spyOn(window, 'open').mockReturnValue(win as unknown as Window)
    render(<AppTile href="/c/1/sales/quotations" windowName="apexis-1-sales" label="ขาย" title="ขาย (เปิดในแท็บใหม่)" icon={null} tone="bg-app-sales" />)
    const link = screen.getByRole('link', { name: 'ขาย (เปิดในแท็บใหม่)' })
    expect(link).toHaveAttribute('target', 'apexis-1-sales')
    await userEvent.click(link)
    expect(open).toHaveBeenCalledWith('/c/1/sales/quotations', 'apexis-1-sales')
    expect(win.focus).toHaveBeenCalled()
  })
})

describe('CloseAppButton', () => {
  beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }))
  afterEach(() => {
    vi.useRealTimers()
    push.mockReset()
  })

  it('ปิดแท็บ; ปิดไม่ได้ (เปิดเอง) → กลับหน้ารวมแอปในแท็บนี้', async () => {
    const close = vi.spyOn(window, 'close').mockImplementation(() => {})
    render(<CloseAppButton launcherHref="/c/1" label="ปิด" />)
    await userEvent.click(screen.getByRole('button', { name: 'ปิด' }))
    expect(close).toHaveBeenCalled()
    vi.advanceTimersByTime(150)
    expect(push).toHaveBeenCalledWith('/c/1')
  })
})
