import Link from 'next/link'
import type { ComponentProps } from 'react'
import { cx } from '@/lib/cx'
import { buttonClass, type ButtonSize, type ButtonVariant } from '../Button/Button'

export type ButtonLinkProps = ComponentProps<typeof Link> & {
  variant?: ButtonVariant
  size?: ButtonSize
}

/** ลิงก์ที่หน้าตาเป็นปุ่ม (เช่น "สร้าง") — ใช้แทน <Button onClick={router.push}> เพื่อให้เปิดแท็บใหม่ / คัดลอกลิงก์ได้ */
export function ButtonLink({ variant = 'primary', size = 'md', className, ...rest }: ButtonLinkProps) {
  return <Link {...rest} className={cx(buttonClass(variant, size), className)} />
}
