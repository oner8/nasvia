import type { HTMLAttributes, ReactNode } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium leading-5 whitespace-nowrap',
  {
    variants: {
      variant: {
        default: 'bg-[var(--color-muted)] text-[var(--color-muted-foreground)]',
        primary: 'bg-[var(--color-primary)]/12 text-[var(--color-primary)]',
        success: 'bg-[var(--color-success)]/15 text-[var(--color-success)]',
        danger: 'bg-[var(--color-danger)]/14 text-[var(--color-danger)]',
        outline: 'border border-[var(--color-border)] text-[var(--color-muted-foreground)]',
      },
    },
    defaultVariants: { variant: 'default' },
  },
)

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {
  children?: ReactNode
}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'rounded-2xl border border-[var(--color-border)] bg-[var(--color-card)] text-[var(--color-card-foreground)] shadow-soft',
        className,
      )}
      {...props}
    />
  )
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col gap-1 p-5 pb-3', className)} {...props} />
}

export function CardTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return <h3 className={cn('text-base font-semibold tracking-tight', className)} {...props} />
}

export function CardDescription({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn('text-xs leading-relaxed text-[var(--color-muted-foreground)]', className)} {...props} />
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-5 pt-0', className)} {...props} />
}

export function Separator({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('h-px w-full bg-[var(--color-border)]', className)} {...props} />
}
