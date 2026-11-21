'use client'

import { useRouter } from 'next/navigation'
import { Fragment, useState } from 'react'
import { Alert, Badge, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { DEFAULT_TEMPLATE, previewNumber } from '@/lib/numbering'
import type { DocumentNumbering } from './resources'

/**
 * รูปแบบเลขที่เอกสารทุกชนิด — แก้ทีละแถว (PUT /document-numberings/{docType})
 * ระหว่างพิมพ์แสดงตัวอย่างเลขที่ทันที (ตรวจด้วยกติกาเดียวกับ backend) — บันทึกแล้วตัวอย่างมาจาก backend
 */
export function NumberingTable({ apiPath, items, canManage }: { apiPath: string; items: DocumentNumbering[]; canManage: boolean }) {
  const t = texts.numbering
  const router = useRouter()
  const [editing, setEditing] = useState<string | null>(null)
  const [draft, setDraft] = useState({ prefix: '', template: '' })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const now = new Date()
  const preview = previewNumber(draft.prefix, draft.template, { date: now, fiscalYear: now.getFullYear() })

  function start(n: DocumentNumbering) {
    setEditing(n.doc_type)
    setDraft({ prefix: n.prefix, template: n.template })
    setError(null)
  }

  async function save(docType: string, name: string) {
    if (!preview.ok) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(`${apiPath}/${encodeURIComponent(docType)}`, {
        method: 'PUT',
        body: { prefix: draft.prefix.trim().toUpperCase(), template: draft.template.trim() },
      })
      setEditing(null)
      showToast(format(t.savedToast, { name }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <details className="group rounded-(--radius-panel) border border-line bg-surface px-4 py-3 text-sm shadow-card">
        <summary className="cursor-pointer font-medium text-primary marker:text-ink-subtle">{t.helpTitle}</summary>
        <dl className="mt-3 grid gap-x-6 gap-y-1 sm:grid-cols-[auto_1fr]">
          {t.help.map(([token, desc]) => (
            <Fragment key={token}>
              <dt className="font-mono text-[13px] text-ink">{token}</dt>
              <dd className="text-ink-muted">{desc}</dd>
            </Fragment>
          ))}
        </dl>
        <p className="mt-3 text-ink-muted">{t.resetHint}</p>
      </details>

      <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full border-collapse text-sm tabular-nums">
          <thead className="bg-subtle text-[13px] text-ink-muted">
            <tr>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.columns.docType}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.columns.prefix}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.columns.template}</th>
              <th scope="col" className="h-10 border-b border-line px-4 text-left font-medium">{t.columns.example}</th>
              {canManage && <th scope="col" className="h-10 border-b border-line px-4" />}
            </tr>
          </thead>
          <tbody>
            {items.map((n) =>
              editing === n.doc_type ? (
                <tr key={n.doc_type} className="border-b border-line bg-primary-soft/60 last:border-b-0">
                  <td className="px-4 py-3 align-top">
                    <div className="font-medium">{n.name}</div>
                    <div className="text-xs text-ink-subtle">{n.doc_type}</div>
                  </td>
                  <td className="px-4 py-2 align-top">
                    <TextField
                      label={t.columns.prefix}
                      className="w-28 [&>label]:sr-only"
                      value={draft.prefix}
                      maxLength={10}
                      onChange={(e) => setDraft((d) => ({ ...d, prefix: e.target.value.toUpperCase() }))}
                    />
                  </td>
                  <td className="px-4 py-2 align-top">
                    <TextField
                      label={t.columns.template}
                      className="min-w-56 font-mono [&>label]:sr-only"
                      value={draft.template}
                      maxLength={60}
                      error={preview.ok ? undefined : preview.error}
                      onChange={(e) => setDraft((d) => ({ ...d, template: e.target.value }))}
                    />
                  </td>
                  <td className="px-4 py-3 align-top font-mono text-[13px] font-medium text-primary" aria-live="polite">
                    {preview.ok ? preview.example : '–'}
                  </td>
                  <td className="px-4 py-2 align-top">
                    <div className="flex flex-wrap justify-end gap-2">
                      <Button size="sm" variant="ghost" disabled={busy} onClick={() => setDraft({ prefix: n.default_prefix, template: DEFAULT_TEMPLATE })}>
                        {t.useDefault}
                      </Button>
                      <Button size="sm" variant="secondary" disabled={busy} onClick={() => setEditing(null)}>
                        {texts.common.cancel}
                      </Button>
                      <Button size="sm" loading={busy} loadingText={texts.common.saving} disabled={!preview.ok} onClick={() => save(n.doc_type, n.name)}>
                        {texts.common.save}
                      </Button>
                    </div>
                  </td>
                </tr>
              ) : (
                <tr key={n.doc_type} className="border-b border-line last:border-b-0 hover:bg-subtle">
                  <td className="px-4 py-3">
                    <div className="font-medium">{n.name}</div>
                    <div className="text-xs text-ink-subtle">{n.doc_type}</div>
                  </td>
                  <td className="px-4 py-3 font-mono text-[13px]">{n.prefix || '–'}</td>
                  <td className="px-4 py-3 font-mono text-[13px]">
                    {n.template} {n.custom ? <Badge tone="info">{t.custom}</Badge> : <Badge>{t.default}</Badge>}
                  </td>
                  <td className="px-4 py-3 font-mono text-[13px]">{n.example}</td>
                  {canManage && (
                    <td className="px-4 py-2 text-right">
                      <Button size="sm" variant="secondary" disabled={editing !== null} onClick={() => start(n)}>
                        {t.edit}
                      </Button>
                    </td>
                  )}
                </tr>
              ),
            )}
          </tbody>
        </table>
      </div>
      {error && <Alert tone="error">{error}</Alert>}
    </div>
  )
}
