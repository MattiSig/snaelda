import type * as React from 'react'

import { cn } from '@/lib/utils'

function Checkbox({ className, type: _type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      data-slot="checkbox"
      type="checkbox"
      className={cn(
        'size-5 w-5 shrink-0 rounded-[6px] border border-input bg-[var(--surface-2)] accent-primary outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

export { Checkbox }
