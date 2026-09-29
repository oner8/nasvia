import type { Site } from './types'

export type SiteSortKey = 'name' | 'category' | 'visibility' | 'address'

export interface SiteSort {
  key: SiteSortKey
  dir: 'asc' | 'desc'
}

/** 表头标签，用于提示文案。 */
export const SORT_LABELS: Record<SiteSortKey, string> = {
  name: '站点',
  category: '分类',
  visibility: '可见性',
  address: '地址',
}

/**
 * 表头三态切换：无 → 升序 → 降序 → 无（回到手动顺序）。
 * 点另一列时从升序重新开始。
 */
export function nextSort(current: SiteSort | null, key: SiteSortKey): SiteSort | null {
  if (!current || current.key !== key) return { key, dir: 'asc' }
  return current.dir === 'asc' ? { key, dir: 'desc' } : null
}

/** 中文按拼音、数字按自然序比较。 */
const collator = new Intl.Collator('zh-Hans-CN', { numeric: true, sensitivity: 'base' })

/** 取某一列的排序文本；地址列外网为空时回落到内网地址。 */
function sortValue(site: Site, key: SiteSortKey, categoryNameOf: (id: number | null) => string): string {
  if (key === 'name') return site.name
  if (key === 'category') return categoryNameOf(site.category_id)
  if (key === 'visibility') return site.effective_visibility === 'public' ? '公开' : '私密'
  return site.url || site.lan_url || ''
}

/**
 * 按表头条件排序；sort 为 null 时原样返回（手动顺序）。
 * 稳定排序：同值保持传入顺序；不修改入参数组。
 */
export function sortSites(
  sites: Site[],
  sort: SiteSort | null,
  categoryNameOf: (id: number | null) => string,
): Site[] {
  if (!sort) return sites
  const factor = sort.dir === 'asc' ? 1 : -1
  return [...sites].sort((a, b) => {
    const diff = collator.compare(sortValue(a, sort.key, categoryNameOf), sortValue(b, sort.key, categoryNameOf))
    return diff === 0 ? 0 : diff * factor
  })
}

/**
 * 拖动排序：把 dragId 插到 targetId 之前（after=false）或之后（after=true），返回新顺序。
 * 拖到自己身上、或任一 id 不在列表里时原样返回。
 */
export function moveInList(ids: number[], dragId: number, targetId: number, after: boolean): number[] {
  if (dragId === targetId) return ids
  if (!ids.includes(dragId) || !ids.includes(targetId)) return ids
  const rest = ids.filter((id) => id !== dragId)
  const insertAt = rest.indexOf(targetId) + (after ? 1 : 0)
  return [...rest.slice(0, insertAt), dragId, ...rest.slice(insertAt)]
}

/**
 * 后台列表的关键词筛选：保持传入顺序（不按相关度或置顶重排），
 * 多个关键词之间是「与」关系——拖动排序需要「看到的顺序就是真实顺序」。
 */
export function filterSitesByKeyword(sites: Site[], keyword: string): Site[] {
  const tokens = (keyword ?? '')
    .toLowerCase()
    .split(/\s+/)
    .map((token) => token.trim())
    .filter(Boolean)
  if (!tokens.length) return sites
  return sites.filter((site) =>
    tokens.every((token) =>
      [site.name, site.description, site.tags, site.url, site.lan_url].some((field) =>
        (field ?? '').toLowerCase().includes(token),
      ),
    ),
  )
}
