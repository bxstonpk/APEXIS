import type { ButtonHTMLAttributes } from 'react'
import { cx } from '@/lib/cx'
import { Spinner } from '../Spinner/Spinner'

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost'
  fullWidth?: boolean
  /** แสดง spinner และปิดการกด — ข้อความระหว่างโหลดส่งผ่าน loadingText (มาจาก config) */
  loading?: boolean
  loadingText?: string
}

const variants = {
  primary: 'bg-primary text-on-primary hover:enabled:bg-primary-hover',
  secondary: 'border-line-strong bg-surface text-ink hover:enabled:border-primary',
  ghost: 'bg-transparent text-primary hover:enabled:underline',
} as const

/** ปุ่มพื้นฐาน — ไม่มีข้อความในตัว ข้อความทั้งหมดส่งมาจากผู้ใช้ component */
export function Button({
  variant = 'primary',
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
      className={cx(
        'inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-lg border border-transparent px-6 font-semibold transition-colors',
        'outline-none focus-visible:ring-3 focus-visible:ring-primary/35 disabled:cursor-not-allowed disabled:opacity-70',
        variants[variant],
        fullWidth && 'w-full',
        className,
      )}
    >
      {loading && <Spinner />}
      <span>{loading && loadingText ? loadingText : children}</span>
    </button>
  )
}
