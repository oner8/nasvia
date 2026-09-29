import { forwardRef } from 'react'
import type { InputHTMLAttributes, LabelHTMLAttributes, TextareaHTMLAttributes } from 'react'

import { cn } from '../../lib/utils'

const fieldBase =
  'w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] px-3 py-2 text-sm text-[var(--color-card-foreground)] outline-none transition placeholder:text-[var(--color-muted-foreground)] focus-visible:border-[var(--color-primary)] focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40 disabled:cursor-not-allowed disabled:opacity-60'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...props }, ref) {
    return <input ref={ref} className={cn(fieldBase, 'h-10', className)} {...props} />
  },
)

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function Textarea({ className, ...props }, ref) {
    return <textarea ref={ref} className={cn(fieldBase, 'min-h-24 resize-y leading-relaxed', className)} {...props} />
  },
)

export const Label = forwardRef<HTMLLabelElement, LabelHTMLAttributes<HTMLLabelElement>>(
  function Label({ className, ...props }, ref) {
    return (
      <label
        ref={ref}
        className={cn('mb-1.5 block text-xs font-medium text-[var(--color-muted-foreground)]', className)}
        {...props}
      />
    )
  },
)

export const Checkbox = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Checkbox({ className, ...props }, ref) {
    return (
      <input
        ref={ref}
        type="checkbox"
        className={cn(
          'size-4 cursor-pointer rounded border border-[var(--color-border)] accent-[var(--color-primary)]',
          className,
        )}
        {...props}
      />
    )
  },
)
