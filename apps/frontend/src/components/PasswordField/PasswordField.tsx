'use client'

import { useState } from 'react'
import { TextField, type TextFieldProps } from '../TextField/TextField'

export type PasswordFieldProps = Omit<TextFieldProps, 'type' | 'endAdornment'> & {
  /** ข้อความของปุ่มแสดง / ซ่อนรหัสผ่าน (สำหรับ screen reader และ tooltip) — มาจาก config */
  showLabel: string
  hideLabel: string
}

/** ช่องรหัสผ่านพร้อมปุ่มแสดง / ซ่อน */
export function PasswordField({ showLabel, hideLabel, ...field }: PasswordFieldProps) {
  const [visible, setVisible] = useState(false)
  const toggleLabel = visible ? hideLabel : showLabel
  return (
    <TextField
      {...field}
      type={visible ? 'text' : 'password'}
      endAdornment={
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-label={toggleLabel}
          aria-pressed={visible}
          title={toggleLabel}
          className="inline-flex size-9 cursor-pointer items-center justify-center rounded-md text-ink-muted outline-none hover:text-ink focus-visible:ring-3 focus-visible:ring-primary/35"
        >
          {visible ? <EyeOffIcon /> : <EyeIcon />}
        </button>
      }
    />
  )
}

function EyeIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}

function EyeOffIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-5" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M17.94 17.94A10.94 10.94 0 0 1 12 19C5.5 19 2 12 2 12a19.8 19.8 0 0 1 5.06-5.94" />
      <path d="M9.9 4.24A10.4 10.4 0 0 1 12 4c6.5 0 10 7 10 7a19.9 19.9 0 0 1-3.17 4.19" />
      <path d="M14.12 14.12a3 3 0 1 1-4.24-4.24" />
      <path d="M2 2l20 20" />
    </svg>
  )
}
