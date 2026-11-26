'use client'

import { useRouter } from 'next/navigation'
import { useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { EditTable } from './BomForm'
import type { Bom, ProductionOrder, Ref } from './types'

/**
 * ใบสั่งผลิต (สร้าง / แก้ร่าง)
 *   - เลือกสินค้า → ดึงสูตรที่ใช้งานของสินค้า (GET /boms?item_id=&status=active) แล้วเลือกสูตรหลักให้
 *   - คลังตั้งต้นจากค่าตั้งการผลิต
 *   - แก้ร่าง: PUT พร้อมจำนวนวัตถุดิบที่ต้องใช้ (materials) — สร้างใหม่ backend คำนวณจากสูตร × จำนวน
 */
export function OrderForm({
  apiPath,
  bomsApi,
  itemsApi,
  warehousesApi,
  docsPath,
  listHref,
  defaultWarehouse,
  order,
}: {
  apiPath: string
  bomsApi: string
  itemsApi: string
  warehousesApi: string
  docsPath: string
  listHref: string
  defaultWarehouse?: Ref | null
  order?: ProductionOrder
}) {
  const t = texts.manufacturing.orders
  const f = t.fields
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [head, setHead] = useState({
    date: order?.date ?? today(),
    quantity: order?.quantity ?? '',
    start: order?.planned_start ?? '',
    end: order?.planned_end ?? '',
    description: order?.description ?? '',
  })
  const [item, setItem] = useState<Ref | null>(order?.item ?? null)
  const [bom, setBom] = useState<Ref | null>(order?.bom ?? null)
  const [noBom, setNoBom] = useState(false)
  const [warehouse, setWarehouse] = useState<Ref | null>(order?.warehouse ?? defaultWarehouse ?? null)
  const [materials, setMaterials] = useState(() => (order?.materials ?? []).map((m) => ({ item: m.item, required: m.required_quantity })))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function pickItem(v: Ref | null) {
    setItem(v)
    setBom(null)
    setNoBom(false)
    if (!v) return
    try {
      const boms = await apiRequest<Bom[]>(`${bomsApi}?${new URLSearchParams({ item_id: v.id, status: 'active', page_size: '50' })}`)
      const pick = boms.find((b) => b.is_default) ?? boms[0]
      if (pick) setBom({ id: pick.id, code: pick.code, name: pick.name })
      else setNoBom(true)
    } catch {
      setNoBom(false)
    }
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    const errs: Record<string, string> = {}
    if (!head.date) errs.date = requiredMessage(f.date)
    if (!item) errs.item = requiredMessage(f.item)
    if (!bom) errs.bom = noBom ? t.noBom : requiredMessage(f.bom)
    const q = normalizeAmount(head.quantity)
    if (!q || toUnits(q) <= 0n) errs.quantity = texts.manufacturing.boms.errors.qtyInvalid
    if (!warehouse) errs.warehouse = requiredMessage(f.warehouse)
    if (head.start && head.end && head.end < head.start) errs.end = t.planRange
    materials.forEach((m, i) => {
      const r = normalizeAmount(m.required)
      if (r === null || r === '' || toUnits(r) < 0n) errs[`m-${i}`] = t.panels.qtyInvalid
    })
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      date: head.date,
      item_id: item!.id,
      bom_id: bom!.id,
      quantity: q,
      warehouse_id: warehouse!.id,
      planned_start: head.start || null,
      planned_end: head.end || null,
      description: head.description.trim(),
    }
    if (order) body.materials = materials.map((m) => ({ item_id: m.item.id, required_quantity: normalizeAmount(m.required) }))
    setBusy(true)
    try {
      const id = order ? (await apiRequest(`${apiPath}/${order.id}`, { method: 'PUT', body }), order.id) : (await apiRequest<{ id: string }>(apiPath, { method: 'POST', body })).id
      showToast(texts.common.toast.saved)
      router.push(`${docsPath}/${id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      <FormSection title={f.item}>
        <ReferenceSelect
          label={f.item}
          required
          source={itemsApi}
          params={{ status: 'active', type: 'stock' }}
          value={item}
          onChange={(v) => void pickItem(v)}
          error={errors.item}
          texts={texts.reference}
        />
        <ReferenceSelect
          label={f.bom}
          required
          source={bomsApi}
          params={item ? { item_id: item.id, status: 'active' } : { status: 'active' }}
          value={bom}
          onChange={setBom}
          disabled={!item}
          error={errors.bom ?? (noBom ? t.noBom : undefined)}
          texts={texts.reference}
        />
        <TextField
          label={f.quantity}
          required
          inputMode="decimal"
          value={head.quantity}
          error={errors.quantity}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => setHead((h) => ({ ...h, quantity: e.target.value }))}
        />
        <ReferenceSelect
          label={f.warehouse}
          required
          source={warehousesApi}
          params={{ status: 'active' }}
          value={warehouse}
          onChange={setWarehouse}
          error={errors.warehouse}
          texts={texts.reference}
        />
      </FormSection>

      <FormSection title={f.plan}>
        <TextField label={f.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <span className="hidden sm:block" />
        <TextField label={f.plannedStart} type="date" value={head.start} onChange={(e) => setHead((h) => ({ ...h, start: e.target.value }))} />
        <TextField label={f.plannedEnd} type="date" value={head.end} error={errors.end} onChange={(e) => setHead((h) => ({ ...h, end: e.target.value }))} />
        <div className="sm:col-span-2">
          <TextField label={f.description} value={head.description} maxLength={500} onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
        </div>
      </FormSection>

      {order && materials.length > 0 && (
        <section aria-labelledby="order-materials" className="flex min-w-0 flex-col gap-3">
          <h2 id="order-materials" className="text-[15px] font-semibold">
            {t.materials.title}
          </h2>
          <EditTable head={[['#', 'w-10 text-center'], [t.materials.item, ''], [t.materials.required, 'w-44 text-right']]}>
            {materials.map((m, i) => (
              <tr key={m.item.id} className="border-b border-line align-top last:border-b-0">
                <td className="px-3 pt-4 text-center text-[13px] text-ink-subtle tabular-nums">{i + 1}</td>
                <td className="px-2 pt-4">
                  <span className="text-[13px] text-ink-muted">{m.item.code}</span> {m.item.name}
                </td>
                <td className="px-2 py-2">
                  <TextField
                    label={`${t.materials.required} ${m.item.name}`}
                    hideLabel
                    inputMode="decimal"
                    value={m.required}
                    error={errors[`m-${i}`]}
                    className="[&_input]:text-right [&_input]:tabular-nums"
                    onChange={(e) => setMaterials((ms) => ms.map((x, j) => (j === i ? { ...x, required: e.target.value } : x)))}
                  />
                </td>
              </tr>
            ))}
          </EditTable>
        </section>
      )}

      <FormBar>
        <div className="ml-auto flex flex-wrap gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.saveDraft}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
