import { useState } from 'react'

import type { Site } from '../lib/types'
import { cn, firstLetter, shortHost } from '../lib/utils'
import { Tooltip } from './ui/tooltip'

export interface SiteAppProps {
  site: Site
  href: string
  onOpen?: (site: Site, href: string) => void
}

/**
 * 应用视图条目：图标方块 + 名称居中（类 iOS 启动器网格）。
 *
 * 间距按 iOS 主屏比例：图标占格子宽度的 70%、圆角约 22%（squircle 观感）、名称紧贴图标下方；
 * 列数（手机 4 / 平板 5、6 / 桌面 8）与格子间距见 GRID_CLASS.app。
 * 描述不占版面，改在悬停提示里给出；图标是否套灰底与卡片一致（由 icon_source 决定）：
 * favicon 需要衬托底色，HD-Icons / nasicon / 内置占位图自带底色 → 直接铺满。
 */
export function SiteApp({ site, href, onOpen }: SiteAppProps) {
  const [iconFailed, setIconFailed] = useState(false)
  const description = site.description || shortHost(href)
  const framed = !iconFailed && site.icon_source === 'favicon'

  return (
    <Tooltip side="top" content={description ? `${site.name} · ${description}` : site.name}>
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
          'group flex min-w-0 cursor-pointer flex-col items-center gap-1.5 rounded-xl p-1.5',
          'outline-none transition-colors',
          'hover:bg-[var(--color-accent)]/50 focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40',
        )}
      >
        <span
          className={cn(
            'flex aspect-square w-[70%] shrink-0 items-center justify-center overflow-hidden rounded-[22%]',
            'transition-transform duration-200 group-hover:scale-105',
            framed && 'bg-[var(--color-muted)]',
          )}
        >
          {iconFailed ? (
            <span className="flex size-full items-center justify-center rounded-[22%] bg-[var(--color-muted)] text-sm font-semibold text-[var(--color-muted-foreground)]">
              {firstLetter(site.name)}
            </span>
          ) : (
            <img
              src={site.icon}
              alt=""
              loading="lazy"
              decoding="async"
              className={cn('object-contain', framed ? 'size-1/2' : 'size-full')}
              onError={() => setIconFailed(true)}
            />
          )}
        </span>
        <span className="w-full truncate text-center text-xs font-medium">{site.name}</span>
      </a>
    </Tooltip>
  )
}
