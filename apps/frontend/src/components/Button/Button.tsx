import type { ButtonHTMLAttributes } from 'react'
import { cx } from '@/lib/cx'
import { Spinner } from '../Spinner/Spinner'

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  size?: ButtonSize
  fullWidth?: boolean
  /** แสดง spinner และปิดการกด — ข้อความระหว่างโหลดส่งผ่าน loadingText (มาจาก config) */
  loading?: boolean
  loadingText?: string
}

const variants = {
  primary: 'bg-primary text-on-primary hover:enabled:bg-primary-hover [a]:hover:bg-primary-hover',
  secondary: 'border-line-strong bg-surface text-ink hover:enabled:border-primary [a]:hover:border-primary',
  ghost: 'bg-transparent text-primary hover:enabled:underline [a]:hover:underline',
  danger: 'border-danger-line bg-surface text-danger hover:enabled:bg-danger-soft',
} as const

const sizes = {
  md: 'min-h-11 px-6',
  sm: 'min-h-9 px-4 text-sm',
} as const

export type ButtonVariant = keyof typeof variants
export type ButtonSize = keyof typeof sizes

/** class ของปุ่ม — ใช้ร่วมกับ ButtonLink (ลิงก์ที่หน้าตาเป็นปุ่ม) */
export function buttonClass(variant: ButtonVariant = 'primary', size: ButtonSize = 'md', fullWidth = false): string {
  return cx(
    'inline-flex cursor-pointer items-center justify-center gap-2 rounded-lg border border-transparent font-semibold whitespace-nowrap transition-colors',
    'outline-none focus-visible:ring-3 focus-visible:ring-primary/35 disabled:cursor-not-allowed disabled:opacity-70',
    sizes[size],
    variants[variant],
    fullWidth && 'w-full',
  )
}

/** ปุ่มพื้นฐาน — ไม่มีข้อความในตัว ข้อความทั้งหมดส่งมาจากผู้ใช้ component */
export function Button({
  variant = 'primary',
  size = 'md',
  fullWidth = false,
  loading = false,
  loadingText,
  disabled,
  className,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cx(buttonClass(variant, size, fullWidth), className)}
    >
      {loading && <Spinner />}
      <span>{loading && loadingText ? loadingText : children}</span>
    </button>
  )
}
