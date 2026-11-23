import { cn } from '@/lib/utils'

/** shadcn/ui Skeleton — ใช้ bg-muted แทน bg-accent (accent ของ APEXIS คือสีส้มโลโก้) */
function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="skeleton" className={cn('animate-pulse rounded-md bg-muted motion-reduce:animate-none', className)} {...props} />
}

export { Skeleton }
