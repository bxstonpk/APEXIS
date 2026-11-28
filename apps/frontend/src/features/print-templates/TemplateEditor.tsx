'use client'

import { Download, Upload } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { builtinTemplate, familyOf } from './builtin'
import { HELPERS, sampleData, VARIABLES, variableSnippet } from './catalog'
import { renderDocument, type AssetMap } from './engine'
import { loadAssetMap, templatesApi } from './TemplatedPrint'
import { PAPERS, TEMPLATE_DOC_TYPES, isTemplateDocType, type DocumentTemplate, type Paper, type TemplateDocType } from './types'

const MAX_HTML = 200 * 1024

export type EditorInitial = { id?: string; docType: TemplateDocType; name: string; html: string; paper: Paper; status: 'active' | 'inactive' }

/** ดาวน์โหลด HTML ของแม่แบบเป็นไฟล์ .html */
export function downloadHtml(name: string, html: string) {
  const url = URL.createObjectURL(new Blob([html], { type: 'text/html;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = `${name.replace(/[\\/:*?"<>|]+/g, '_') || 'template'}.html`
  a.click()
  URL.revokeObjectURL(url)
}

/** อ่านไฟล์ .html ที่ผู้ใช้เลือก (ฝั่ง browser) — คืนข้อความ error หรือเนื้อไฟล์ */
export async function readHtmlFile(file: File): Promise<{ html: string } | { error: string }> {
  const e = texts.printTemplates.errors
  if (!/\.html?$/i.test(file.name)) return { error: e.fileType }
  if (file.size > MAX_HTML) return { error: e.fileTooLarge }
  const html =
    typeof file.text === 'function'
      ? await file.text()
      : await new Promise<string>((resolve, reject) => {
          const r = new FileReader()
          r.onload = () => resolve(String(r.result))
          r.onerror = () => reject(r.error)
          r.readAsText(file)
        })
  return { html }
}

/**
 * แก้แม่แบบ: โค้ด HTML (ซ้าย, เลขบรรทัด) + ตัวอย่างกระดาษ (ขวา, iframe sandbox) + ตัวแปร / ตัวช่วยคลิกแทรกที่ตำแหน่งเคอร์เซอร์
 * สร้างใหม่: เลือกชนิดเอกสาร, นำเข้าไฟล์ .html ได้, ตั้งเป็นค่าเริ่มต้นทันทีได้
 */
export function TemplateEditor({ companyId, initial, listHref, canManage }: { companyId: string; initial: EditorInitial; listHref: string; canManage: boolean }) {
  const t = texts.printTemplates
  const router = useRouter()
  const create = !initial.id
  const [docType, setDocType] = useState<TemplateDocType>(initial.docType)
  const [name, setName] = useState(initial.name)
  const [paper, setPaper] = useState<Paper>(initial.paper)
  const [status, setStatus] = useState(initial.status)
  const [isDefault, setIsDefault] = useState(false)
  const [html, setHtml] = useState(initial.html)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [assets, setAssets] = useState<AssetMap>({})
  const code = useRef<HTMLTextAreaElement>(null)
  const gutter = useRef<HTMLDivElement>(null)

  useEffect(() => {
    void loadAssetMap(companyId).then(setAssets)
  }, [companyId])

  const preview = useMemo(() => {
    try {
      return { html: renderDocument(html, paper, sampleData(docType), assets) }
    } catch (err) {
      return { error: format(t.errors.render, { message: err instanceof Error ? err.message : String(err) }) }
    }
  }, [html, paper, docType, assets, t.errors.render])

  const lines = useMemo(() => html.split('\n').length, [html])

  const insert = (snippet: string) => {
    const el = code.current
    if (!el) return setHtml((h) => h + snippet)
    const start = el.selectionStart
    const end = el.selectionEnd
    const next = html.slice(0, start) + snippet + html.slice(end)
    setHtml(next)
    requestAnimationFrame(() => {
      el.focus()
      el.setSelectionRange(start + snippet.length, start + snippet.length)
    })
  }

  const onImport = async (file: File | undefined) => {
    if (!file) return
    const res = await readHtmlFile(file)
    if ('error' in res) return setErrors((e) => ({ ...e, html: res.error }))
    setErrors((e) => ({ ...e, html: '' }))
    setHtml(res.html)
    if (!name.trim()) setName(file.name.replace(/\.html?$/i, ''))
  }

  async function submit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    const errs: Record<string, string> = {}
    if (!name.trim()) errs.name = t.errors.nameRequired
    if (!html.trim()) errs.html = t.errors.htmlRequired
    else if (new Blob([html]).size > MAX_HTML) errs.html = t.errors.fileTooLarge
    setErrors(errs)
    if (Object.keys(errs).length > 0) return
    setBusy(true)
    setFormError(null)
    try {
      const body = { doc_type: docType, name: name.trim(), html, paper, status, is_default: isDefault }
      const saved = create
        ? await apiRequest<DocumentTemplate>(templatesApi(companyId), { method: 'POST', body })
        : await apiRequest<DocumentTemplate>(`${templatesApi(companyId)}/${initial.id}`, { method: 'PUT', body })
      showToast(t.toast.saved)
      if (create) router.push(`${listHref}/${saved.id}`)
      else router.refresh()
    } catch (err) {
      const msg = errorMessage(err)
      // เนื้อหาที่ไม่อนุญาต / HTML ผิด แสดงที่ช่อง HTML
      if (err instanceof Error && 'code' in err && /template/.test(String((err as { code: string }).code))) setErrors({ html: `${msg} (${err.message})` })
      else setFormError(msg)
    } finally {
      setBusy(false)
    }
  }

  const family = familyOf(docType)
  const docTypeOptions = TEMPLATE_DOC_TYPES.map((d) => ({ value: d, label: t.docTypes[d] ?? d }))

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5">
      {formError && <Alert tone="error">{formError}</Alert>}
      <div className="grid gap-4 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card sm:grid-cols-2 lg:grid-cols-4">
        <SelectField
          label={t.fields.docType}
          required
          value={docType}
          disabled={!create || !canManage}
          options={docTypeOptions}
          onChange={(e) => {
            const v = e.target.value
            if (!isTemplateDocType(v)) return
            setDocType(v)
            if (create && html === builtinTemplate(docType).html) {
              setHtml(builtinTemplate(v).html)
              setPaper(builtinTemplate(v).paper)
            }
          }}
        />
        <TextField label={t.fields.name} required value={name} maxLength={100} error={errors.name} disabled={!canManage} onChange={(e) => setName(e.target.value)} />
        <SelectField label={t.fields.paper} value={paper} disabled={!canManage} options={PAPERS.map((p) => ({ value: p, label: t.papers[p] ?? p }))} onChange={(e) => setPaper(e.target.value as Paper)} />
        {create ? (
          <label className="flex items-center gap-2 self-end pb-2 text-sm">
            <input type="checkbox" checked={isDefault} disabled={!canManage} onChange={(e) => setIsDefault(e.target.checked)} className="size-4 accent-primary" />
            {t.fields.isDefault}
          </label>
        ) : (
          <SelectField
            label={t.fields.status}
            value={status}
            disabled={!canManage}
            options={[
              { value: 'active', label: texts.common.statuses.active! },
              { value: 'inactive', label: texts.common.statuses.inactive! },
            ]}
            onChange={(e) => setStatus(e.target.value as 'active' | 'inactive')}
          />
        )}
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        {/* โค้ด */}
        <section aria-labelledby="tpl-code" className="flex min-w-0 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 id="tpl-code" className="text-[15px] font-semibold">
              {t.editor.code}
            </h2>
            <div className="ml-auto flex flex-wrap gap-2">
              {canManage && (
                <label className="inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-(--radius-control) border border-line-strong bg-surface px-3 text-sm font-medium hover:bg-subtle focus-within:ring-2 focus-within:ring-primary">
                  <Upload aria-hidden className="size-4" />
                  {t.actions.import}
                  <input type="file" accept=".html,.htm,text/html" className="sr-only" onChange={(e) => void onImport(e.target.files?.[0])} />
                </label>
              )}
              <Button type="button" variant="secondary" onClick={() => downloadHtml(name, html)}>
                <Download aria-hidden />
                {t.actions.export}
              </Button>
            </div>
          </div>
          <div className={`flex min-h-[32rem] overflow-hidden rounded-(--radius-panel) border bg-surface font-mono text-[13px] leading-5 ${errors.html ? 'border-danger' : 'border-line'}`}>
            <div ref={gutter} aria-hidden className="select-none overflow-hidden border-r border-line bg-subtle px-2 py-3 text-right text-ink-muted tabular-nums">
              {Array.from({ length: lines }, (_, i) => (
                <div key={i}>{i + 1}</div>
              ))}
            </div>
            <textarea
              ref={code}
              aria-label={t.fields.html}
              aria-invalid={errors.html ? true : undefined}
              aria-describedby={errors.html ? 'tpl-html-error' : undefined}
              value={html}
              readOnly={!canManage}
              spellCheck={false}
              wrap="off"
              onChange={(e) => setHtml(e.target.value)}
              onScroll={(e) => {
                if (gutter.current) gutter.current.scrollTop = e.currentTarget.scrollTop
              }}
              className="min-h-[32rem] w-full resize-y bg-transparent px-3 py-3 outline-none"
            />
          </div>
          {errors.html && (
            <p id="tpl-html-error" role="alert" className="text-sm text-danger">
              {errors.html}
            </p>
          )}
          {canManage && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Palette title={t.editor.variables} items={VARIABLES[family].map((v) => ({ snippet: variableSnippet(v), label: v.label, code: v.path }))} onPick={insert} insertLabel={t.actions.insert} />
              <Palette title={t.editor.helpers} items={HELPERS.map((h) => ({ snippet: h.snippet, label: h.label, code: h.snippet }))} onPick={insert} insertLabel={t.actions.insert} />
            </div>
          )}
        </section>

        {/* ตัวอย่าง */}
        <section aria-labelledby="tpl-preview" className="flex min-w-0 flex-col gap-2">
          <h2 id="tpl-preview" className="text-[15px] font-semibold">
            {t.editor.preview} <span className="text-sm font-normal text-ink-muted">· {t.sampleData}</span>
          </h2>
          <div className="overflow-auto rounded-(--radius-panel) border border-line bg-neutral-100 p-3">
            {'error' in preview ? (
              <Alert tone="error">{preview.error}</Alert>
            ) : (
              <iframe title={t.editor.preview} sandbox="allow-same-origin" srcDoc={preview.html} className="block h-[60rem] w-full min-w-[220mm] border-0 bg-transparent" />
            )}
          </div>
        </section>
      </div>

      <div className="sticky bottom-0 z-10 flex justify-end gap-2 border-t border-line bg-canvas/95 py-3 backdrop-blur">
        <ButtonLink href={listHref} variant="secondary">
          {texts.common.cancel}
        </ButtonLink>
        {canManage && (
          <Button type="submit" loading={busy} loadingText={t.actions.saving}>
            {t.actions.save}
          </Button>
        )}
      </div>
    </form>
  )
}

function Palette({ title, items, onPick, insertLabel }: { title: string; items: { snippet: string; label: string; code: string }[]; onPick: (s: string) => void; insertLabel: string }) {
  return (
    <div className="rounded-(--radius-panel) border border-line bg-surface">
      <h3 className="border-b border-line px-3 py-2 text-sm font-semibold">{title}</h3>
      <ul className="max-h-64 overflow-auto py-1">
        {items.map((it) => (
          <li key={it.code}>
            <button
              type="button"
              onClick={() => onPick(it.snippet)}
              aria-label={`${insertLabel} ${it.code}`}
              className="flex w-full cursor-pointer items-baseline justify-between gap-3 px-3 py-1.5 text-left hover:bg-subtle focus-visible:bg-subtle focus-visible:outline-none"
            >
              <code className="truncate font-mono text-[12.5px] text-primary">{it.code}</code>
              <span className="shrink-0 text-[12.5px] text-ink-muted">{it.label}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
