import { AnimatePresence, motion } from 'motion/react'
import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-react'

import { cn } from '../lib/utils'
import { useUI } from '../stores/ui'

const ICONS = {
  success: CheckCircle2,
  error: AlertTriangle,
  info: Info,
} as const

/** 轻量 toast 容器（自绘，无额外依赖）。 */
export function Toaster() {
  const toasts = useUI((state) => state.toasts)
  const dismiss = useUI((state) => state.dismissToast)

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[80] flex w-[min(92vw,22rem)] flex-col gap-2">
      <AnimatePresence initial={false}>
        {toasts.map((item) => {
          const Icon = ICONS[item.variant]
          return (
            <motion.div
              key={item.id}
              layout
              initial={{ opacity: 0, y: 12, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 8, scale: 0.97 }}
              transition={{ duration: 0.18, ease: 'easeOut' }}
              className={cn(
                'pointer-events-auto flex items-start gap-3 rounded-xl border bg-[var(--color-card)] p-3 shadow-soft',
                item.variant === 'error' ? 'border-[var(--color-danger)]/40' : 'border-[var(--color-border)]',
              )}
            >
              <Icon
                className={cn(
                  'mt-0.5 size-4 shrink-0',
                  item.variant === 'success'
                    ? 'text-[var(--color-success)]'
                    : item.variant === 'error'
                      ? 'text-[var(--color-danger)]'
                      : 'text-[var(--color-primary)]',
                )}
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium leading-snug">{item.title}</p>
                {item.description ? (
                  <p className="mt-0.5 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                    {item.description}
                  </p>
                ) : null}
              </div>
              <button
                type="button"
                aria-label="关闭提示"
                onClick={() => dismiss(item.id)}
                className="rounded-md p-0.5 text-[var(--color-muted-foreground)] transition hover:bg-[var(--color-muted)]"
              >
                <X className="size-3.5" />
              </button>
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
