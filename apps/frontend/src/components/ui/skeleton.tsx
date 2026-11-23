import { cn } from '@/lib/utils'

/** shadcn/ui Skeleton — ใช้ bg-line แทน bg-accent (accent ของ APEXIS คือสีส้มโลโก้; muted จางจนกลืนกับพื้นหลังหน้า) */
function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="skeleton" className={cn('animate-pulse rounded-md bg-line motion-reduce:animate-none', className)} {...props} />
}

export { Skeleton }
