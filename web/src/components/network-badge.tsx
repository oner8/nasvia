import { Globe, Wifi } from 'lucide-react'

import { isLoopbackHost, type NetworkReason } from '../lib/network'
import { cn } from '../lib/utils'
import { Tooltip } from './ui/tooltip'

export interface NetworkBadgeProps {
  mode: 'lan' | 'wan'
  reason: NetworkReason
  host: string
  /** 服务端看到的访客 IP（经反代时为真实 IP）。 */
  clientIP: string
  className?: string
}

function describe({ mode, reason, host, clientIP }: Omit<NetworkBadgeProps, 'className'>): string {
  const ip = clientIP ? `访客 IP ${clientIP}` : '访客 IP'
  if (mode === 'lan') {
    if (reason === 'home_egress') return `${ip} 来自家庭公网出口，站点将优先使用内网地址`
    if (reason === 'lan_rule') return `${ip} 命中内网网段，站点将优先使用内网地址`
    if (isLoopbackHost(host)) return `当前地址 ${host} 是本机地址，站点将优先使用内网地址`
    return `当前地址 ${host} 命中内网网段，站点将优先使用内网地址`
  }
  return `当前地址 ${host}、${ip} 均未命中内网网段或家庭公网出口，站点使用外网地址（可在后台「设置」中配置）`
}

/**
 * 网络模式标识：内网 = wifi 图标，外网 = 地球图标。
 * 鼠标悬停（或键盘聚焦）显示判定依据的悬浮提示（含服务端看到的访客 IP，便于排查反代配置）。
 */
export function NetworkBadge({ className, ...props }: NetworkBadgeProps) {
  const isLan = props.mode === 'lan'
  const label = isLan ? '内网访问' : '外网访问'
  const detail = describe(props)

  return (
    <Tooltip
      side="bottom"
      content={
        <span className="flex flex-col gap-0.5">
          <span className="font-medium">{label}</span>
          <span className="text-[var(--color-muted-foreground)]">{detail}</span>
        </span>
      }
    >
      <span
        role="img"
        tabIndex={0}
        aria-label={`${label}：${detail}`}
        className={cn(
          'inline-flex size-7 cursor-help items-center justify-center rounded-lg outline-none transition-colors',
          'text-[var(--color-foreground)] hover:bg-[var(--color-muted)]',
          'focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40',
          className,
        )}
      >
        {isLan ? <Wifi className="size-3.5" /> : <Globe className="size-3.5" />}
      </span>
    </Tooltip>
  )
}
