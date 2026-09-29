import { Tooltip as BaseTooltip } from '@base-ui/react/tooltip'
import type { ReactElement, ReactNode } from 'react'

import { cn } from '../../lib/utils'

export interface TooltipProps {
  /** 悬浮提示内容。 */
  content: ReactNode
  /** 触发元素（会被 clone 并挂上悬停/聚焦事件）。 */
  children: ReactElement
  side?: 'top' | 'right' | 'bottom' | 'left'
  sideOffset?: number
  /** 悬停多久后显示（毫秒）。 */
  delay?: number
  className?: string
}

/**
 * 基于 @base-ui/react 的悬浮提示：悬停或键盘聚焦即显示，样式与主题一致。
 * 相比原生 title 属性，提示内容可排版、延迟可控、深色模式下同样清晰。
 */
export function Tooltip({
  content,
  children,
  side = 'bottom',
  sideOffset = 8,
  delay = 120,
  className,
}: TooltipProps) {
  return (
    <BaseTooltip.Root disableHoverablePopup>
      <BaseTooltip.Trigger render={children} delay={delay} />
      <BaseTooltip.Portal>
        <BaseTooltip.Positioner side={side} sideOffset={sideOffset} align="center" className="z-[70] outline-none">
          <BaseTooltip.Popup
            role="tooltip"
            className={cn(
              'max-w-[18rem] rounded-lg border border-[var(--color-border)] bg-[var(--color-card)]',
              'px-2.5 py-1.5 text-xs leading-relaxed text-[var(--color-card-foreground)] shadow-soft',
              'transition-opacity duration-150 data-[starting-style]:opacity-0 data-[ending-style]:opacity-0',
              className,
            )}
          >
            {content}
          </BaseTooltip.Popup>
        </BaseTooltip.Positioner>
      </BaseTooltip.Portal>
    </BaseTooltip.Root>
  )
}
