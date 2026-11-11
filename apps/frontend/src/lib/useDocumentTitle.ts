import { useEffect } from 'react'
import { texts } from '@/config'

/** ตั้งชื่อแท็บของ browser เป็น "<ชื่อหน้า> | <ชื่อแอป>" — ชื่อมาจาก config */
export function useDocumentTitle(title: string) {
  useEffect(() => {
    document.title = `${title} | ${texts.app.name}`
  }, [title])
}
