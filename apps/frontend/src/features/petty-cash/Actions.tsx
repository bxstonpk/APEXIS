'use client'

import { Printer } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Button, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'

/** การกระทำของเอกสาร: POST {apiPath}/{path} หลังยืนยัน — ปุ่มยกเลิกสีแดงอยู่ซ้าย, ปุ่มหลักอยู่ขวาสุด */
export type PcAction = { path: string; label: string; confirm: string; primary?: boolean; danger?: boolean }

export function PcActions({ apiPath, docNo, actions, printHref }: { apiPath: string; docNo: string; actions: PcAction[]; printHref?: string }) {
  const router = useRouter()
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function run(a: PcAction) {
    if (!window.confirm(format(a.confirm, { no: docNo }))) return
    setBusy(a.label)
    setError(null)
    try {
      await apiRequest(`${apiPath}/${a.path}`, { method: 'POST', body: {} })
      showToast(texts.common.toast.saved)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  const button = (a: PcAction) => (
    <Button
      key={a.label}
      variant={a.danger ? 'danger' : a.primary ? 'primary' : 'secondary'}
      loading={busy === a.label}
      loadingText={texts.pettyCash.vouchers.actions.working}
      disabled={busy !== null}
      onClick={() => void run(a)}
    >
      {a.label}
    </Button>
  )
  if (!actions.length && !printHref) return null
  return (
    <div className="flex flex-col gap-3 print:hidden">
      {error && <Alert tone="error">{error}</Alert>}
      <div className="flex flex-wrap items-center gap-2 rounded-(--radius-panel) border border-line bg-surface px-4 py-3 shadow-card">
        {actions.filter((a) => a.danger).map(button)}
        <div className="ml-auto flex flex-wrap gap-2">
          {printHref && (
            <ButtonLink href={printHref} variant="secondary">
              <Printer aria-hidden />
              {texts.pettyCash.vouchers.actions.print}
            </ButtonLink>
          )}
          {actions.filter((a) => !a.danger).map(button)}
        </div>
      </div>
    </div>
  )
}
