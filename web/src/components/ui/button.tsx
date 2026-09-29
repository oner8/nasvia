import { forwardRef } from 'react'
import type { ButtonHTMLAttributes } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '../../lib/utils'

const buttonVariants = cva(
  'inline-flex shrink-0 select-none items-center justify-center gap-2 whitespace-nowrap rounded-xl font-medium outline-none transition-all duration-150 focus-visible:ring-2 focus-visible:ring-[var(--color-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-background)] disabled:pointer-events-none disabled:opacity-50 active:scale-[0.98]',
  {
    variants: {
      variant: {
        default: 'bg-[var(--color-primary)] text-[var(--color-primary-foreground)] shadow-soft hover:brightness-110',
        outline:
          'border border-[var(--color-border)] bg-[var(--color-card)] text-[var(--color-card-foreground)] hover:bg-[var(--color-muted)]',
        ghost: 'text-[var(--color-foreground)] hover:bg-[var(--color-muted)]',
        subtle: 'bg-[var(--color-muted)] text-[var(--color-foreground)] hover:bg-[var(--color-accent)]',
        danger: 'bg-[var(--color-danger)] text-[var(--color-danger-foreground)] hover:brightness-110',
        link: 'text-[var(--color-primary)] underline-offset-4 hover:underline',
      },
      size: {
        sm: 'h-8 px-3 text-xs',
        md: 'h-10 px-4 text-sm',
        lg: 'h-11 px-5 text-sm',
        icon: 'h-9 w-9',
        'icon-sm': 'h-8 w-8',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  },
)

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant, size, type = 'button', ...props },
  ref,
) {
  return <button ref={ref} type={type} className={cn(buttonVariants({ variant, size }), className)} {...props} />
})

export { buttonVariants }
