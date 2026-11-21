'use client'

import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'

export type ResourceFormOptions = {
  /** API ของรายการ เช่น /companies/{id}/departments */
  apiPath: string
  /** ไม่มี = สร้างใหม่ (POST); มี = แก้ไข (PUT apiPath/{id}) และลบได้ */
  id?: string
  /** หน้ารายการ (พร้อมเงื่อนไขเดิม) — ไปที่นี่หลังบันทึก / ลบสำเร็จ */
  listHref: string
}

/**
 * การบันทึก / ลบของฟอร์มข้อมูลหลัก: เรียก API ผ่าน /api/v1 แล้วกลับหน้ารายการ
 * error แสดงด้วยข้อความจาก config (texts.errors[error.code])
 */
export function useResourceForm({ apiPath, id, listHref }: ResourceFormOptions) {
  const router = useRouter()
  const [busy, setBusy] = useState<'save' | 'delete' | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  async function run(kind: 'save' | 'delete', request: () => Promise<unknown>) {
    setFormError(null)
    setBusy(kind)
    try {
      await request()
      const t = texts.common.toast
      showToast(kind === 'delete' ? t.deleted : id ? t.saved : t.created)
      router.push(listHref)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(null)
    }
  }

  return {
    busy,
    formError,
    setFormError,
    save: (body: unknown) =>
      run('save', () => (id ? apiRequest(`${apiPath}/${id}`, { method: 'PUT', body }) : apiRequest(apiPath, { method: 'POST', body }))),
    remove: (confirmText: string) => {
      if (!id || !window.confirm(confirmText)) return
      return run('delete', () => apiRequest(`${apiPath}/${id}`, { method: 'DELETE' }))
    },
  }
}

/** ค่าว่าง = null (ช่องอ้างอิง / วันที่ที่ไม่บังคับ) */
export function orNull(value: string | null | undefined): string | null {
  return value ? value : null
}
