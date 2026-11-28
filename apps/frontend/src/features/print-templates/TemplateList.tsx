'use client'

import { Copy, Download, Star, Trash2 } from 'lucide-react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Badge, Button, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { builtinTemplate } from './builtin'
import { downloadHtml } from './TemplateEditor'
import { templatesApi } from './TemplatedPrint'
import type { DocumentTemplate, TemplateDocType } from './types'

/**
 * รายการแม่แบบแยกตามชนิดเอกสาร: แถวแรก = แม่แบบมาตรฐานของระบบ (อ่านอย่างเดียว, คัดลอกไปสร้างได้), ต่อด้วยแม่แบบของบริษัท
 * ค่าเริ่มต้นของชนิดเอกสาร = แม่แบบของบริษัทที่ตั้งไว้ หรือแม่แบบมาตรฐาน (ไม่มีแม่แบบใดเป็นค่าเริ่มต้น)
 */
export function TemplateList({
  companyId,
  basePath,
  docTypes,
  items,
  canManage,
}: {
  companyId: string
  basePath: string
  docTypes: TemplateDocType[]
  items: DocumentTemplate[]
  canManage: boolean
}) {
  const t = texts.printTemplates
  const router = useRouter()
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const run = async (key: string, fn: () => Promise<unknown>, toast: string) => {
    setBusy(key)
    setError(null)
    try {
      await fn()
      showToast(toast)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  const exportOne = async (x: DocumentTemplate) => {
    try {
      const full = await apiRequest<DocumentTemplate>(`${templatesApi(companyId)}/${x.id}`)
      downloadHtml(full.name, full.html ?? '')
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {error && <Alert tone="error">{error}</Alert>}
      {docTypes.map((dt) => {
        const rows = items.filter((x) => x.doc_type === dt)
        const hasDefault = rows.some((x) => x.is_default)
        const typeName = t.docTypes[dt] ?? dt
        return (
          <section key={dt} aria-labelledby={`tpl-${dt}`} className="overflow-hidden rounded-(--radius-panel) border border-line bg-surface shadow-card">
            <h2 id={`tpl-${dt}`} className="border-b border-line bg-subtle px-4 py-2.5 text-[15px] font-semibold">
              {typeName}
            </h2>
            <ul className="divide-y divide-line">
              <li className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
                <span className="min-w-0 flex-1 font-medium">{t.builtinName}</span>
                <Badge tone="neutral">{t.builtin}</Badge>
                {!hasDefault && <Badge tone="info">{t.default}</Badge>}
                <span className="text-sm text-ink-muted">{t.papers[builtinTemplate(dt).paper]}</span>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" variant="secondary" onClick={() => downloadHtml(`${typeName} - ${t.builtinName}`, builtinTemplate(dt).html)}>
                    <Download aria-hidden />
                    {t.actions.export}
                  </Button>
                  {canManage && (
                    <ButtonLink href={`${basePath}/new?doc_type=${dt}&from=builtin`} variant="secondary">
                      <Copy aria-hidden />
                      {t.actions.copyBuiltin}
                    </ButtonLink>
                  )}
                </div>
              </li>
              {rows.map((x) => (
                <li key={x.id} className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
                  <Link href={`${basePath}/${x.id}`} className="min-w-0 flex-1 truncate font-medium text-primary hover:underline">
                    {x.name}
                  </Link>
                  {x.is_default && <Badge tone="info">{t.default}</Badge>}
                  {x.status === 'inactive' && <Badge tone="neutral">{t.inactive}</Badge>}
                  <span className="text-sm text-ink-muted">{t.papers[x.paper]}</span>
                  <div className="flex flex-wrap gap-2">
                    <Button type="button" variant="secondary" onClick={() => void exportOne(x)}>
                      <Download aria-hidden />
                      {t.actions.export}
                    </Button>
                    {canManage && !x.is_default && x.status === 'active' && (
                      <Button
                        type="button"
                        variant="secondary"
                        loading={busy === `d-${x.id}`}
                        onClick={() => {
                          if (window.confirm(format(t.confirm.setDefault, { name: x.name, type: typeName })))
                            void run(`d-${x.id}`, () => apiRequest(`${templatesApi(companyId)}/${x.id}/default`, { method: 'PUT' }), t.toast.defaultSet)
                        }}
                      >
                        <Star aria-hidden />
                        {t.actions.setDefault}
                      </Button>
                    )}
                    {canManage && (
                      <Button
                        type="button"
                        variant="danger"
                        loading={busy === `x-${x.id}`}
                        aria-label={`${t.actions.delete} ${x.name}`}
                        onClick={() => {
                          if (window.confirm(format(t.confirm.delete, { name: x.name })))
                            void run(`x-${x.id}`, () => apiRequest(`${templatesApi(companyId)}/${x.id}`, { method: 'DELETE' }), t.toast.deleted)
                        }}
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          </section>
        )
      })}
    </div>
  )
}
