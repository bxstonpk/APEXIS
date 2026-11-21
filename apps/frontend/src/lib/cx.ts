/** รวม class ของ Tailwind ที่มีเงื่อนไข เช่น cx('px-4', error && 'border-danger') */
export function cx(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(' ')
}
