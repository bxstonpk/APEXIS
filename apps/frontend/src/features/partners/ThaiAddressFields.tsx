'use client'

import { TextField } from '@/components'
import { texts } from '@/config'
import { formatThaiAddress, isBangkok, type ThaiAddress } from './thaiAddress'

/**
 * ช่องที่อยู่ตามแบบกรมสรรพากร — เรียงตามลำดับที่พิมพ์บนเอกสาร
 * ป้ายตำบล / อำเภอเปลี่ยนเป็น แขวง / เขต เมื่อจังหวัดเป็นกรุงเทพมหานคร; ด้านล่างแสดงที่อยู่ที่จะพิมพ์บนใบกำกับภาษี
 */
export function ThaiAddressFields({
  value,
  onChange,
  disabled,
  postalError,
}: {
  value: ThaiAddress
  onChange: (v: ThaiAddress) => void
  disabled?: boolean
  postalError?: string
}) {
  const f = texts.partners.address
  const bkk = isBangkok(value.province)
  const set = (k: keyof ThaiAddress) => (e: { target: { value: string } }) => onChange({ ...value, [k]: e.target.value })
  const field = (k: keyof ThaiAddress, label: string, max = 100, extra: Partial<Parameters<typeof TextField>[0]> = {}) => (
    <TextField key={k} label={label} value={value[k]} maxLength={max} disabled={disabled} onChange={set(k)} {...extra} />
  )
  const preview = formatThaiAddress(value)
  return (
    <>
      <div className="grid gap-4 sm:col-span-2 sm:grid-cols-2 lg:grid-cols-3">
        {field('house_no', f.houseNo)}
        {field('moo', f.moo)}
        {field('village', f.village)}
        {field('building', f.building)}
        {field('room_no', f.roomNo)}
        {field('floor_no', f.floorNo)}
        {field('soi', f.soi)}
        {field('yaek', f.yaek)}
        {field('street', f.street)}
        {field('sub_district', bkk ? f.khwaeng : f.subDistrict)}
        {field('district', bkk ? f.khet : f.district)}
        {field('province', f.province)}
        {field('postal_code', f.postalCode, 5, { inputMode: 'numeric', error: postalError })}
      </div>
      <div className="rounded-(--radius-control) border border-dashed border-line-strong bg-subtle px-3 py-2.5 text-sm sm:col-span-2">
        <p className="text-[13px] text-ink-muted">{f.preview}</p>
        <p className="mt-0.5 break-words">{preview || <span className="text-ink-muted">–</span>}</p>
      </div>
    </>
  )
}
