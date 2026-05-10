import type * as React from 'react'

import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      data-slot="input"
      type={type}
      className={cn(
        'flex min-h-12 w-full rounded-[14px] border border-input bg-[var(--surface-2)] px-4 py-3 text-base text-foreground transition-[border-color,background,transform] outline-none placeholder:text-[color-mix(in_oklch,var(--paper-muted)_62%,var(--background))] focus-visible:border-ring focus-visible:bg-[var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
