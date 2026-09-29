import { Dialog as BaseDialog } from '@base-ui/react/dialog'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '../../lib/utils'

export interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title?: ReactNode
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
  className?: string
  /** 内容区（可滚动部分）的类名，例如 p-0 去掉默认内边距。 */
  bodyClassName?: string
}

/**
 * 基于 @base-ui/react 的对话框。未传 title/description 时只保留一个悬浮关闭按钮。
 * 只有中间的内容区滚动：标题栏、关闭按钮与底部按钮固定在弹窗内，内容再长也不会被滚走。
 */
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  className,
  bodyClassName,
}: DialogProps) {
  const hasHeader = Boolean(title || description)
  const closeButton = (
    <BaseDialog.Close
      aria-label="关闭"
      className="shrink-0 rounded-lg p-1.5 text-[var(--color-muted-foreground)] outline-none transition hover:bg-[var(--color-muted)]"
    >
      <X className="size-4" />
    </BaseDialog.Close>
  )

  return (
    <BaseDialog.Root open={open} onOpenChange={(next) => onOpenChange(next)}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop
          className={cn(
            'fixed inset-0 z-50 bg-black/45 backdrop-blur-[2px] transition-opacity',
            'data-[starting-style]:opacity-0 data-[ending-style]:opacity-0',
          )}
        />
        <BaseDialog.Popup
          className={cn(
            'fixed left-1/2 top-1/2 z-50 flex max-h-[90vh] w-[min(94vw,42rem)] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden',
            'rounded-2xl border border-[var(--color-border)] bg-[var(--color-card)] shadow-soft outline-none',
            'transition-[opacity,scale] duration-150 data-[starting-style]:scale-95 data-[starting-style]:opacity-0 data-[ending-style]:scale-95 data-[ending-style]:opacity-0',
            className,
          )}
        >
          {hasHeader ? (
            <div className="flex shrink-0 items-start justify-between gap-4 px-5 pb-4 pt-5">
              <div>
                {title ? (
                  <BaseDialog.Title className="text-base font-semibold tracking-tight">
                    {title}
                  </BaseDialog.Title>
                ) : null}
                {description ? (
                  <BaseDialog.Description className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                    {description}
                  </BaseDialog.Description>
                ) : null}
              </div>
              {closeButton}
            </div>
          ) : (
            <div className="absolute right-3 top-3 z-10">{closeButton}</div>
          )}
          <div className={cn('min-h-0 flex-1 overflow-y-auto px-5 pb-5', !hasHeader && 'pt-5', bodyClassName)}>
            {children}
          </div>
          {footer ? <div className="flex shrink-0 justify-end gap-2 px-5 pb-5">{footer}</div> : null}
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  )
}
