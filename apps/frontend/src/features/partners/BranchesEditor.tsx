'use client'

import { Plus, Trash2 } from 'lucide-react'
import { Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { ThaiAddressFields } from './ThaiAddressFields'
import { cleanAddress, emptyThaiAddress, type ThaiAddress } from './thaiAddress'

/** สาขาเพิ่มเติมของลูกค้า / ผู้ขาย (สาขาหลัก = เลขสาขา / ที่อยู่ในส่วนข้อมูลภาษีด้านบน) */
export type BranchDraft = { key: number; branch_no: string; name: string; address: ThaiAddress }

let nextKey = 1
export const newBranch = (p: Partial<Omit<BranchDraft, 'key'>> = {}): BranchDraft => ({
  key: nextKey++,
  branch_no: '',
  name: '',
  address: emptyThaiAddress,
  ...p,
})

export function branchesFrom(list: { branch_no: string; name: string; address_detail: Partial<ThaiAddress> }[] | undefined): BranchDraft[] {
  return (list ?? []).map((b) => newBranch({ branch_no: b.branch_no, name: b.name, address: cleanAddress(b.address_detail) }))
}

export function branchesPayload(list: BranchDraft[]) {
  return list.map((b) => ({ branch_no: b.branch_no, name: b.name.trim(), address_detail: cleanAddress(b.address) }))
}

/** ตรวจก่อนส่ง: เลข 5 หลัก, ไม่ซ้ำกันและไม่ซ้ำสาขาหลัก — คืน error ต่อ key */
export function branchErrors(list: BranchDraft[], mainNo: string): Record<number, string> {
  const t = texts.partners.branches
  const seen = new Set([mainNo])
  const out: Record<number, string> = {}
  for (const b of list) {
    if (!/^\d{5}$/.test(b.branch_no)) out[b.key] = texts.validation.taxBranchNo
    else if (seen.has(b.branch_no)) out[b.key] = t.duplicate
    seen.add(b.branch_no)
  }
  return out
}

export function BranchesEditor({
  value,
  onChange,
  errors = {},
  disabled,
}: {
  value: BranchDraft[]
  onChange: (v: BranchDraft[]) => void
  errors?: Record<number, string>
  disabled?: boolean
}) {
  const t = texts.partners.branches
  const up = (key: number, patch: Partial<BranchDraft>) => onChange(value.map((b) => (b.key === key ? { ...b, ...patch } : b)))
  return (
    <div className="flex flex-col gap-4 sm:col-span-2">
      {value.length === 0 && <p className="text-sm text-ink-muted">{t.empty}</p>}
      {value.map((b, i) => (
        <fieldset key={b.key} className="grid gap-4 rounded-(--radius-control) border border-line p-4 sm:grid-cols-2">
          <legend className="px-1 text-[13px] font-medium text-ink-muted">{format(t.title, { n: String(i + 1) })}</legend>
          <TextField
            label={t.branchNo}
            required            value={b.branch_no}
            maxLength={5}
            inputMode="numeric"
            disabled={disabled}
            error={errors[b.key]}
            onChange={(e) => up(b.key, { branch_no: e.target.value.replace(/\D/g, '') })}
          />
          <TextField label={t.name} value={b.name} maxLength={200} disabled={disabled} onChange={(e) => up(b.key, { name: e.target.value })} />
          <ThaiAddressFields value={b.address} onChange={(address) => up(b.key, { address })} disabled={disabled} />
          {!disabled && (
            <div className="sm:col-span-2">
              <Button type="button" variant="danger" size="sm" onClick={() => onChange(value.filter((x) => x.key !== b.key))}>
                <Trash2 aria-hidden />
                {format(t.remove, { n: b.branch_no || String(i + 1) })}
              </Button>
            </div>
          )}
        </fieldset>
      ))}
      {!disabled && (
        <div>
          <Button type="button" variant="secondary" onClick={() => onChange([...value, newBranch()])}>
            <Plus aria-hidden />
            {t.add}
          </Button>
        </div>
      )}
    </div>
  )
}
