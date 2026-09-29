import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Eraser, KeyRound, Loader2, RefreshCw, Save, ShieldCheck } from 'lucide-react'

import { api, apiErrorMessage, type HDIconsTestResult, type SettingsPayload } from '../../lib/api'
import { useAdminSettings } from '../../hooks/use-api'
import { DEFAULT_LAN_RULES, normalizeRules } from '../../lib/network'
import type { AdminSettings } from '../../lib/types'
import { toast } from '../../stores/ui'
import { Button } from '../ui/button'
import { Dialog } from '../ui/dialog'
import { Input, Label, Textarea } from '../ui/input'
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/primitives'
import { Select, Switch } from '../ui/select'
import {
  normalizeNavPosition,
  normalizeNavStyle,
  normalizeNavVisible,
  type NavPosition,
  type NavStyle,
  type NavVisible,
} from '../../lib/nav'

const AUTH_MODES = [
  { value: 'public', label: '公开', description: '任何人可打开首页，只看到标记为公开的服务' },
  { value: 'private', label: '私密', description: '首页需要密码登录，登录后可见全部服务' },
]

/** 分类导航位置（后台统一设置，前台所有访客一致）。 */
const NAV_POSITION_OPTIONS = [
  { value: 'left', label: '左侧', description: '紧贴内容左侧、垂直居中（默认）' },
  { value: 'right', label: '右侧', description: '紧贴内容右侧、垂直居中' },
]

/** 分类导航是否显示（所有访客一致）。 */
const NAV_VISIBLE_OPTIONS = [
  { value: 'show', label: '显示', description: '前台显示分类导航（默认）' },
  { value: 'hide', label: '隐藏', description: '前台完全不显示分类导航，内容占满整行' },
]

/** 分类导航标签样式：显示分类名，或只显示图标。 */
const NAV_STYLE_OPTIONS = [
  { value: 'text', label: '显示名称', description: '标签显示分类名（默认）' },
  { value: 'icon', label: '仅图标', description: '只显示图标，悬停显示名称与数量' },
]

/** 系统设置：站点标题、前台模式、内网网段、图标来源（含 HD-Icons 与镜像前缀）、密码与数据清空。 */
export function SettingsPanel() {
  const queryClient = useQueryClient()
  const settingsQuery = useAdminSettings()

  const [title, setTitle] = useState('')
  const [authMode, setAuthMode] = useState<'public' | 'private'>('public')
  const [lanCIDRs, setLanCIDRs] = useState('')
  const [homeEgress, setHomeEgress] = useState('')
  const [sources, setSources] = useState({ hdicons: true, nasicon: true, site: true, duckduckgo: true, google: true })
  const [mirrors, setMirrors] = useState('')
  const [nasBase, setNasBase] = useState('')
  const [navPosition, setNavPosition] = useState<NavPosition>('left')
  const [navStyle, setNavStyle] = useState<NavStyle>('text')
  const [navVisible, setNavVisible] = useState<NavVisible>('show')
  const [hdTest, setHdTest] = useState<HDIconsTestResult | null>(null)
  const [password, setPassword] = useState('')
  const [confirmState, setConfirmState] = useState<{ title: string; description: string; action: () => void } | null>(
    null,
  )

  useEffect(() => {
    const data = settingsQuery.data
    if (!data) return
    setTitle(data.site_title)
    setAuthMode(data.auth_mode)
    setLanCIDRs(data.lan_cidrs_raw)
    setHomeEgress(data.home_egress_raw ?? '')
    setSources(data.favicon_sources)
    setMirrors(data.hdicons_mirrors_raw)
    setNasBase(data.nasicon_base_raw)
    setNavPosition(normalizeNavPosition(data.nav_position))
    setNavStyle(normalizeNavStyle(data.nav_style))
    setNavVisible(normalizeNavVisible(data.nav_visible))
  }, [settingsQuery.data])

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['admin', 'settings'] }),
      queryClient.invalidateQueries({ queryKey: ['config'] }),
      queryClient.invalidateQueries({ queryKey: ['admin', 'sites'] }),
      queryClient.invalidateQueries({ queryKey: ['admin', 'categories'] }),
      queryClient.invalidateQueries({ queryKey: ['sites'] }),
      queryClient.invalidateQueries({ queryKey: ['categories'] }),
    ])
  }

  const saveMutation = useMutation({
    mutationFn: (payload: SettingsPayload) => api.updateSettings(payload),
    onSuccess: async () => {
      await invalidate()
      toast.success('设置已保存')
      // DDNS 域名 / auto 在服务端异步解析，稍后再取一次，好显示解析结果
      window.setTimeout(() => void queryClient.invalidateQueries({ queryKey: ['admin', 'settings'] }), 4000)
    },
    onError: (error) => toast.error('保存失败', apiErrorMessage(error)),
  })

  const testMutation = useMutation({
    mutationFn: () => api.testHDIcons(),
    onSuccess: (result) => {
      setHdTest(result)
      const usable = result.mirrors.filter((row) => row.ok).length
      if (usable > 0) toast.success(`有 ${usable}/${result.mirrors.length} 个镜像前缀可用`)
      else toast.error('镜像前缀都不可用', '换一个前缀再试，或先关掉 HD-Icons 图标源')
    },
    onError: (error) => toast.error('测试失败', apiErrorMessage(error)),
  })

  const passwordMutation = useMutation({
    mutationFn: (value: string) => api.changePassword(value),
    onSuccess: async () => {
      setPassword('')
      await invalidate()
      toast.success('密码已更新')
    },
    onError: (error) => toast.error('修改密码失败', apiErrorMessage(error)),
  })

  const purgeMutation = useMutation({
    mutationFn: () => api.purgeAll(),
    onSuccess: async () => {
      await invalidate()
      toast.success('已清空全部站点、分类与图标缓存')
    },
    onError: (error) => toast.error('清空失败', apiErrorMessage(error)),
  })

  const settings = settingsQuery.data

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>基础设置</CardTitle>
          <CardDescription>站点标题会显示在首页左上角，可在浏览器标签与页面标题中看到。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <Label>站点标题</Label>
              <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="NASVIA" />
            </div>
            <div>
              <Label>前台访问模式</Label>
              <Select
                value={authMode}
                onChange={(value) => setAuthMode(value as 'public' | 'private')}
                options={AUTH_MODES}
              />
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                切换为「私密」前请确保已设置密码；后台管理在任何模式下都需要登录。
              </p>
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between gap-2">
              <Label>内网网段 / IP 前缀（每行一条，也支持逗号分隔）</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-xs"
                onClick={() => setLanCIDRs(mergeRules(lanCIDRs, DEFAULT_LAN_RULES))}
              >
                填入常用网段
              </Button>
            </div>
            <Textarea
              rows={4}
              value={lanCIDRs}
              onChange={(event) => setLanCIDRs(event.target.value)}
              placeholder={'192.168.1.0/24\n10.0.\nhome.arpa\n*.lan'}
            />
            <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
              支持 CIDR（192.168.1.0/24）、IP 前缀（10.0.）、精确 IP、主机名或 .lan 后缀。浏览器的访问地址、服务端看到的访客 IP
              任一命中即优先使用站点的内网地址，否则使用外网地址；用 localhost / 127.0.0.1 访问时固定视为内网。经反代用域名访问时<strong>不要</strong>把该域名写进来（否则在外面也会判成内网），改用下面的「家庭公网出口」。
            </p>
          </div>
          <div>
            <Label>家庭公网出口（经反代 / 内网穿透用域名访问时，据此区分在家 / 在外）</Label>
            <Textarea
              rows={2}
              value={homeEgress}
              onChange={(event) => setHomeEgress(event.target.value)}
              placeholder={'nas.example.com\nauto'}
            />
            <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
              每行一条：家里的 DDNS 域名（推荐，每 5 分钟重新解析）、<code>auto</code>（NAS 自动探测自己的公网出口 IP）、固定公网 IP 或 CIDR。
              反代看到的访客 IP 等于它即判为内网；反代需传递 X-Forwarded-For（Nginx Proxy Manager 默认已传）。
              浏览器若走代理访问该域名，需让它直连。
            </p>
            {settingsQuery.data?.home_egress_status ? (
              <HomeEgressStatus status={settingsQuery.data.home_egress_status} />
            ) : null}
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <Label>分类导航位置</Label>
              <Select
                value={navPosition}
                onChange={(value) => setNavPosition(value as NavPosition)}
                options={NAV_POSITION_OPTIONS}
              />
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                导航与内容同一行、紧贴内容并垂直居中（顶部导航已取消）；手机与平板（&lt;1024px）整块隐藏，内容占满整行。
              </p>
            </div>
            <div>
              <Label>分类导航标签样式</Label>
              <Select
                value={navStyle}
                onChange={(value) => setNavStyle(value as NavStyle)}
                options={NAV_STYLE_OPTIONS}
              />
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                选「仅图标」时，悬停标签会显示「名称 · 数量」；各分类的图标可在「分类」面板里单独指定，
                留空则按分类名自动匹配。
              </p>
            </div>
            <div>
              <Label>分类导航</Label>
              <Select
                value={navVisible}
                onChange={(value) => setNavVisible(value as NavVisible)}
                options={NAV_VISIBLE_OPTIONS}
              />
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                选「隐藏」时前台完全不显示分类导航（所有访客一致），内容占满整行。
              </p>
            </div>
          </div>

          <div className="flex flex-col gap-3 rounded-xl border border-[var(--color-border)] p-3">
            <div>
              <p className="text-sm font-medium">图标来源</p>
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                按顺序尝试：先按站点名/中文名匹配 HD-Icons 的圆角图标（风格统一），找不到再到 nasicon.top 按
                中文名/域名找（它的索引里带中文名，中式站点更容易配上），仍没有才抓站点自己的 favicon；全部失败时使用内置占位图。
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-6">
              <div className="flex items-center gap-2">
                <Switch
                  checked={sources.hdicons}
                  onChange={(next) => setSources({ ...sources, hdicons: next })}
                  ariaLabel="HD-Icons 圆角图标源"
                />
                <span className="text-xs">HD-Icons 圆角图标（按名称匹配，优先）</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={sources.nasicon}
                  onChange={(next) => setSources({ ...sources, nasicon: next })}
                  ariaLabel="nasicon.top 图标源"
                />
                <span className="text-xs">nasicon.top（中文名/域名匹配，HD-Icons 之后兜底）</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={sources.site}
                  onChange={(next) => setSources({ ...sources, site: next })}
                  ariaLabel="站点自身图标"
                />
                <span className="text-xs">站点自身 favicon.ico / HTML 图标</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={sources.duckduckgo}
                  onChange={(next) => setSources({ ...sources, duckduckgo: next })}
                  ariaLabel="DuckDuckGo 图标源"
                />
                <span className="text-xs">DuckDuckGo 回退</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={sources.google}
                  onChange={(next) => setSources({ ...sources, google: next })}
                  ariaLabel="Google S2 图标源"
                />
                <span className="text-xs">Google S2 回退</span>
              </div>
            </div>
            <p className="text-xs text-[var(--color-muted-foreground)]">
              nasicon 生效地址：{nasBase || 'https://nasicon.top'}（想换镜像/自建镜像时设环境变量 NASVIA_NASICON_BASE）；
              索引缓存 7 天，只按需下载命中的那一个图标。
            </p>

            <div>
              <Label>HD-Icons 镜像前缀（每行一个，按顺序尝试）</Label>
              <Textarea
                rows={3}
                value={mirrors}
                onChange={(event) => setMirrors(event.target.value)}
                placeholder={
                  'https://raw.githubusercontent.com/xushier/HD-Icons/main\nhttps://cdn.jsdelivr.net/gh/xushier/HD-Icons@main\nhttps://gh-proxy.com/https://raw.githubusercontent.com/xushier/HD-Icons/main'
                }
              />
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-2"
                  disabled={testMutation.isPending}
                  onClick={() => testMutation.mutate()}
                >
                  {testMutation.isPending ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <RefreshCw className="size-3.5" />
                  )}
                  测试连通性
                </Button>
                <span className="text-xs text-[var(--color-muted-foreground)]">
                  直连 raw.githubusercontent.com 不通时，换成上面示例里的镜像代理前缀。
                </span>
              </div>
              {hdTest ? (
                <div className="mt-2 flex flex-col gap-1.5 rounded-xl bg-[var(--color-muted)]/40 p-3">
                  {hdTest.mirrors.map((row) => (
                    <div key={row.mirror} className="flex items-start gap-2 text-xs">
                      <span className={row.ok ? 'text-[var(--color-success)]' : 'text-[var(--color-danger)]'}>
                        {row.ok ? '✓' : '✗'}
                      </span>
                      <span className="min-w-0 flex-1 break-all font-mono">{row.mirror}</span>
                      <span className="shrink-0 text-[var(--color-muted-foreground)]">
                        {row.ok ? `${row.ms}ms · ${row.count} 个图标` : row.error || '不可用'}
                      </span>
                    </div>
                  ))}
                  <p className="text-xs text-[var(--color-muted-foreground)]">
                    {hdTest.cache
                      ? `本地索引缓存：${hdTest.cache.count} 个图标 · 来自 ${hdTest.cache.mirror} · ${
                          hdTest.cache.fresh ? '有效' : '已过期'
                        }（${new Date(hdTest.cache.fetched_at).toLocaleString()}）`
                      : '本地还没有索引缓存（首次抓取图标时自动建立，之后离线也能匹配）。'}
                  </p>
                </div>
              ) : null}
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button
              className="gap-2"
              disabled={saveMutation.isPending}
              onClick={() =>
                saveMutation.mutate({
                  site_title: title,
                  auth_mode: authMode,
                  lan_cidrs: lanCIDRs,
                  home_egress: homeEgress,
                  favicon_sources: sources,
                  hdicons_mirrors: mirrors,
                  nav_position: navPosition,
                  nav_style: navStyle,
                  nav_visible: navVisible,
                })
              }
            >
              {saveMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存设置
            </Button>
            {settings ? (
              <Badge variant={settings.password_configured ? 'success' : 'danger'}>
                {settings.password_configured ? '已设置后台密码' : '未设置密码'}
              </Badge>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="size-4 text-[var(--color-primary)]" />
            修改后台密码
          </CardTitle>
          <CardDescription>
            修改后会保存 bcrypt 散列到数据库，并优先于环境变量 <code className="font-mono">NASVIA_PASSWORD</code>。
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-2">
          <div className="w-64">
            <Label>新密码（至少 6 位）</Label>
            <Input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
            />
          </div>
          <Button
            variant="outline"
            disabled={passwordMutation.isPending || password.length < 6}
            onClick={() => passwordMutation.mutate(password)}
          >
            更新密码
          </Button>
        </CardContent>
      </Card>

      <Card className="border-[var(--color-danger)]/35">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-[var(--color-danger)]">
            <ShieldCheck className="size-4" />
            危险操作
          </CardTitle>
          <CardDescription>清空操作不可撤销，配置与密码会保留。</CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            variant="danger"
            className="gap-2"
            onClick={() =>
              setConfirmState({
                title: '清空全部数据？',
                description: '所有站点、分类与已缓存的图标都会被删除，配置与密码保留。此操作不可撤销。',
                action: () => purgeMutation.mutate(),
              })
            }
          >
            <Eraser className="size-4" />
            清空站点与分类
          </Button>
        </CardContent>
      </Card>

      <Dialog
        open={confirmState !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmState(null)
        }}
        title={confirmState?.title}
        description={confirmState?.description}
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmState(null)}>
              取消
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                confirmState?.action()
                setConfirmState(null)
              }}
            >
              确认执行
            </Button>
          </>
        }
      />
    </div>
  )
}

/** 家庭公网出口里 DDNS 域名 / auto 当前解析到的地址（后台排障用）。 */
function HomeEgressStatus({ status }: { status: AdminSettings['home_egress_status'] }) {
  if (status.resolved.length === 0 && !status.error) return null
  return (
    <div className="mt-1.5 flex flex-col gap-0.5 text-xs text-[var(--color-muted-foreground)]">
      {status.resolved.map((row) => (
        <span key={row} className="tabular-nums">
          {row}
        </span>
      ))}
      {status.error ? <span className="text-[var(--color-danger)]">解析失败：{status.error}</span> : null}
    </div>
  )
}

/** 把 extra 里还没有的规则追加到现有文本末尾（不改动用户已有的内容与顺序）。 */
function mergeRules(current: string, extra: readonly string[]): string {
  const existing = normalizeRules(current)
  const seen = new Set(existing.map((rule) => rule.toLowerCase()))
  const merged = [...existing, ...extra.filter((rule) => !seen.has(rule.toLowerCase()))]
  return merged.join('\n')
}
