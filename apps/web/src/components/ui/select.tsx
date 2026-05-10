import type * as React from 'react'

import { cn } from '@/lib/utils'

function Select({ className, ...props }: React.ComponentProps<'select'>) {
  return (
    <select
      data-slot="select"
      className={cn(
        'flex min-h-12 w-full rounded-[14px] border border-input bg-[var(--surface-2)] px-4 py-3 text-base text-foreground transition-[border-color,background,transform] outline-none focus-visible:border-ring focus-visible:bg-[var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

export { Select }
