import { useMemo, useState } from 'react'
import { LayoutGrid, LogIn, Search, Settings, Sparkles } from 'lucide-react'

import { AdminDialog } from '../components/admin/admin-dialog'
import { BrandLogo } from '../components/brand-logo'
import { CategoryTabs, type CategoryTab } from '../components/category-tabs'
import { ClockGreeting } from '../components/clock-greeting'
import { CommandPalette } from '../components/command-palette'
import { LoginCard } from '../components/login-card'
import { NetworkBadge } from '../components/network-badge'
import { SiteGrid, SiteGridSkeleton } from '../components/site-grid'
import { ThemeToggle } from '../components/theme'
import { WebSearch } from '../components/web-search'
import { Button } from '../components/ui/button'
import { Tooltip } from '../components/ui/tooltip'
import { useCategories, useConfig, useSites } from '../hooks/use-api'
import { useNetworkMode } from '../hooks/use-network-mode'
import { useViewMode } from '../hooks/use-view-mode'
import {
  NAV_CONTENT_CLASS,
  NAV_GRID_CLASS,
  navRailClass,
  navRailColumnClass,
  normalizeNavPosition,
  normalizeNavStyle,
  normalizeNavVisible,
} from '../lib/nav'
import type { Site } from '../lib/types'
import { visibleCategories, visibleSites } from '../lib/visibility'
import { useUI } from '../stores/ui'

export function HomePage() {
  const configQuery = useConfig()
  const config = configQuery.data
  const authenticated = config?.authenticated ?? false
  const needLogin = config?.auth_mode === 'private' && !authenticated

  const sitesQuery = useSites(!needLogin)
  const categoriesQuery = useCategories(!needLogin)
  const network = useNetworkMode(config?.lan_cidrs, config?.network)
  const setPaletteOpen = useUI((state) => state.setPaletteOpen)
  const adminDialogOpen = useUI((state) => state.adminDialogOpen)
  const setAdminDialogOpen = useUI((state) => state.setAdminDialogOpen)
  const { view, setView } = useViewMode()

  // 导航位置（左/右）与标签样式（显示名称/仅图标）由后台统一设置决定；
  // 顶部导航已取消，历史值 top 在 lib/nav.ts 里会被归一成 left。
  const navPosition = normalizeNavPosition(config?.nav_position)
  const navStyle = normalizeNavStyle(config?.nav_style)
  // 分类导航是否显示由后台统一控制（设置 → 布局）
  const navVisible = normalizeNavVisible(config?.nav_visible)

  const [activeTab, setActiveTab] = useState<CategoryTab['id']>('all')

  const rawSites = sitesQuery.data?.items ?? []
  const rawCategories = categoriesQuery.data?.items ?? []

  // 服务端已按身份过滤，这里再兜底过滤一次，确保未登录时私密内容绝不出现。
  const sites = useMemo(
    () => visibleSites(rawSites, rawCategories, authenticated),
    [rawSites, rawCategories, authenticated],
  )
  const categories = useMemo(
    () => visibleCategories(rawSites, rawCategories, authenticated),
    [rawSites, rawCategories, authenticated],
  )

  const pinned = useMemo(() => sites.filter((site) => site.pinned), [sites])

  const tabs: CategoryTab[] = useMemo(() => {
    const list: CategoryTab[] = [{ id: 'all', name: '全部', count: sites.length }]
    if (pinned.length > 0) list.push({ id: 'pinned', name: '常用', count: pinned.length })
    for (const category of categories) {
      list.push({ id: category.id, name: category.name, count: category.site_count, icon: category.icon })
    }
    return list
  }, [sites.length, pinned.length, categories])

  const shown = useMemo(() => {
    if (activeTab === 'all') return sites
    if (activeTab === 'pinned') return pinned
    return sites.filter((site) => site.category_id === activeTab)
  }, [activeTab, sites, pinned])
  /** 「全部」视图按分类分组展示；其余标签为扁平列表。 */
  const grouped = useMemo(() => {
    if (activeTab !== 'all') return null
    const buckets = new Map<number | 'none', Site[]>()
    for (const site of shown) {
      const key = site.category_id ?? 'none'
      const list = buckets.get(key)
      if (list) list.push(site)
      else buckets.set(key, [site])
    }
    const sections: Array<[string, Site[]]> = []
    for (const category of categories) {
      const items = buckets.get(category.id)
      if (items && items.length > 0) sections.push([category.name, items])
    }
    const uncategorized = buckets.get('none')
    if (uncategorized && uncategorized.length > 0) sections.push(['未分类', uncategorized])
    return sections
  }, [activeTab, shown, categories])

  /** 三种视图共用同一份地址解析（内网命中则用内网地址）。 */
  const resolveHref = (site: Site) => network.pick(site)

  const showRail = tabs.length > 1 && navVisible === 'show'

  if (needLogin) {
    /**
     * 私密模式登录页：三行网格（1fr / auto / 1fr）。首尾两行等高，登录卡因此精确落在视口垂直中线上
     * ——此前头部（品牌 + 时钟）占着文档流，卡片只在剩余空间里居中，实测偏低约 40px。
     * 视口过矮时两个 1fr 回落到各自内容高度，页面正常滚动且元素不重叠。
     */
    return (
      <div className="mx-auto grid min-h-dvh w-full max-w-5xl grid-rows-[1fr_auto_1fr] px-4 py-8 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-4">
          <BrandLogo title={config?.site_title} />
          <div className="flex items-center justify-between">
            <ClockGreeting />
          </div>
        </div>

        <LoginCard
          title="需要登录"
          description="该实例已设为「私密」模式，登录后即可查看全部服务。"
          passwordMissing={config?.password_configured === false}
        />

        {/* 对称配重行（与头部等高）：留着它登录卡才会精确居中，不放内容。 */}
        <div aria-hidden />

        {/* 私密模式下 /admin 深链要能打开后台弹窗（此前该分支未渲染它，点了没反应）。 */}
        <AdminDialog open={adminDialogOpen} onOpenChange={setAdminDialogOpen} />
      </div>
    )
  }

  return (
    <div className={NAV_GRID_CLASS}>
      {/*
        中间列始终是 60rem 主内容；分类导航使用左右留白，不参与主内容宽度计算。
        小于 xl（1280px）时隐藏侧栏，避免覆盖主内容。
      */}
      {showRail ? (
        <div className={navRailColumnClass(navPosition)}>
          <CategoryTabs
            tabs={tabs}
            active={activeTab}
            onChange={setActiveTab}
            style={navStyle}
            side={navPosition === 'right' ? 'right' : 'left'}
            className={navRailClass(navStyle)}
          />
        </div>
      ) : null}

      <div className={NAV_CONTENT_CLASS}>
        <header className="mb-8 flex items-center gap-2.5">
          <BrandLogo title={config?.site_title} />
        </header>

        <div className="flex-1">
          {/* 工具栏：左侧问候 + 时钟，右侧操作按钮组 */}
          <div className="mb-6 flex items-center justify-between gap-3">
            <ClockGreeting />
            <div className="flex items-center gap-1 rounded-xl border border-[var(--color-border)] bg-[var(--color-muted)]/40 p-1">
              <Tooltip side="bottom" content="搜索服务（⌘K）">
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="搜索服务"
                  onClick={() => setPaletteOpen(true)}
                  className="size-7"
                >
                  <Search className="size-3.5" />
                </Button>
              </Tooltip>
              <div className="mx-0.5 h-4 w-px bg-[var(--color-border)]" aria-hidden />
              <ThemeToggle className="size-7 rounded-lg" view={view} onViewChange={setView} />
              <div className="mx-0.5 h-4 w-px bg-[var(--color-border)]" aria-hidden />
              <NetworkBadge
                mode={network.mode}
              />
              {/* 未登录：登录图标；已登录：齿轮（后台管理） */}
              <Tooltip side="bottom" content={authenticated ? '后台管理' : '登录'}>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={authenticated ? '后台管理' : '登录'}
                  className="size-7"
                  onClick={() => setAdminDialogOpen(true)}
                >
                  {authenticated ? <Settings className="size-3.5" /> : <LogIn className="size-3.5" />}
                </Button>
              </Tooltip>
            </div>
          </div>

          {/* 联网搜索：常驻搜索框（原来顶部导航栏那条位置），回车在新标签页打开所选引擎的结果页 */}
          <WebSearch className="mb-8" />

          {sitesQuery.iconPollingTimedOut ? (
            <div className="mb-4 flex items-center justify-between gap-3 rounded-xl bg-[var(--color-muted)]/50 px-3 py-2 text-xs">
              <span>后台仍在处理图标，自动检查已暂停。</span>
              <Button size="sm" variant="outline" onClick={sitesQuery.retryIconPolling}>
                刷新图标状态
              </Button>
            </div>
          ) : null}

          {sitesQuery.isLoading || categoriesQuery.isLoading ? (
            <SiteGridSkeleton view={view} />
          ) : sitesQuery.isError ? (
            <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-[var(--color-border)] py-16">
              <p className="text-sm text-[var(--color-muted-foreground)]">
                无法加载服务列表，请确认 NASVIA 服务正常运行。
              </p>
              <Button variant="outline" onClick={() => void sitesQuery.refetch()}>
                重试
              </Button>
            </div>
          ) : shown.length === 0 ? (
            <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-[var(--color-border)] py-16">
              <p className="flex items-center gap-2 text-sm text-[var(--color-muted-foreground)]">
                <Sparkles className="size-4" />
                {sites.length === 0 ? '还没有任何服务' : '该分类下暂无可见服务'}
              </p>
              <div className="flex flex-wrap justify-center gap-2">
                <Button variant="outline" className="gap-2" onClick={() => setAdminDialogOpen(true)}>
                  <LayoutGrid className="size-4" />
                  前往后台添加站点
                </Button>
                {authenticated ? null : (
                  <Button variant="ghost" className="gap-2" onClick={() => setAdminDialogOpen(true)}>
                    <LogIn className="size-4" />
                    登录
                  </Button>
                )}
              </div>
            </div>
          ) : grouped ? (
            <div className="flex flex-col gap-8">
              {grouped.map(([name, items]) => (
                <section key={name} className="flex flex-col gap-3">
                  <div className="flex items-center gap-2">
                    <h2 className="text-xs font-semibold tracking-tight">{name}</h2>
                    <span className="text-xs tabular-nums text-[var(--color-muted-foreground)]">
                      {items.length}
                    </span>
                  </div>
                  <SiteGrid sites={items} resolveHref={resolveHref} view={view} />
                </section>
              ))}
            </div>
          ) : (
            <SiteGrid sites={shown} resolveHref={resolveHref} view={view} />
          )}
        </div>

        <footer className="mt-auto flex flex-col items-center gap-3 pb-6 pt-8">
          <span className="text-xs text-[var(--color-muted-foreground)]/70">
            © {new Date().getFullYear()} NASVIA
            {config ? ` · v${config.version}` : ''}
          </span>
        </footer>

        <CommandPalette sites={sites} resolveUrl={(site) => network.pick(site)} />
        <AdminDialog open={adminDialogOpen} onOpenChange={setAdminDialogOpen} />
      </div>
    </div>
  )
}
