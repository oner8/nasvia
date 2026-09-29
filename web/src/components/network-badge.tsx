import { Globe, Wifi } from 'lucide-react'

import { cn } from '../lib/utils'
import { Tooltip } from './ui/tooltip'

export interface NetworkBadgeProps {
  mode: 'lan' | 'wan'
  className?: string
}

/**
 * 网络模式标识：内网 = wifi 图标，外网 = 地球图标。
 * 鼠标悬停（或键盘聚焦）只显示当前结果，配置与排障说明统一放在后台设置。
 */
export function NetworkBadge({ className, mode }: NetworkBadgeProps) {
  const isLan = mode === 'lan'
  const label = isLan ? '内网' : '外网'

  return (
    <Tooltip side="bottom" content={label}>
      <span
        role="img"
        tabIndex={0}
        aria-label={label}
        className={cn(
          'inline-flex size-7 cursor-help items-center justify-center rounded-lg outline-none transition-colors',
          'text-[var(--color-foreground)] hover:bg-[var(--color-muted)]',
          'focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40',
          className,
        )}
      >
        {isLan ? <Wifi className="size-3.5" /> : <Globe className="size-3.5" />}
      </span>
    </Tooltip>
  )
}
