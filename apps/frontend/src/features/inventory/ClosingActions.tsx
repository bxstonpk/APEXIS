'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Alert, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatDate } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { requiredMessage } from '@/lib/validation'

/** ปิดงวดสต็อก ณ วันสิ้นงวด — ยืนยันก่อน, ผลแจ้งด้วย toast แล้วโหลดรายการใหม่ */
export function CreateClosing({ apiPath, defaultDate }: { apiPath: string; defaultDate: string }) {
  const t = texts.inventory.closing
  const router = useRouter()
  const [date, setDate] = useState(defaultDate)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldError, setFieldError] = useState<string>()

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!date) return setFieldError(requiredMessage(t.date))
    setFieldError(undefined)
    if (!window.confirm(format(t.confirm.create!, { date: formatDate(date) }))) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'POST', body: { date } })
      showToast(t.done)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-3 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card">
      {error && <Alert tone="error">{error}</Alert>}
      <div className="flex flex-wrap items-end gap-2">
        <TextField label={t.date} hint={t.dateHint} required type="date" value={date} error={fieldError} onChange={(e) => setDate(e.target.value)} />
        <Button type="submit" loading={busy} loadingText={texts.common.saving} className="sm:mb-[22px]">
          {t.create}
        </Button>
      </div>
    </form>
  )
}

export function CancelClosing({ apiPath, periodEnd }: { apiPath: string; periodEnd: string }) {
  const t = texts.inventory.closing
  const router = useRouter()
  const [busy, setBusy] = useState(false)
  async function cancel() {
    if (!window.confirm(format(t.confirm.cancel!, { date: formatDate(periodEnd) }))) return
    setBusy(true)
    try {
      await apiRequest(apiPath, { method: 'PATCH', body: { status: 'cancelled' } })
      showToast(texts.sales.toast.done)
      router.refresh()
    } catch (err) {
      window.alert(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Button size="sm" variant="danger" loading={busy} loadingText={texts.sales.actions.working} onClick={() => void cancel()}>
      {t.cancel}
    </Button>
  )
}
