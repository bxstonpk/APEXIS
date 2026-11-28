'use client'

import { ReceiptText } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useId, useState, type FormEvent } from 'react'
import { Alert, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { requiredMessage } from '@/lib/validation'

/**
 * ขายบริการ: ออกใบกำกับภาษีก่อนรับเงิน (POST /service-invoices/{id}/tax-invoice {date})
 * แผงในหน้า (ไม่ใช่ modal) — ระบุวันที่ ยืนยัน แล้วภาษีขายที่ยังไม่ถึงกำหนดทั้งหมดเกิด ณ วันนั้น
 */
export function TaxInvoiceAction({ apiPath, docNo }: { apiPath: string; docNo: string }) {
  const t = texts.sales.serviceInvoices.taxInvoice
  const router = useRouter()
  const headingId = useId()
  const [open, setOpen] = useState(false)
  const [date, setDate] = useState(today())
  const [dateError, setDateError] = useState<string>()
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!date) return setDateError(requiredMessage(t.date))
    setDateError(undefined)
    if (!window.confirm(format(t.confirm, { no: docNo, date: formatDate(date) }))) return
    setBusy(true)
    setError(null)
    try {
      const res = await apiRequest<{ tax_invoice?: { no: string } | null }>(`${apiPath}/tax-invoice`, { method: 'POST', body: { date } })
      showToast(format(t.done, { no: res?.tax_invoice?.no ?? '' }))
      setOpen(false)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  if (!open)
    return (
      <div className="flex justify-end print:hidden">
        <Button variant="secondary" onClick={() => setOpen(true)}>
          <ReceiptText aria-hidden />
          {t.action}
        </Button>
      </div>
    )

  return (
    <section aria-labelledby={headingId} className="rounded-(--radius-panel) border border-primary/40 bg-surface p-5 shadow-card print:hidden">
      <h2 id={headingId} className="text-[15px] font-semibold">
        {t.title}
      </h2>      {error && (
        <div className="mt-3">
          <Alert tone="error">{error}</Alert>
        </div>
      )}
      <form noValidate onSubmit={submit} className="mt-4 flex flex-wrap items-end gap-3">
        <TextField label={t.date} required type="date" value={date} error={dateError} onChange={(e) => setDate(e.target.value)} className="w-full sm:w-56" />
        <div className="ml-auto flex gap-2">
          <Button variant="secondary" disabled={busy} onClick={() => setOpen(false)}>
            {texts.common.cancel}
          </Button>
          <Button type="submit" loading={busy} loadingText={texts.sales.actions.working}>
            {t.submit}
          </Button>
        </div>
      </form>
    </section>
  )
}
