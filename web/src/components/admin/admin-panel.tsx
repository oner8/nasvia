import { useState } from 'react'
import { motion } from 'motion/react'
import { Layers, ListChecks, LogOut, Settings2 } from 'lucide-react'

import { useConfig, useLogout } from '../../hooks/use-api'
import { toast } from '../../stores/ui'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Badge } from '../ui/primitives'
import { Tooltip } from '../ui/tooltip'
import { CategoryPanel } from './category-panel'
import { SettingsPanel } from './settings-panel'
import { SitePanel } from './site-panel'

const TABS = [
  { id: 'sites', label: '站点', Icon: ListChecks },
  { id: 'categories', label: '分类', Icon: Layers },
  { id: 'settings', label: '设置', Icon: Settings2 },
] as const

type TabId = (typeof TABS)[number]['id']

/** 后台管理面板：站点 / 分类 / 设置三个标签页，可嵌入整页或弹窗。 */
export function AdminPanel() {
  const configQuery = useConfig()
  const config = configQuery.data
  const logout = useLogout()
  const [tab, setTab] = useState<TabId>('sites')

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <div
          role="tablist"
          aria-label="后台导航"
          className="inline-flex w-fit items-center gap-1 rounded-2xl border border-[var(--color-border)] bg-[var(--color-muted)]/40 p-1"
        >
          {TABS.map(({ id, label, Icon }) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              onClick={() => setTab(id)}
              className={cn(
                'relative z-10 flex cursor-pointer items-center gap-2 rounded-lg px-3 py-1.5 text-xs font-medium outline-none transition-colors',
                tab === id
                  ? 'text-[var(--color-background)]'
                  : 'text-[var(--color-muted-foreground)] hover:text-[var(--color-foreground)]',
              )}
            >
              {tab === id ? (
                <motion.span
                  layoutId="admin-tab-indicator"
                  className="absolute inset-0 -z-10 rounded-lg bg-[var(--color-foreground)]"
                  transition={{ type: 'spring', stiffness: 500, damping: 40 }}
                />
              ) : null}
              <Icon className="relative size-3.5" />
              <span className="relative">{label}</span>
            </button>
          ))}
        </div>

        <div className="ml-auto flex items-center gap-2">
          {config ? (
            <Badge variant={config.auth_mode === 'private' ? 'danger' : 'success'}>
              {config.auth_mode === 'private' ? '私密' : '公开'}
            </Badge>
          ) : null}
          <Tooltip side="bottom" content="退出登录">
            <Button
              variant="ghost"
              size="icon"
              aria-label="退出登录"
              className="size-7"
              disabled={logout.isPending}
              onClick={async () => {
                await logout.mutateAsync()
                toast.info('已退出登录')
              }}
            >
              <LogOut className="size-3.5" />
            </Button>
          </Tooltip>
        </div>
      </div>

      {tab === 'sites' ? <SitePanel /> : null}
      {tab === 'categories' ? <CategoryPanel /> : null}
      {tab === 'settings' ? <SettingsPanel /> : null}
    </div>
  )
}
