import type { Site } from '../lib/types'
import { GRID_CLASS, type ViewMode } from '../lib/view-mode'
import { SiteApp } from './site-app'
import { SiteCard } from './site-card'

export interface SiteGridProps {
  sites: Site[]
  /** 依据当前网络解析出的站点地址（内外网自动切换）。 */
  resolveHref: (site: Site) => string
  view: ViewMode
  onOpen?: (site: Site, href: string) => void
}

/**
 * 站点网格：按视图模式选择条目样式（应用图标 / 卡片 / 紧凑列表），列数类名来自 GRID_CLASS。
 * 空列表渲染 null，交由调用方决定空状态。
 */
export function SiteGrid({ sites, resolveHref, view, onOpen }: SiteGridProps) {
  if (sites.length === 0) return null
  return (
    <div className={GRID_CLASS[view]}>
      {sites.map((site) => {
        const props = { site, href: resolveHref(site), onOpen }
        return view === 'app' ? (
          <SiteApp key={site.id} {...props} />
        ) : (
          <SiteCard key={site.id} {...props} compact={view === 'list'} />
        )
      })}
    </div>
  )
}

/** 加载骨架：列数与当前视图的真实网格一致，切换视图时不跳动。 */
export function SiteGridSkeleton({ view, count = 6 }: { view: ViewMode; count?: number }) {
  return (
    <div className={GRID_CLASS[view]}>
      {Array.from({ length: count }).map((_, index) =>
        view === 'app' ? (
          <div key={index} className="flex animate-pulse-soft flex-col items-center gap-1.5 p-1.5">
            <span className="aspect-square w-[70%] rounded-[22%] bg-[var(--color-muted)]" />
            <span className="h-3 w-3/5 rounded-full bg-[var(--color-muted)]" />
          </div>
        ) : view === 'list' ? (
          <div
            key={index}
            className="h-11 animate-pulse-soft rounded-xl border border-[var(--color-border)] bg-[var(--color-muted)]/40"
          />
        ) : (
          <div
            key={index}
            className="h-16 animate-pulse-soft rounded-xl border border-[var(--color-border)] bg-[var(--color-muted)]/40"
          />
        ),
      )}
    </div>
  )
}
