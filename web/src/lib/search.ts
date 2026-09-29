/**
 * ⌘K 全局搜索的匹配与排序：按名称、描述、地址（含内网地址）、标签做大小写不敏感的模糊匹配。
 * 排序优先级：名称完全匹配 > 名称前缀 > 名称包含 > 标签 > 描述 > 地址。
 */

import type { CategoryView, Site } from './types'
import { visibleSites } from './visibility.ts'

export interface ScoredSite {
  site: Site
  score: number
}

function tokenScore(site: Site, token: string): number {
  const name = (site.name ?? '').toLowerCase()
  if (!token) return 0
  if (name === token) return 130
  if (name.startsWith(token)) return 110
  if (name.includes(token)) return 90
  if ((site.tags ?? '').toLowerCase().includes(token)) return 70
  if ((site.description ?? '').toLowerCase().includes(token)) return 55
  if ((site.url ?? '').toLowerCase().includes(token)) return 40
  if ((site.lan_url ?? '').toLowerCase().includes(token)) return 40
  return 0
}

/** 依据查询串给站点打分；0 表示不匹配。多个关键词之间是「与」关系。 */
export function scoreSite(site: Site, query: string): number {
  const tokens = (query ?? '')
    .toLowerCase()
    .split(/\s+/)
    .map((token) => token.trim())
    .filter(Boolean)
  if (!tokens.length) return 1
  let total = 0
  for (const token of tokens) {
    const score = tokenScore(site, token)
    if (score === 0) return 0
    total += score
  }
  return total
}

/** 在给定站点集合内搜索并按相关度排序。 */
export function searchSites(sites: Site[], query: string): Site[] {
  const trimmed = (query ?? '').trim()
  if (!trimmed) {
    return [...sites].sort(compareBySort)
  }
  const scored: ScoredSite[] = []
  for (const site of sites) {
    const score = scoreSite(site, trimmed)
    if (score > 0) {
      scored.push({ site, score: score + (site.pinned ? 8 : 0) })
    }
  }
  scored.sort((a, b) => b.score - a.score || compareBySort(a.site, b.site))
  return scored.map((item) => item.site)
}

function compareBySort(a: Site, b: Site): number {
  if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
  return a.sort - b.sort || a.id - b.id
}

/** 先按身份过滤可见性，再搜索：未登录时私密站点永远不会出现在结果里。 */
export function searchVisibleSites(
  sites: Site[],
  categories: CategoryView[],
  authenticated: boolean,
  query: string,
): Site[] {
  return searchSites(visibleSites(sites, categories, authenticated), query)
}
