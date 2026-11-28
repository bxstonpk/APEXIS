'use client'

import { useParams, usePathname, useRouter, useSearchParams } from 'next/navigation'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Alert, SelectField } from '@/components'
import { format, texts } from '@/config'
import { apiRequest } from '@/lib/http'
import { builtinTemplate } from './builtin'
import { renderDocument, type AssetMap } from './engine'
import type { DocumentAsset, DocumentTemplate, Paper, TemplateDocType } from './types'

const BUILTIN = 'builtin'

export const templatesApi = (companyId: string) => `/companies/${companyId}/document-templates`
export const assetsApi = (companyId: string) => `/companies/${companyId}/document-assets`
export const assetUrl = (companyId: string, id: string) => `/api/v1/companies/${companyId}/document-assets/${id}/content`

/** รูปของบริษัท → { ชื่อ: URL } ให้ {{asset "ชื่อ"}} */
export async function loadAssetMap(companyId: string): Promise<AssetMap> {
  const list = await apiRequest<DocumentAsset[]>(assetsApi(companyId)).catch(() => [] as DocumentAsset[])
  return Object.fromEntries(list.map((a) => [a.name, assetUrl(companyId, a.id)]))
}

/**
 * เอกสารพิมพ์ผ่านแม่แบบ: ?template=<id> > แม่แบบค่าเริ่มต้นของบริษัท > แม่แบบมาตรฐานของระบบ
 * แสดงใน iframe sandbox (ไม่รันสคริปต์) — ปุ่มพิมพ์ของ PrintToolbar สั่งพิมพ์ iframe นี้ (data-print-frame)
 * มีแม่แบบมากกว่าหนึ่ง = แสดงช่องเลือกแม่แบบ (ไม่พิมพ์)
 */
export function TemplatedPrint({ docType, data }: { docType: TemplateDocType; data: Record<string, unknown> }) {
  const { companyId } = useParams<{ companyId: string }>()
  const search = useSearchParams()
  const pathname = usePathname()
  const router = useRouter()
  const chosen = search.get('template') ?? ''
  const t = texts.printTemplates
  const [list, setList] = useState<DocumentTemplate[]>([])
  const [tpl, setTpl] = useState<{ id: string; html: string; paper: Paper } | null>(null)
  const [assets, setAssets] = useState<AssetMap | null>(null)
  const [height, setHeight] = useState(1200)
  const frame = useRef<HTMLIFrameElement>(null)

  useEffect(() => {
    let live = true
    const builtin = builtinTemplate(docType)
    const pick =
      chosen === BUILTIN
        ? Promise.resolve(null)
        : chosen
          ? apiRequest<DocumentTemplate>(`${templatesApi(companyId)}/${encodeURIComponent(chosen)}`)
          : apiRequest<DocumentTemplate>(`${templatesApi(companyId)}/default?doc_type=${docType}`)
    Promise.all([
      pick.catch(() => null),
      apiRequest<DocumentTemplate[]>(`${templatesApi(companyId)}?doc_type=${docType}&status=active&page_size=100`).catch(() => []),
      loadAssetMap(companyId),
    ]).then(([custom, rows, map]) => {
      if (!live) return
      setList(rows)
      setAssets(map)
      setTpl(custom?.html ? { id: custom.id, html: custom.html, paper: custom.paper } : { id: BUILTIN, ...builtin })
    })
    return () => {
      live = false
    }
  }, [companyId, docType, chosen])

  const rendered = useMemo(() => {
    if (!tpl || !assets) return null
    try {
      return { html: renderDocument(tpl.html, tpl.paper, data, assets) }
    } catch (err) {
      return { error: format(t.errors.render, { message: err instanceof Error ? err.message : String(err) }) }
    }
  }, [tpl, assets, data, t.errors.render])

  const fit = () => {
    const doc = frame.current?.contentDocument
    if (doc) setHeight(Math.max(doc.documentElement.scrollHeight, 400))
  }

  const onPick = (id: string) => {
    const q = new URLSearchParams(search.toString())
    if (id) q.set('template', id)
    else q.delete('template')
    router.replace(`${pathname}${q.size ? `?${q}` : ''}`)
  }

  if (!rendered) {
    return <div aria-busy="true" aria-label={t.print.loading} className="a4-sheet mx-auto animate-pulse bg-white shadow-card" />
  }
  return (
    <div className="flex flex-col gap-3">
      {list.length > 0 && (
        <div className="mx-auto w-full max-w-[210mm] print:hidden">
          <SelectField
            label={t.template}
            value={tpl?.id ?? BUILTIN}
            onChange={(e) => onPick(e.target.value)}
            options={[{ value: BUILTIN, label: `${t.builtinName} (${t.builtin})` }, ...list.map((x) => ({ value: x.id, label: x.is_default ? `${x.name} (${t.default})` : x.name }))]}
            className="sm:max-w-sm"
          />
        </div>
      )}
      {'error' in rendered ? (
        <div className="mx-auto w-full max-w-[210mm]">
          <Alert tone="error">{rendered.error}</Alert>
        </div>
      ) : (
        <iframe
          ref={frame}
          data-print-frame=""
          title={t.print.frameTitle}
          sandbox="allow-same-origin allow-modals"
          srcDoc={rendered.html}
          onLoad={fit}
          style={{ height }}
          className="mx-auto block w-full max-w-[calc(297mm+2rem)] border-0 bg-transparent"
        />
      )}
    </div>
  )
}
