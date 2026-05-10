import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import type * as React from 'react'

import { cn } from '@/lib/utils'

const buttonVariants = cva(
  "inline-flex min-h-11 shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-full border border-transparent px-4 py-2.5 text-sm font-bold transition-[background,border-color,color,transform] outline-none disabled:pointer-events-none disabled:opacity-50 disabled:hover:translate-y-0 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground shadow-[0_10px_24px_oklch(7%_0.022_336_/_0.26)] hover:-translate-y-px hover:bg-[var(--primary-hover)]',
        destructive:
          'bg-destructive text-[var(--paper)] hover:-translate-y-px hover:bg-[var(--destructive-hover)] focus-visible:ring-destructive/20',
        outline:
          'border-border bg-[var(--surface-2)] text-[var(--paper)] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-3)]',
        secondary:
          'bg-secondary text-secondary-foreground hover:-translate-y-px hover:bg-[var(--thread-teal)]',
        ghost: 'text-[var(--paper-muted)] hover:bg-[var(--surface-2)] hover:text-[var(--paper)]',
        link: 'text-primary underline-offset-4 hover:underline',
        plain:
          'min-h-0 justify-start whitespace-normal rounded-none border-0 bg-transparent p-0 text-inherit hover:bg-transparent hover:text-inherit',
      },
      size: {
        default: 'h-10',
        sm: 'h-9 px-3',
        lg: 'h-12 px-7',
        icon: 'size-11 p-0',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : 'button'

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
