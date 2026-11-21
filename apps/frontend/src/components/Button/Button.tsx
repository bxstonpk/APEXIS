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
  primary: 'border-primary bg-primary text-on-primary shadow-card hover:border-primary-hover hover:bg-primary-hover',
  secondary: 'border-line-strong bg-surface text-ink shadow-card hover:border-ink-subtle hover:bg-subtle',
  ghost: 'border-transparent text-ink-muted hover:bg-neutral-soft hover:text-ink',
  danger: 'border-transparent text-danger hover:bg-danger-soft',
} as const

const sizes = {
  md: 'h-10 px-4 text-sm',
  sm: 'h-8 px-3 text-[13px]',
} as const

export type ButtonVariant = keyof typeof variants
export type ButtonSize = keyof typeof sizes

/** class ของปุ่ม — ใช้ร่วมกับ ButtonLink (ลิงก์ที่หน้าตาเป็นปุ่ม) */
export function buttonClass(variant: ButtonVariant = 'primary', size: ButtonSize = 'md', fullWidth = false): string {
  return cx(
    'inline-flex cursor-pointer items-center justify-center gap-1.5 rounded-(--radius-control) border font-medium whitespace-nowrap transition-colors',
    'outline-none focus-visible:ring-3 focus-visible:ring-primary/25 disabled:pointer-events-none disabled:opacity-55',
    '[&_svg]:size-4 [&_svg]:shrink-0',
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
      {loading && loadingText ? loadingText : children}
    </button>
  )
}
