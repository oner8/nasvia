import { useMemo } from 'react'

import { normalizeRules, pickByMode, resolveNetwork, type NetworkReason } from '../lib/network'
import type { ServerNetwork, Site } from '../lib/types'

export interface NetworkContext {
  host: string
  mode: 'lan' | 'wan'
  /** 判为内网的依据（外网时为 null）。 */
  reason: NetworkReason
  /** 服务端看到的访客 IP（经反代时为真实 IP），用于悬停提示里排障。 */
  clientIP: string
  rules: string[]
  /** 依据当前网络选择站点地址。 */
  pick: (site: Pick<Site, 'url' | 'lan_url'>) => string
  isPrivate: boolean
}

/** 综合地址栏规则与服务端按访客 IP 的判定，得出当前网络模式。 */
export function useNetworkMode(
  rules: string[] | string | undefined,
  server?: ServerNetwork | null,
): NetworkContext {
  const list = useMemo(() => normalizeRules(rules), [rules])
  const host = typeof window !== 'undefined' ? window.location.hostname : ''
  const { mode, reason } = resolveNetwork(host, list, server)

  return {
    host,
    rules: list,
    mode,
    reason,
    clientIP: server?.client_ip ?? '',
    isPrivate: mode === 'lan',
    pick: (site) => pickByMode(site, mode),
  }
}
