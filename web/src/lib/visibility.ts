/**
 * 可见性规则（与后端 internal/visibility 一致；后端为权威，前端用于列表/搜索/计数的即时过滤）。
 *
 *   - 已登录：可见全部。
 *   - **分类是可见性的上限**：分类为 private 时，其下站点对访客一律不可见
 *     （即使站点显式 public 也不可见；后端也不允许这么设置）。
 *   - 分类 public 时：站点 public 可见、private 不可见、inherit 跟随分类。
 *   - inherit 且无分类 → 视为公开。
 *   - 分类自身 public → 可见；private → 不可见（不因站点的显式 public 而提升）。
 */

import type { CategoryView, Site } from './types'

export function categoryVisibilityMap(
  categories: CategoryView[],
): Map<number, CategoryView['visibility']> {
  const map = new Map<number, CategoryView['visibility']>()
  for (const category of categories) {
    map.set(category.id, category.visibility)
  }
  return map
}

/** 站点的实际可见性（把 inherit / public 都按所属分类的上限解析）。 */
export function effectiveVisibility(
  site: Pick<Site, 'visibility' | 'category_id'>,
  categoryVisibility: Map<number, CategoryView['visibility']> | Record<number, 'public' | 'private'>,
): 'public' | 'private' {
  if ((site.visibility ?? 'inherit') === 'private') return 'private'
  const categoryId = site.category_id
  if (categoryId === null || categoryId === undefined) return 'public'
  const lookup =
    categoryVisibility instanceof Map
      ? categoryVisibility.get(categoryId)
      : categoryVisibility[categoryId]
  if (!lookup) return 'public'
  return lookup === 'private' ? 'private' : 'public'
}

/** 单个站点对当前身份是否可见。 */
export function isSiteVisible(
  site: Pick<Site, 'visibility' | 'category_id'>,
  categoryVisibility: Map<number, CategoryView['visibility']> | Record<number, 'public' | 'private'>,
  authenticated: boolean,
): boolean {
  if (authenticated) return true
  return effectiveVisibility(site, categoryVisibility) === 'public'
}

/** 过滤出当前身份可见的站点。 */
export function visibleSites(sites: Site[], categories: CategoryView[], authenticated: boolean): Site[] {
  if (authenticated) return sites.slice()
  const map = categoryVisibilityMap(categories)
  return sites.filter((site) => isSiteVisible(site, map, false))
}

/** 过滤出当前身份有可见站点的分类，并重算计数（空分类不进入首页导航）。 */
export function visibleCategories(
  sites: Site[],
  categories: CategoryView[],
  authenticated: boolean,
): CategoryView[] {
  const visible = visibleSites(sites, categories, authenticated)
  const counts = new Map<number, number>()
  for (const site of visible) {
    if (site.category_id === null || site.category_id === undefined) continue
    counts.set(site.category_id, (counts.get(site.category_id) ?? 0) + 1)
  }
  return categories
    .filter((category) => authenticated || category.visibility === 'public')
    .map((category) => ({ ...category, site_count: counts.get(category.id) ?? 0 }))
    .filter((category) => category.site_count > 0)
}
