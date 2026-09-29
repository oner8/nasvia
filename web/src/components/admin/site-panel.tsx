import { useCallback, useEffect, useMemo, useState, type DragEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronUp,
  ChevronsUpDown,
  Eraser,
  GripVertical,
  Loader2,
  Pin,
  PinOff,
  Plus,
  RefreshCw,
  Save,
  Trash2,
} from 'lucide-react'

import { moveInList, nextSort, SORT_LABELS, sortSites, type SiteSort, type SiteSortKey } from '../../lib/site-order'
import { cn } from '../../lib/utils'

import { api, apiErrorCode, apiErrorMessage, type SitePayload } from '../../lib/api'
import type { CategoryView, Site, Visibility } from '../../lib/types'
import { filterSitesByKeyword } from '../../lib/site-order'
import { useAdminCategories, useAdminSites } from '../../hooks/use-api'
import { toast } from '../../stores/ui'
import { Button } from '../ui/button'
import { Dialog } from '../ui/dialog'
import { Checkbox, Input, Label, Textarea } from '../ui/input'
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/primitives'
import { Select } from '../ui/select'
import { IconPicker } from './icon-picker'

interface Draft {
  id?: number
  name: string
  description: string
  url: string
  lan_url: string
  category_id: number | null
  visibility: Visibility
  tags: string
  pinned: boolean
  /** 手动指定的 HD-Icons 条目名（空 = 自动匹配）。 */
  icon_name: string
  /** 服务端当前的自动匹配结果（只读提示）。 */
  icon_auto: string
}

const EMPTY_DRAFT: Draft = {
  name: '',
  description: '',
  url: '',
  lan_url: '',
  category_id: null,
  visibility: 'inherit',
  tags: '',
  pinned: false,
  icon_name: '',
  icon_auto: '',
}

const VISIBILITY_OPTIONS = [
  { value: 'inherit', label: '继承分类', description: '跟随所属分类的可见性' },
  { value: 'public', label: '公开', description: '任何人都能看到' },
  { value: 'private', label: '私密', description: '仅登录后可见' },
]

/** 可点击排序的表头：三态循环（升序 → 降序 → 手动顺序）。 */
function SortHeader({
  label,
  sortKey,
  sort,
  onSort,
  className,
}: {
  label: string
  sortKey: SiteSortKey
  sort: SiteSort | null
  onSort: (key: SiteSortKey) => void
  className?: string
}) {
  const active = sort?.key === sortKey
  const Icon = !active ? ChevronsUpDown : sort?.dir === 'asc' ? ChevronUp : ChevronDown
  return (
    <th className={className}>
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        title="点击排序：升序 → 降序 → 手动顺序"
        className={cn(
          'flex items-center gap-1 rounded-md px-1 py-0.5 transition hover:bg-[var(--color-muted)]',
          active ? 'text-[var(--color-foreground)]' : 'text-[var(--color-muted-foreground)]',
        )}
      >
        {label}
        <Icon className={cn('size-3', active ? 'opacity-100' : 'opacity-50')} />
      </button>
    </th>
  )
}

/** 站点管理面板：表格 + 批量操作 + 表单。 */
export function SitePanel() {
  const queryClient = useQueryClient()
  const sitesQuery = useAdminSites()
  const categoriesQuery = useAdminCategories()

  const [selected, setSelected] = useState<number[]>([])
  const [fetchingIconIDs, setFetchingIconIDs] = useState<Set<number>>(() => new Set())
  const [keyword, setKeyword] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [draft, setDraft] = useState<Draft>(EMPTY_DRAFT)
  const [confirmState, setConfirmState] = useState<{ title: string; description: string; action: () => void } | null>(
    null,
  )

  const [sort, setSort] = useState<SiteSort | null>(null)
  /** 拖动过程中的本地顺序（null = 用服务端顺序）。 */
  const [dragOrder, setDragOrder] = useState<number[] | null>(null)
  const [dragId, setDragId] = useState<number | null>(null)

  const sites = sitesQuery.data?.items ?? []
  const categories: CategoryView[] = categoriesQuery.data?.items ?? []

  const categoryNames = useMemo(() => {
    const map = new Map<number, string>()
    for (const category of categories) map.set(category.id, category.name)
    return map
  }, [categories])

  /** 拖动期间用本地顺序覆盖服务端顺序；松手保存后交还服务端顺序。 */
  const orderedSites = useMemo(() => {
    if (!dragOrder) return sites
    const byId = new Map(sites.map((site) => [site.id, site]))
    return dragOrder
      .map((id) => byId.get(id))
      .filter((site): site is Site => Boolean(site))
  }, [sites, dragOrder])

  const matched = useMemo(() => filterSitesByKeyword(orderedSites, keyword), [orderedSites, keyword])

  const categoryNameOf = useCallback(
    (id: number | null) => (id === null ? '未分类' : (categoryNames.get(id) ?? '')),
    [categoryNames],
  )

  /** 表头排序后的最终展示列表（sort 为 null 时就是手动顺序）。 */
  const filtered = useMemo(() => sortSites(matched, sort, categoryNameOf), [matched, sort, categoryNameOf])

  useEffect(() => {
    setSelected((current) => current.filter((id) => sites.some((site) => site.id === id)))
  }, [sites])

  /** 分类是否为私密分类（可见性上限）。 */
  const categoryIsPrivate = useCallback(
    (id: number | null) =>
      id !== null && categories.some((category) => category.id === id && category.visibility === 'private'),
    [categories],
  )

  /** 表单里：分类私密时不允许把站点设为公开。 */
  const publicLocked = draft.category_id !== null && categoryIsPrivate(draft.category_id)
  const visibilityOptions = useMemo(
    () =>
      VISIBILITY_OPTIONS.map((option) =>
        option.value === 'public' && publicLocked
          ? {
              ...option,
              label: '公开（不可选）',
              description: '所属分类是私密分类，站点不能单独设为公开',
              disabled: true,
            }
          : option,
      ),
    [publicLocked],
  )

  /** 选中项里是否含私密分类的站点：批量「设为公开」要禁用。 */
  const selectionPublicLocked = useMemo(
    () =>
      selected.some((id) => {
        const site = sites.find((item) => item.id === id)
        return site ? categoryIsPrivate(site.category_id) : false
      }),
    [selected, sites, categoryIsPrivate],
  )

  // 分类改成私密（或编辑历史数据）时把「公开」回落到「继承」，避免提交必然失败。
  useEffect(() => {
    if (!dialogOpen || draft.visibility !== 'public' || draft.category_id === null) return
    if (!categoryIsPrivate(draft.category_id)) return
    setDraft((current) => ({ ...current, visibility: 'inherit' }))
  }, [dialogOpen, draft.visibility, draft.category_id, categoryIsPrivate])

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['admin', 'sites'] }),
      queryClient.invalidateQueries({ queryKey: ['admin', 'categories'] }),
      queryClient.invalidateQueries({ queryKey: ['sites'] }),
      queryClient.invalidateQueries({ queryKey: ['categories'] }),
    ])
  }

  const saveMutation = useMutation({
    mutationFn: async (payload: Draft) => {
      const body: SitePayload = {
        name: payload.name,
        description: payload.description,
        url: payload.url,
        lan_url: payload.lan_url,
        visibility: payload.visibility,
        tags: payload.tags,
        pinned: payload.pinned,
        clear_category: payload.category_id === null,
      }
      const iconName = payload.icon_name.trim()
      if (iconName) body.icon_name = iconName
      else body.clear_icon_name = true
      if (payload.category_id !== null) body.category_id = payload.category_id
      if (payload.id) return api.updateSite(payload.id, body)
      return api.createSite(body)
    },
    onSuccess: async (site, variables) => {
      await invalidate()
      setDialogOpen(false)
      setDraft(EMPTY_DRAFT)
      toast.success(variables.id ? '站点已更新' : '站点已创建', '图标将在后台自动抓取')
    },
    onError: (error) => toast.error('保存失败', apiErrorMessage(error)),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteSite(id),
    onSuccess: async () => {
      await invalidate()
      toast.success('站点已删除')
    },
    onError: (error) => toast.error('删除失败', apiErrorMessage(error)),
  })

  const batchMutation = useMutation({
    mutationFn: (payload: Parameters<typeof api.batchSites>[0]) => api.batchSites(payload),
    onSuccess: async (result) => {
      await invalidate()
      setSelected([])
      toast.success(`已处理 ${result.affected} 个站点`)
    },
    onError: (error) => toast.error('批量操作失败', apiErrorMessage(error)),
  })

  const moveMutation = useMutation({
    mutationFn: ({ id, direction }: { id: number; direction: 'up' | 'down' }) => api.moveSite(id, direction),
    onSuccess: invalidate,
    onError: (error) => toast.error('排序失败', apiErrorMessage(error)),
  })

  const reorderMutation = useMutation({
    mutationFn: (ids: number[]) => api.reorderSites(ids),
    onSuccess: async () => {
      await invalidate()
      setDragOrder(null)
      toast.success('顺序已保存')
    },
    onError: (error) => {
      setDragOrder(null)
      toast.error('顺序保存失败', apiErrorMessage(error))
    },
  })

  const markIconPending = (id: number) => {
    for (const queryKey of [['admin', 'sites'], ['sites']] as const) {
      queryClient.setQueryData<{ items: Site[] }>(queryKey, (current) =>
        current
          ? {
              ...current,
              items: current.items.map((site) =>
                site.id === id ? { ...site, icon_state: 'pending' } : site,
              ),
            }
          : current,
      )
    }
  }

  const refetchIcon = async (id: number) => {
    setFetchingIconIDs((current) => new Set(current).add(id))
    try {
      await api.refetchIcon(id)
      markIconPending(id)
      toast.success('已重新排队抓取图标')
    } catch (error) {
      if (apiErrorCode(error) === 'icon_fetch_in_progress') {
        markIconPending(id)
        toast.success('该站点的图标正在抓取')
      } else {
        toast.error('抓取失败', apiErrorMessage(error))
      }
    } finally {
      setFetchingIconIDs((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
    }
  }

  const purgeMutation = useMutation({
    mutationFn: () => api.purgeSites(),
    onSuccess: async (result) => {
      await invalidate()
      toast.success(`已清空 ${result.deleted} 个站点`)
    },
    onError: (error) => toast.error('清空失败', apiErrorMessage(error)),
  })

  const allSelected = filtered.length > 0 && filtered.every((site) => selected.includes(site.id))

  const toggleAll = () => {
    setSelected(allSelected ? [] : filtered.map((site) => site.id))
  }

  const openCreate = () => {
    setDraft({ ...EMPTY_DRAFT, category_id: categories[0]?.id ?? null })
    setDialogOpen(true)
  }

  const openEdit = (site: Site) => {
    setDraft({
      id: site.id,
      name: site.name,
      description: site.description,
      url: site.url,
      lan_url: site.lan_url,
      category_id: site.category_id,
      visibility: site.visibility,
      tags: site.tags,
      pinned: site.pinned,
      icon_name: site.icon_name ?? '',
      icon_auto: site.icon_auto ?? '',
    })
    setDialogOpen(true)
  }

  /** 只有「手动顺序」下允许拖动：表头排序会重排显示顺序，此时拖动没有意义。 */
  const canDrag = sort === null && !reorderMutation.isPending

  const handleSort = (key: SiteSortKey) => setSort((current) => nextSort(current, key))

  const handleDragStart = (event: DragEvent<HTMLTableRowElement>, id: number) => {
    if (!canDrag) return
    setDragId(id)
    setDragOrder(orderedSites.map((site) => site.id))
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(id))
  }

  /** 拖动经过某行：按光标是否过半决定插到该行前/后，列表实时让位。 */
  const handleDragOver = (event: DragEvent<HTMLTableRowElement>, targetId: number) => {
    if (!canDrag || dragId === null) return
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
    const rect = event.currentTarget.getBoundingClientRect()
    const after = event.clientY > rect.top + rect.height / 2
    const current = dragOrder ?? orderedSites.map((site) => site.id)
    const next = moveInList(current, dragId, targetId, after)
    if (next.join(',') !== current.join(',')) setDragOrder(next)
  }

  /** 松手：顺序有变化才提交（提交全量顺序，服务端要求不重不漏）。 */
  const finishDrag = () => {
    const ids = dragOrder
    setDragId(null)
    if (!ids) return
    if (ids.join(',') === sites.map((site) => site.id).join(',')) {
      setDragOrder(null)
      return
    }
    reorderMutation.mutate(ids)
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="flex-row items-center justify-between gap-3 pb-3">
          <div>
            <CardTitle>站点管理</CardTitle>
            <CardDescription>共 {sites.length} 个站点，选中后可批量设置可见性或删除。</CardDescription>
          </div>
          <Button onClick={openCreate} className="gap-2">
            <Plus className="size-4" />
            新建站点
          </Button>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder="筛选站点名称 / 描述 / 地址"
              className="max-w-xs"
            />
            {selected.length > 0 ? (
              <>
                <Badge variant="primary">已选 {selected.length}</Badge>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={selectionPublicLocked}
                  title={selectionPublicLocked ? '选中的站点里有属于私密分类的，不能设为公开' : undefined}
                  onClick={() => batchMutation.mutate({ ids: selected, action: 'visibility', visibility: 'public' })}
                >
                  设为公开
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => batchMutation.mutate({ ids: selected, action: 'visibility', visibility: 'private' })}
                >
                  设为私密
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => batchMutation.mutate({ ids: selected, action: 'visibility', visibility: 'inherit' })}
                >
                  设为继承
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1"
                  onClick={() => batchMutation.mutate({ ids: selected, action: 'pinned', pinned: true })}
                >
                  <Pin className="size-3.5" />
                  置顶
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="gap-1"
                  onClick={() => batchMutation.mutate({ ids: selected, action: 'pinned', pinned: false })}
                >
                  <PinOff className="size-3.5" />
                  取消置顶
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  className="gap-1"
                  onClick={() =>
                    setConfirmState({
                      title: `删除选中的 ${selected.length} 个站点？`,
                      description: '删除后无法恢复，请确认。',
                      action: () => batchMutation.mutate({ ids: selected, action: 'delete' }),
                    })
                  }
                >
                  <Trash2 className="size-3.5" />
                  批量删除
                </Button>
              </>
            ) : null}
            <div className="ml-auto flex items-center gap-2">
              <Button
                size="sm"
                variant="ghost"
                className="gap-1 text-[var(--color-muted-foreground)]"
                onClick={() => setKeyword('')}
              >
                重置筛选
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="gap-1"
                onClick={() =>
                  setConfirmState({
                    title: '清空全部站点？',
                    description: '所有站点（含私密站点）都会被删除，分类会保留。此操作不可撤销。',
                    action: () => purgeMutation.mutate(),
                  })
                }
              >
                <Eraser className="size-3.5" />
                清空站点
              </Button>
            </div>
          </div>

          {sort ? (
            <p className="text-xs text-[var(--color-muted-foreground)]">
              已按「{SORT_LABELS[sort.key]}」{sort.dir === 'asc' ? '升序' : '降序'}排列：拖动与上下移动已停用（再点表头切到降序，
              再点一次回到手动顺序）。
            </p>
          ) : null}

          {sitesQuery.iconPollingTimedOut ? (
            <div className="flex items-center justify-between gap-3 rounded-xl bg-[var(--color-muted)]/50 px-3 py-2 text-xs">
              <span>后台仍在处理图标，自动检查已暂停。</span>
              <Button size="sm" variant="outline" onClick={sitesQuery.retryIconPolling}>
                刷新图标状态
              </Button>
            </div>
          ) : null}

          <div className="overflow-x-auto rounded-xl border border-[var(--color-border)]">
            <table className="w-full min-w-[52rem] border-collapse text-sm">
              <thead className="bg-[var(--color-muted)]/60 text-left text-xs text-[var(--color-muted-foreground)]">
                <tr>
                  <th className="w-10 px-3 py-2">
                    <Checkbox checked={allSelected} onChange={toggleAll} aria-label="全选" />
                  </th>
                  <SortHeader label="站点" sortKey="name" sort={sort} onSort={handleSort} className="px-3 py-2" />
                  <SortHeader label="分类" sortKey="category" sort={sort} onSort={handleSort} className="px-3 py-2" />
                  <SortHeader
                    label="可见性"
                    sortKey="visibility"
                    sort={sort}
                    onSort={handleSort}
                    className="px-3 py-2"
                  />
                  <SortHeader label="地址" sortKey="address" sort={sort} onSort={handleSort} className="px-3 py-2" />
                  <th className="w-40 px-3 py-2 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((site) => {
                  const fetchingIcon = fetchingIconIDs.has(site.id) || site.icon_state === 'pending'
                  return (
                  <tr
                    key={site.id}
                    draggable={canDrag}
                    onDragStart={(event) => handleDragStart(event, site.id)}
                    onDragOver={(event) => handleDragOver(event, site.id)}
                    onDrop={(event) => {
                      event.preventDefault()
                      finishDrag()
                    }}
                    onDragEnd={finishDrag}
                    className={cn(
                      'border-t border-[var(--color-border)] align-middle transition-colors',
                      dragId === site.id ? 'opacity-50' : 'hover:bg-[var(--color-muted)]/40',
                    )}
                  >
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-1.5">
                        <span
                          title={canDrag ? '拖动调整顺序' : '表头排序中不可拖动；点表头可回到手动顺序'}
                          className={cn('shrink-0', canDrag ? 'cursor-grab' : 'cursor-not-allowed opacity-40')}
                        >
                          <GripVertical className="size-3.5" />
                        </span>
                        <Checkbox
                          checked={selected.includes(site.id)}
                          aria-label={`选择 ${site.name}`}
                          onChange={(event) =>
                            setSelected((current) =>
                              event.target.checked
                                ? [...current, site.id]
                                : current.filter((id) => id !== site.id),
                            )
                          }
                        />
                      </div>
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-2">
                        <img
                          src={site.icon}
                          alt=""
                          className="size-7 rounded-lg bg-[var(--color-muted)] object-contain p-0.5"
                        />
                        <div className="min-w-0">
                          <p className="flex items-center gap-1 truncate font-medium">
                            {site.name}
                            {site.pinned ? <Pin className="size-3 text-[var(--color-primary)]" /> : null}
                          </p>
                          <p className="truncate text-xs text-[var(--color-muted-foreground)]">
                            {site.description || '—'}
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-2 text-xs text-[var(--color-muted-foreground)]">
                      {site.category_id !== null ? (categoryNames.get(site.category_id) ?? '—') : '未分类'}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex flex-col gap-1">
                        <Badge
                          variant={
                            site.effective_visibility === 'private'
                              ? 'danger'
                              : site.visibility === 'public'
                                ? 'success'
                                : 'default'
                          }
                        >
                          {site.visibility === 'inherit' ? '继承' : site.visibility === 'public' ? '公开' : '私密'}
                        </Badge>
                        <span className="text-[0.7rem] text-[var(--color-muted-foreground)]">
                          生效：{site.effective_visibility === 'public' ? '公开' : '私密'}
                        </span>
                      </div>
                    </td>
                    <td className="max-w-[16rem] px-3 py-2">
                      <p className="truncate text-xs">{site.url || '—'}</p>
                      <p className="truncate text-xs text-[var(--color-muted-foreground)]">
                        {site.lan_url || '未设置内网地址'}
                      </p>
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          title={canDrag ? '上移' : '表头排序中不可移动，点表头可回到手动顺序'}
                          disabled={!canDrag}
                          onClick={() => moveMutation.mutate({ id: site.id, direction: 'up' })}
                        >
                          <ArrowUp className="size-3.5" />
                        </Button>
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          title={canDrag ? '下移' : '表头排序中不可移动，点表头可回到手动顺序'}
                          disabled={!canDrag}
                          onClick={() => moveMutation.mutate({ id: site.id, direction: 'down' })}
                        >
                          <ArrowDown className="size-3.5" />
                        </Button>
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          title="重新抓取图标"
                          disabled={fetchingIcon}
                          onClick={() => void refetchIcon(site.id)}
                        >
                          <RefreshCw className={cn('size-3.5', fetchingIcon && 'animate-spin')} />
                        </Button>
                        <Button size="sm" variant="outline" onClick={() => openEdit(site)}>
                          编辑
                        </Button>
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          title="删除"
                          className="text-[var(--color-danger)]"
                          onClick={() =>
                            setConfirmState({
                              title: `删除「${site.name}」？`,
                              description: '删除后无法恢复。',
                              action: () => deleteMutation.mutate(site.id),
                            })
                          }
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                  )
                })}
                {filtered.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-3 py-10 text-center text-xs text-[var(--color-muted-foreground)]">
                      {sites.length === 0 ? '还没有站点，点击右上角「新建站点」开始。' : '没有匹配的站点。'}
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setDraft(EMPTY_DRAFT)
        }}
        title={draft.id ? '编辑站点' : '新建站点'}
        description="填写内网与外网地址后，页面会根据访问来源自动选择；图标由服务端自动抓取。"
        footer={
          <>
            <Button variant="ghost" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button
              className="gap-2"
              disabled={saveMutation.isPending}
              onClick={() => {
                if (!draft.name.trim()) {
                  toast.error('请填写站点名称')
                  return
                }
                if (!draft.url.trim() && !draft.lan_url.trim()) {
                  toast.error('内网地址与外网地址至少填写一个')
                  return
                }
                saveMutation.mutate(draft)
              }}
            >
              {saveMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </Button>
          </>
        }
      >
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <Label>站点名称 *</Label>
            <Input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          </div>
          <div className="sm:col-span-2">
            <Label>描述</Label>
            <Textarea
              rows={2}
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              placeholder="一句话说明这个服务是做什么的"
            />
          </div>
          <div>
            <Label>外网地址</Label>
            <Input
              value={draft.url}
              onChange={(event) => setDraft({ ...draft, url: event.target.value })}
              placeholder="https://jellyfin.example.com"
            />
          </div>
          <div>
            <Label>内网地址</Label>
            <Input
              value={draft.lan_url}
              onChange={(event) => setDraft({ ...draft, lan_url: event.target.value })}
              placeholder="http://192.168.1.10:8096"
            />
          </div>
          <div>
            <Label>所属分类</Label>
            <Select
              value={draft.category_id === null ? 'none' : String(draft.category_id)}
              onChange={(value) => setDraft({ ...draft, category_id: value === 'none' ? null : Number(value) })}
              options={[
                { value: 'none', label: '未分类' },
                ...categories.map((category) => ({
                  value: String(category.id),
                  label: category.name,
                  description: category.visibility === 'private' ? '私密分类' : undefined,
                })),
              ]}
            />
          </div>
          <div>
            <Label>可见性</Label>
            <Select
              value={draft.visibility}
              onChange={(value) => setDraft({ ...draft, visibility: value as Visibility })}
              options={visibilityOptions}
            />
            {publicLocked ? (
              <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
                所属分类是私密分类，站点不能单独设为公开；已按「继承」处理。
              </p>
            ) : null}
          </div>
          <div>
            <Label>标签（空格或逗号分隔）</Label>
            <Input value={draft.tags} onChange={(event) => setDraft({ ...draft, tags: event.target.value })} />
          </div>
          <div className="sm:col-span-2">
            <IconPicker
              value={draft.icon_name}
              auto={draft.icon_auto}
              onChange={(value) => setDraft({ ...draft, icon_name: value })}
            />
          </div>
          <div className="flex items-end gap-2 pb-2">
            <Checkbox
              id="site-pinned"
              checked={draft.pinned}
              onChange={(event) => setDraft({ ...draft, pinned: event.target.checked })}
            />
            <label htmlFor="site-pinned" className="text-xs text-[var(--color-muted-foreground)]">
              置顶到「常用」
            </label>
          </div>
        </div>
      </Dialog>

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
