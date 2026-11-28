import type { Metadata } from 'next'
import { texts } from '@/config'

export const metadata: Metadata = { title: texts.purchasing.print.workOrder }

/** พิมพ์ใบสั่งจ้าง — หน้าเดียวกับใบสั่งซื้อ (เลือกแม่แบบ purchasing.work_order ตามชนิดของใบสั่ง) */
export { default } from '../../../orders/[docId]/print/page'
