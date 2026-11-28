'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Alert, Button, ReferenceSelect } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import type { LedgerSettings, Ref, YearEnd } from './types'

/** ปุ่มปิด / เปิดปีบัญชี — ยืนยันก่อน แล้วโหลดหน้าใหม่ */
export function YearEndActions({ api, y, canClose }: { api: string; y: YearEnd; canClose: boolean }) {
  const t = texts.yearEnd
  const router = useRouter()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const year = String(y.fiscal_year + 543)
  async function run(action: 'close' | 'reopen') {
    if (!window.confirm(format(t.confirm[action], { year }))) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(`${api}/${y.fiscal_year}/${action}`, { method: 'POST' })
      showToast(format(action === 'close' ? t.toast.closed : t.toast.reopened, { year }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-col items-end gap-2">
      {error && <Alert tone="error">{error}</Alert>}
      {canClose && y.status === 'open' && (
        <Button onClick={() => run('close')} disabled={!y.can_close} loading={busy} loadingText={t.actions.working}>
          {t.actions.close}
        </Button>
      )}
      {canClose && y.status === 'closed' && (
        <Button variant="secondary" onClick={() => run('reopen')} disabled={!y.can_reopen} loading={busy} loadingText={t.actions.working}>
          {t.actions.reopen}
        </Button>
      )}
    </div>
  )
}

/** ค่าตั้งการปิดบัญชี: บัญชีกำไรสะสม / สรุปผลกำไรขาดทุน (ส่วนของเจ้าของ) และสมุดรายวัน */
export function LedgerSettingsForm({ api, initial, readOnly }: { api: { settings: string; accounts: string; journals: string }; initial: LedgerSettings | null; readOnly: boolean }) {
  const t = texts.yearEnd.settings
  const router = useRouter()
  const [retained, setRetained] = useState<Ref | null>(initial?.retained_earnings_account ?? null)
  const [summary, setSummary] = useState<Ref | null>(initial?.income_summary_account ?? null)
  const [journal, setJournal] = useState<Ref | null>(initial?.closing_journal ?? null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  async function submit(ev: FormEvent) {
    ev.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await apiRequest(api.settings, {
        method: 'PUT',
        body: { retained_earnings_account_id: retained?.id ?? null, income_summary_account_id: summary?.id ?? null, closing_journal_id: journal?.id ?? null },
      })
      showToast(t.saved)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  const equity = { type: 'equity', postable: 'true', status: 'active' }
  return (
    <form onSubmit={submit} className="grid gap-4 sm:grid-cols-3">
      {error && (
        <div className="sm:col-span-3">
          <Alert tone="error">{error}</Alert>
        </div>
      )}
      <ReferenceSelect label={t.retainedEarnings} source={api.accounts} params={equity} value={retained} onChange={setRetained} disabled={readOnly} texts={texts.reference} />
      <ReferenceSelect label={t.incomeSummary} source={api.accounts} params={equity} value={summary} onChange={setSummary} disabled={readOnly} texts={texts.reference} />
      <ReferenceSelect label={t.closingJournal} source={api.journals} params={{ status: 'active' }} value={journal} onChange={setJournal} disabled={readOnly} texts={texts.reference} />
      {!readOnly && (
        <div className="flex justify-end sm:col-span-3">
          <Button type="submit" variant="secondary" loading={busy}>
            {t.save}
          </Button>
        </div>
      )}
    </form>
  )
}
