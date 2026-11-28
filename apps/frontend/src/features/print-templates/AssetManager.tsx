'use client'

import { Copy, Trash2, Upload } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Alert, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { assetsApi, assetUrl } from './TemplatedPrint'
import type { DocumentAsset } from './types'

const MAX_ASSET = 500 * 1024
const ASSET_NAME = /^[a-z0-9][a-z0-9_-]{0,39}$/

/** ตรวจไฟล์รูปก่อนอัปโหลด (backend ตรวจเนื้อไฟล์จริงอีกชั้น) */
export function checkAsset(name: string, file: File | null): Record<string, string> {
  const e = texts.printTemplates.errors
  const errs: Record<string, string> = {}
  if (!ASSET_NAME.test(name)) errs.name = e.assetName
  if (!file) errs.file = e.assetType
  else if (!/^image\/(png|jpeg|svg\+xml)$/.test(file.type)) errs.file = e.assetType
  else if (file.size > MAX_ASSET) errs.file = e.assetTooLarge
  return errs
}

const toBase64 = (file: File) =>
  new Promise<string>((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result))
    r.onerror = () => reject(r.error)
    r.readAsDataURL(file)
  })

/** รูปของแม่แบบ (โลโก้ / ลายเซ็น / ตราประทับ) — อัปโหลด / ลบ / คัดลอก {{asset "ชื่อ"}} */
export function AssetManager({ companyId, items, canManage }: { companyId: string; items: DocumentAsset[]; canManage: boolean }) {
  const t = texts.printTemplates
  const a = t.assets
  const router = useRouter()
  const [name, setName] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function upload(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    const errs = checkAsset(name.trim(), file)
    setErrors(errs)
    if (Object.keys(errs).length > 0 || !file) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(assetsApi(companyId), { method: 'POST', body: { name: name.trim(), data: await toBase64(file) } })
      showToast(t.toast.uploaded)
      setName('')
      setFile(null)
      ev.currentTarget?.reset()
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function remove(x: DocumentAsset) {
    if (!window.confirm(format(t.confirm.deleteAsset, { name: x.name }))) return
    try {
      await apiRequest(`${assetsApi(companyId)}/${x.id}`, { method: 'DELETE' })
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const snippet = (x: DocumentAsset) => `<img src="{{asset "${x.name}"}}" alt="${x.name}">`

  return (
    <section aria-labelledby="tpl-assets" className="rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <h2 id="tpl-assets" className="border-b border-line bg-subtle px-4 py-2.5 text-[15px] font-semibold">
        {a.heading}
      </h2>
      <div className="flex flex-col gap-4 p-4">
        {error && <Alert tone="error">{error}</Alert>}
        {canManage && (
          <form onSubmit={upload} noValidate className="grid items-start gap-3 sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_auto]">
            <TextField label={a.name} required value={name} maxLength={40} error={errors.name} onChange={(e) => setName(e.target.value.toLowerCase())} />
            <div className="flex flex-col gap-1.5">
              <label htmlFor="tpl-asset-file" className="text-sm font-medium">
                {a.file}
                <span className="ml-0.5 text-danger">*</span>
              </label>
              <input
                id="tpl-asset-file"
                type="file"
                accept="image/png,image/jpeg,image/svg+xml"
                aria-invalid={errors.file ? true : undefined}
                aria-describedby={errors.file ? 'tpl-asset-file-error' : undefined}
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                className="h-10 text-sm file:mr-3 file:h-10 file:cursor-pointer file:rounded-(--radius-control) file:border file:border-line-strong file:bg-surface file:px-3"
              />
              {errors.file && (
                <p id="tpl-asset-file-error" role="alert" className="text-sm text-danger">
                  {errors.file}
                </p>
              )}
            </div>
            <Button type="submit" loading={busy} className="sm:mt-7">
              <Upload aria-hidden />
              {a.upload}
            </Button>
          </form>
        )}
        {items.length === 0 ? (
          <p className="text-sm text-ink-muted">{a.empty}</p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((x) => (
              <li key={x.id} className="flex items-center gap-3 rounded-(--radius-control) border border-line p-2">
                {/* eslint-disable-next-line @next/next/no-img-element -- รูปจาก API ของบริษัท (ต้องใช้ cookie) */}
                <img src={assetUrl(companyId, x.id)} alt={x.name} className="size-12 shrink-0 rounded border border-line bg-white object-contain" />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-sm">{x.name}</p>
                  <p className="text-xs text-ink-muted">
                    {x.content_type.replace('image/', '').replace('+xml', '')} · {Math.ceil(x.size / 1024)} KB
                  </p>
                </div>
                <Button
                  type="button"
                  variant="secondary"
                  aria-label={`${t.actions.copy} ${x.name}`}
                  onClick={() => void navigator.clipboard?.writeText(snippet(x)).then(() => showToast(t.toast.copied))}
                >
                  <Copy aria-hidden />
                </Button>
                {canManage && (
                  <Button type="button" variant="danger" aria-label={`${t.actions.delete} ${x.name}`} onClick={() => void remove(x)}>
                    <Trash2 aria-hidden />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}
