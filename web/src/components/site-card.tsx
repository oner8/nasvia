import { useState } from 'react'

import type { Site } from '../lib/types'
import { cn, firstLetter, shortHost } from '../lib/utils'

export interface SiteCardProps {
  site: Site
  href: string
  onOpen?: (site: Site, href: string) => void
}

/**
 * 站点条目（磁贴视图）：图标方块 + 名称 + 一句话描述。
 *
 * 图标要不要再套一层灰底方块，由**图标来源**决定（服务端给的 icon_source）：
 * - favicon 多半是带透明通道的小图 → 套一层灰底衬托，网格才整齐；
 * - HD-Icons 与内置占位图自带底色 → 直接铺满方块，否则就是「圆角里再套圆角」。
 */
export function SiteCard({ site, href, onOpen }: SiteCardProps) {
  const [iconFailed, setIconFailed] = useState(false)
  const framed = !iconFailed && site.icon_source === 'favicon'

  return (
    <a
      href={href || '#'}
      target="_blank"
      rel="noreferrer noopener"
      title={href}
      onClick={(event) => {
        if (!href) {
          event.preventDefault()
          return
        }
        onOpen?.(site, href)
      }}
      className={cn(
        'group flex items-center gap-2.5 rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] p-3',
        'shadow-soft outline-none transition-all sm:gap-3.5 sm:p-4',
        'hover:border-[var(--color-foreground)]/15 hover:bg-[var(--color-accent)]/50 hover:shadow-md',
        'focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40',
      )}
    >
      <span
        className={cn(
          'flex size-8 shrink-0 items-center justify-center overflow-hidden sm:size-10',
          framed && 'rounded-lg bg-[var(--color-muted)] sm:rounded-xl',
        )}
      >
        {iconFailed ? (
          <span className="flex size-full items-center justify-center rounded-lg bg-[var(--color-muted)] text-xs font-semibold text-[var(--color-muted-foreground)] sm:rounded-xl sm:text-sm">
            {firstLetter(site.name)}
          </span>
        ) : (
          <img
            src={site.icon}
            alt=""
            loading="lazy"
            decoding="async"
            className={cn('object-contain', framed ? 'size-5 sm:size-6' : 'size-full rounded-lg sm:rounded-xl')}
            onError={() => setIconFailed(true)}
          />
        )}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">{site.name}</span>
        <span className="mt-0.5 block truncate text-xs text-[var(--color-muted-foreground)]">
          {site.description || shortHost(href)}
        </span>
      </span>
    </a>
  )
}
