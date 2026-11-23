'use client'

import Form from 'next/form'
import { useState } from 'react'
import { Button, SelectField, TextField } from '@/components'
import { texts } from '@/config'
import { PRESETS, type Period } from './period'

/** ตัวเลือกช่วงเวลา (แถวเดียวเหนือกราฟ) — ส่งเป็น GET query: กด Back แล้วได้ช่วงเดิม; กำหนดเองแสดงช่องวันที่ */
export function PeriodFilter({ period }: { period: Period }) {
  const t = texts.dashboard.period
  const [preset, setPreset] = useState(period.preset)
  return (
    <Form action="" className="flex flex-wrap items-end gap-2 print:hidden">
      <SelectField
        name="period"
        label={t.label}
        value={preset}
        onChange={(e) => setPreset(e.target.value as Period['preset'])}
        options={PRESETS.map((p) => ({ value: p, label: t[p] }))}
        className="w-40"
      />
      {preset === 'custom' && (
        <>
          <TextField name="start_date" type="date" label={t.start} defaultValue={period.start} required />
          <TextField name="end_date" type="date" label={t.end} defaultValue={period.end} required />
        </>
      )}
      <Button type="submit" variant="secondary">
        {t.apply}
      </Button>
    </Form>
  )
}
