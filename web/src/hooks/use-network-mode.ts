import { useMemo } from 'react'

import { normalizeRules, pickByMode, resolveNetwork } from '../lib/network'
import type { ServerNetwork, Site } from '../lib/types'

export interface NetworkContext {
  mode: 'lan' | 'wan'
  /** 依据当前网络选择站点地址。 */
  pick: (site: Pick<Site, 'url' | 'lan_url'>) => string
}

/** 综合地址栏规则与服务端按访客 IP 的判定，得出当前网络模式。 */
export function useNetworkMode(
  rules: string[] | string | undefined,
  server?: ServerNetwork | null,
): NetworkContext {
  const list = useMemo(() => normalizeRules(rules), [rules])
  const host = typeof window !== 'undefined' ? window.location.hostname : ''
  const { mode } = resolveNetwork(host, list, server)

  return {
    mode,
    pick: (site) => pickByMode(site, mode),
  }
}
