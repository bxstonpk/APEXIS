'use client'

import { BookOpenCheck, Plus } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Button, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { businessTypeName } from '@/config/businessTypes'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'

/**
 * หน้าว่างของผังบัญชี: สร้างผังบัญชีมาตรฐาน (POST /accounts/defaults) หรือสร้างบัญชีเอง
 * ผังมาตรฐานเป็นชุดตามประเภทธุรกิจของบริษัท (backend เลือกเอง) — แสดงชื่อประเภทให้ผู้ใช้รู้ว่าจะได้ผังชุดไหน
 */
export function DefaultChartPanel({ apiPath, createHref, businessType }: { apiPath: string; createHref: string | false; businessType?: string }) {
  const t = texts.accounts.empty
  const type = businessTypeName(businessType) || texts.businessTypes.label
  const router = useRouter()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function create() {
    if (!window.confirm(format(t.confirm, { type }))) return
    setBusy(true)
    setError(null)
    try {
      const res = await apiRequest<{ accounts: number; journals: number }>(apiPath, { method: 'POST' })
      showToast(format(t.created, { accounts: res.accounts, journals: res.journals }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="flex flex-col items-center rounded-(--radius-panel) border border-dashed border-line-strong bg-surface px-6 py-14 text-center">
      <span className="flex size-12 items-center justify-center rounded-full bg-primary-soft text-primary">
        <BookOpenCheck aria-hidden className="size-6" />
      </span>
      <h2 className="mt-4 text-base font-semibold">{t.title}</h2>
      <p className="mt-1 text-sm text-ink-muted">{type}</p>
      {error && (
        <div className="mt-4 w-full max-w-xl text-left">
          <Alert tone="error">{error}</Alert>
        </div>
      )}
      {createHref && (
        <div className="mt-6 flex flex-wrap justify-center gap-2">
          <Button onClick={create} loading={busy} loadingText={t.creating}>
            {t.useDefaults}
          </Button>
          <ButtonLink href={createHref} variant="secondary">
            <Plus aria-hidden />
            {t.createOwn}
          </ButtonLink>
        </div>
      )}
    </section>
  )
}
