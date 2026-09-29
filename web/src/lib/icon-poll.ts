import type { IconStatus, SiteListResponse } from './types'

export const ICON_POLL_MAX_MS = 10 * 60_000

/** 图标轮询：前 30 秒 2s，2 分钟内 5s，之后 10s；失败时退避但不超过 10s。 */
export function iconPollDelay(elapsedMs: number, failures = 0): number {
  const base = elapsedMs < 30_000 ? 2_000 : elapsedMs < 120_000 ? 5_000 : 10_000
  return Math.min(base * 2 ** Math.min(Math.max(failures, 0), 3), 10_000)
}

/** 把轻量状态响应合并回现有完整列表缓存。 */
export function mergeIconStatuses(
  current: SiteListResponse | undefined,
  statuses: IconStatus[],
): SiteListResponse | undefined {
  if (!current || statuses.length === 0) return current
  const byID = new Map(statuses.map((status) => [status.id, status]))
  return {
    ...current,
    items: current.items.map((site) => {
      const status = byID.get(site.id)
      return status ? { ...site, ...status } : site
    }),
  }
}
