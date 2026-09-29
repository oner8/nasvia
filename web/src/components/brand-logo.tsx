import { cn } from '../lib/utils'

export interface BrandMarkProps {
  className?: string
}

/**
 * NASVIA 品牌标识（矢量徽章，跟随主题）。
 *
 * 设计说明：
 * - 圆角方形徽章 = NAS 设备/仪表盘容器，用前景色填充（浅色=近黑，深色=近白）；
 * - 徽章内是一笔连成的「N」：左竖 → 斜向 → 右竖，既是品牌首字母，也像一条航线；
 * - 斜线中点的圆形节点用徽章底色挖空 + 前景描边，像地图上的航点（route waypoint）；
 * - 纯内联 SVG：不请求外部资源、离线可用，深浅色主题下都清晰。
 *
 * ⚠️ 浏览器标签页图标 `web/public/icon.svg` 必须是同一造型（只是它读不到主题变量，
 *    改按系统配色偏好取等价值）；几何参数由 `web/src/lib/brand.test.ts` 守卫，改一边请同步另一边。
 */
export function BrandMark({ className }: BrandMarkProps) {
  return (
    <svg viewBox="0 0 40 40" aria-hidden="true" className={cn('size-5 shrink-0', className)}>
      <rect width="40" height="40" rx="11" fill="var(--color-foreground)" />
      <path
        d="M12.5 29.5V10.5L27.5 29.5V10.5"
        fill="none"
        stroke="var(--color-background)"
        strokeWidth="3.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle
        cx="20"
        cy="20"
        r="2.5"
        fill="var(--color-foreground)"
        stroke="var(--color-background)"
        strokeWidth="1.3"
      />
    </svg>
  )
}

export interface BrandLogoProps {
  /** 站点自定义名称；与品牌名不同时以次要文字跟随显示。 */
  title?: string
  className?: string
  markClassName?: string
}

/** 品牌锁定：徽章 + NASVIA 字标（可选站点自定义名）。 */
export function BrandLogo({ title, className, markClassName }: BrandLogoProps) {
  const name = (title ?? '').trim()
  const showName = name.length > 0 && name !== 'NASVIA' && name !== 'NASVIA 导航'

  return (
    <span className={cn('flex items-center gap-3', className)}>
      <BrandMark className={cn('size-6', markClassName)} />
      <span className="flex items-center gap-2 text-base font-semibold tracking-tight">
        NASVIA
        {showName ? (
          <span className="font-normal text-[var(--color-muted-foreground)]">· {name}</span>
        ) : null}
      </span>
    </span>
  )
}
