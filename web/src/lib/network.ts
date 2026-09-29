/**
 * 内网/外网地址自动切换。两路判定，命中任一即为内网：
 *   1. 浏览器端：当前 hostname 与后台配置的内网网段比对（直连 NAS 局域网地址时零延迟生效）；
 *   2. 服务端：按访客真实 IP 比对内网网段与「家庭公网出口」（经反代访问同一个域名时靠它区分在家 / 在外），
 *      结果随 /api/config 下发。
 *
 * 支持的规则写法：
 *   - CIDR：192.168.1.0/24
 *   - IP 前缀：10.0.
 *   - 精确 IP：192.168.1.10
 *   - 主机名精确/后缀：nas.lan、（.lan 或 *.lan 表示后缀匹配）
 *   - IPv6：以 : 开头的规则按前缀匹配（fd00:、fe80:）
 *   - 多条规则用逗号/分号/换行/空格分隔
 */

import type { ServerNetwork, Site } from './types'

/** 把用户填写的多行文本拆成规则数组。 */
export function normalizeRules(input: string[] | string | null | undefined): string[] {
  if (!input) return []
  const parts = Array.isArray(input) ? input : String(input).split(/[\s,;，；]+/)
  return parts.map((rule) => rule.trim()).filter(Boolean)
}

/** 去掉端口号与方括号，返回小写主机名。 */
export function normalizeHost(raw: string): string {
  let host = (raw ?? '').trim().toLowerCase()
  if (!host) return ''
  if (host.startsWith('[')) {
    const end = host.indexOf(']')
    if (end > 0) return host.slice(1, end)
  }
  if (/^\d+\.\d+\.\d+\.\d+:\d+$/.test(host)) {
    return host.split(':')[0]
  }
  if (host.split(':').length === 2) {
    host = host.split(':')[0]
  }
  return host
}

function ipv4ToInt(ip: string): number | null {
  const parts = ip.split('.')
  if (parts.length !== 4) return null
  let value = 0
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return null
    const n = Number(part)
    if (n > 255) return null
    value = value * 256 + n
  }
  return value
}

function matchesCIDR(host: string, rule: string): boolean {
  const [network, bitsRaw] = rule.split('/')
  const bits = Number(bitsRaw)
  if (!Number.isInteger(bits) || bits < 0 || bits > 32) return false
  const hostInt = ipv4ToInt(host)
  const netInt = ipv4ToInt(network)
  if (hostInt === null || netInt === null) return false
  if (bits === 0) return true
  const mask = bits === 32 ? 0xffffffff : (0xffffffff << (32 - bits)) >>> 0
  return (hostInt & mask) === (netInt & mask)
}

/** 单条规则匹配。 */
export function matchesRule(host: string, ruleRaw: string): boolean {
  const hostname = normalizeHost(host)
  const rule = (ruleRaw ?? '').trim().toLowerCase()
  if (!hostname || !rule) return false

  if (rule.includes('/')) {
    return matchesCIDR(hostname, rule)
  }
  if (rule.startsWith('*.')) {
    const suffix = rule.slice(1)
    return hostname === rule.slice(2) || hostname.endsWith(suffix)
  }
  if (rule.startsWith('.')) {
    return hostname.endsWith(rule)
  }
  if (rule.includes(':')) {
    return hostname.startsWith(rule)
  }
  if (/^\d+\.\d+\.\d+\.\d+$/.test(rule)) {
    return hostname === rule
  }
  if (/^\d+(\.\d+)*\.$/.test(rule)) {
    return hostname.startsWith(rule)
  }
  return hostname === rule || hostname.endsWith(`.${rule}`)
}

/**
 * 常用内网规则：新装实例的默认值，后台「填入常用网段」也用它（与 Go 端 config.DefaultLANRules 保持一致）。
 * 刻意不含 172.16.0.0/12：Docker 网桥网关（172.17.x.x）也在其中，端口映射走用户态代理时
 * 服务端看到的来源都是网关地址，会把外网直连误判成内网。
 */
export const DEFAULT_LAN_RULES = ['192.168.0.0/16', '10.0.0.0/8', '*.lan', '*.local', 'home.arpa']

/** 本机回环地址（localhost / 127.x / ::1）：用它访问说明就在 NAS 本机上，固定视为内网。 */
export function isLoopbackHost(host: string): boolean {
  const hostname = normalizeHost(host)
  return (
    hostname === 'localhost' || hostname.endsWith('.localhost') || /^127\./.test(hostname) || hostname === '::1'
  )
}

/** 当前访问来源是否处于内网（本机回环地址，或命中任意规则即为真）。 */
export function isPrivateHost(host: string, rules: string[] | string): boolean {
  if (isLoopbackHost(host)) return true
  const list = normalizeRules(rules)
  if (!list.length) return false
  return list.some((rule) => matchesRule(host, rule))
}

/** 站点当前应使用的地址：内网命中有内网地址则用内网，否则回落外网。 */
export function pickSiteUrl(
  site: Pick<Site, 'url' | 'lan_url'>,
  host: string,
  rules: string[] | string,
): string {
  return pickByMode(site, networkMode(host, rules))
}

/** 当前网络模式。 */
export function networkMode(host: string, rules: string[] | string): 'lan' | 'wan' {
  return isPrivateHost(host, rules) ? 'lan' : 'wan'
}

/** 判为内网的依据：host = 地址栏命中规则；lan_rule / home_egress = 服务端按访客 IP 判定。 */
export type NetworkReason = 'host' | 'lan_rule' | 'home_egress' | null

/**
 * 综合判定：地址栏命中规则（直连 NAS 局域网地址）优先，其次采用服务端按访客真实 IP 的判定
 * （经反代访问同一个域名时，地址栏无法区分在家 / 在外）。
 */
export function resolveNetwork(
  host: string,
  rules: string[] | string,
  server?: Pick<ServerNetwork, 'mode' | 'reason'> | null,
): { mode: 'lan' | 'wan'; reason: NetworkReason } {
  if (isPrivateHost(host, rules)) return { mode: 'lan', reason: 'host' }
  if (server?.mode === 'lan' && server.reason) return { mode: 'lan', reason: server.reason }
  return { mode: 'wan', reason: null }
}

/** 按判定结果选择站点地址：内网优先内网地址，否则外网；缺一个时回落另一个。 */
export function pickByMode(site: Pick<Site, 'url' | 'lan_url'>, mode: 'lan' | 'wan'): string {
  const lan = (site.lan_url ?? '').trim()
  const wan = (site.url ?? '').trim()
  return mode === 'lan' ? lan || wan : wan || lan
}
