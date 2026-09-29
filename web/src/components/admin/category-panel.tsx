import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Eraser, Plus, Trash2 } from 'lucide-react'

import { api, apiErrorMessage, type CategoryPayload } from '../../lib/api'
import { CATEGORY_ICON_KEYS, CATEGORY_ICON_NAME, categoryIconKey } from '../../lib/category-icon'
import type { CategoryView } from '../../lib/types'
import { useAdminCategories } from '../../hooks/use-api'
import { toast } from '../../stores/ui'
import { CategoryIcon } from '../category-icon'
import { Button } from '../ui/button'
import { Dialog } from '../ui/dialog'
import { Input, Label } from '../ui/input'
import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/primitives'
import { Select } from '../ui/select'

const CATEGORY_VISIBILITY = [
  { value: 'public', label: '公开', description: '访客可见' },
  // 分类是可见性的上限：私密分类下的站点（含显式公开）对访客一律不可见
  { value: 'private', label: '私密', description: '其下站点对访客一律不可见' },
]

/** 分类管理面板：新建 / 内联编辑 / 图标 / 排序 / 可见性 / 删除。 */
export function CategoryPanel() {
  const queryClient = useQueryClient()
  const categoriesQuery = useAdminCategories()
  const categories: CategoryView[] = categoriesQuery.data?.items ?? []

  const [name, setName] = useState('')
  const [visibility, setVisibility] = useState<'public' | 'private'>('public')
  const [iconTarget, setIconTarget] = useState<CategoryView | null>(null)
  const [confirmState, setConfirmState] = useState<{ title: string; description: string; action: () => void } | null>(
    null,
  )

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['admin', 'categories'] }),
      queryClient.invalidateQueries({ queryKey: ['admin', 'sites'] }),
      queryClient.invalidateQueries({ queryKey: ['categories'] }),
      queryClient.invalidateQueries({ queryKey: ['sites'] }),
    ])
  }

  const createMutation = useMutation({
    mutationFn: (payload: CategoryPayload) => api.createCategory(payload),
    onSuccess: async () => {
      await invalidate()
      setName('')
      toast.success('分类已创建')
    },
    onError: (error) => toast.error('创建失败', apiErrorMessage(error)),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: CategoryPayload }) => api.updateCategory(id, payload),
    onSuccess: invalidate,
    onError: (error) => toast.error('更新失败', apiErrorMessage(error)),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteCategory(id),
    onSuccess: async () => {
      await invalidate()
      toast.success('分类已删除', '其下站点已变为「未分类」')
    },
    onError: (error) => toast.error('删除失败', apiErrorMessage(error)),
  })

  const moveMutation = useMutation({
    mutationFn: ({ id, direction }: { id: number; direction: 'up' | 'down' }) => api.moveCategory(id, direction),
    onSuccess: invalidate,
    onError: (error) => toast.error('排序失败', apiErrorMessage(error)),
  })

  const purgeMutation = useMutation({
    mutationFn: () => api.purgeCategories(),
    onSuccess: async (result) => {
      await invalidate()
      toast.success(`已清空 ${result.deleted} 个分类`)
    },
    onError: (error) => toast.error('清空失败', apiErrorMessage(error)),
  })

  /** 选择分类图标：空串 = 回到「按分类名自动匹配」。 */
  const pickIcon = (icon: string) => {
    if (!iconTarget) return
    updateMutation.mutate({ id: iconTarget.id, payload: { icon } })
    toast.success(icon ? `图标已设为「${CATEGORY_ICON_NAME[categoryIconKey('', icon)]}」` : '已改为按分类名自动匹配')
    setIconTarget(null)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>分类管理</CardTitle>
        <CardDescription>
          分类是可见性的上限：设为私密后，其下站点（含显式公开）对访客一律不可见，后台也不再允许把其中的站点设为公开。
          图标留空时按分类名自动匹配，也可以逐个指定。
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap items-end gap-2">
          <div className="min-w-48 flex-1">
            <Label>新建分类名称</Label>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如：媒体中心"
              onKeyDown={(event) => {
                if (event.key === 'Enter' && name.trim()) {
                  createMutation.mutate({ name: name.trim(), visibility })
                }
              }}
            />
          </div>
          <div className="w-40">
            <Label>可见性</Label>
            <Select value={visibility} onChange={(value) => setVisibility(value as 'public' | 'private')} options={CATEGORY_VISIBILITY} />
          </div>
          <Button
            className="gap-2"
            disabled={createMutation.isPending}
            onClick={() => {
              if (!name.trim()) {
                toast.error('请填写分类名称')
                return
              }
              createMutation.mutate({ name: name.trim(), visibility })
            }}
          >
            <Plus className="size-4" />
            添加分类
          </Button>
          <Button
            variant="outline"
            className="gap-2"
            onClick={() =>
              setConfirmState({
                title: '清空全部分类？',
                description: '分类会被删除，其下站点会变为「未分类」但不会被删除；私密分类里「继承」的站点会改为「私密」，不会因此公开。',
                action: () => purgeMutation.mutate(),
              })
            }
          >
            <Eraser className="size-3.5" />
            清空分类
          </Button>
        </div>

        <div className="flex flex-col gap-2">
          {categories.map((category, index) => (
            <div
              key={category.id}
              className="flex flex-wrap items-center gap-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] p-3"
            >
              <button
                type="button"
                title="选择分类图标"
                aria-label={`${category.name} 的图标`}
                onClick={() => setIconTarget(category)}
                className="flex size-9 shrink-0 items-center justify-center rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] text-[var(--color-foreground)] outline-none transition-colors hover:bg-[var(--color-muted)]"
              >
                <CategoryIcon name={category.name} icon={category.icon} className="size-4" />
              </button>
              <Input
                defaultValue={category.name}
                className="h-9 w-48"
                onBlur={(event) => {
                  const next = event.target.value.trim()
                  if (next && next !== category.name) {
                    updateMutation.mutate({ id: category.id, payload: { name: next } })
                  }
                }}
              />
              <div className="w-36">
                <Select
                  value={category.visibility}
                  onChange={(value) =>
                    updateMutation.mutate({ id: category.id, payload: { visibility: value as 'public' | 'private' } })
                  }
                  options={CATEGORY_VISIBILITY}
                  ariaLabel={`${category.name} 可见性`}
                />
              </div>
              <Badge variant="outline">{category.site_count} 个站点</Badge>
              {category.icon ? <Badge variant="outline">图标：手选</Badge> : <Badge variant="outline">图标：自动</Badge>}
              <div className="ml-auto flex items-center gap-1">
                <Button
                  size="icon-sm"
                  variant="ghost"
                  title="上移"
                  disabled={index === 0}
                  onClick={() => moveMutation.mutate({ id: category.id, direction: 'up' })}
                >
                  <ArrowUp className="size-3.5" />
                </Button>
                <Button
                  size="icon-sm"
                  variant="ghost"
                  title="下移"
                  disabled={index === categories.length - 1}
                  onClick={() => moveMutation.mutate({ id: category.id, direction: 'down' })}
                >
                  <ArrowDown className="size-3.5" />
                </Button>
                <Button
                  size="icon-sm"
                  variant="ghost"
                  title="删除"
                  className="text-[var(--color-danger)]"
                  onClick={() =>
                    setConfirmState({
                      title: `删除分类「${category.name}」？`,
                      description: '其下站点会变为「未分类」，不会被删除；私密分类里「继承」的站点会改为「私密」，不会因此公开。',
                      action: () => deleteMutation.mutate(category.id),
                    })
                  }
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            </div>
          ))}
          {categories.length === 0 ? (
            <p className="rounded-xl border border-dashed border-[var(--color-border)] px-3 py-8 text-center text-xs text-[var(--color-muted-foreground)]">
              还没有分类，先添加一个吧。
            </p>
          ) : null}
        </div>
      </CardContent>

      <Dialog
        open={iconTarget !== null}
        onOpenChange={(open) => {
          if (!open) setIconTarget(null)
        }}
        title={iconTarget ? `分类图标：${iconTarget.name}` : undefined}
        description="选「自动」时按分类名匹配（媒体→影音、下载→下载、私密→锁…），也可以固定指定一个图标。"
        footer={
          <Button variant="ghost" onClick={() => setIconTarget(null)}>
            取消
          </Button>
        }
      >
        <div className="grid grid-cols-3 gap-2 sm:grid-cols-6">
          <button
            type="button"
            onClick={() => pickIcon('')}
            className={
              iconTarget && iconTarget.icon === ''
                ? 'flex h-16 flex-col items-center justify-center gap-1 rounded-xl border border-[var(--color-primary)] bg-[var(--color-accent)] text-xs'
                : 'flex h-16 flex-col items-center justify-center gap-1 rounded-xl border border-[var(--color-border)] text-xs text-[var(--color-muted-foreground)] transition-colors hover:bg-[var(--color-muted)]'
            }
          >
            <CategoryIcon name={iconTarget?.name ?? ''} icon="" className="size-5" />
            自动
          </button>
          {CATEGORY_ICON_KEYS.map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => pickIcon(key)}
              className={
                iconTarget && iconTarget.icon === key
                  ? 'flex h-16 flex-col items-center justify-center gap-1 rounded-xl border border-[var(--color-primary)] bg-[var(--color-accent)] text-xs'
                  : 'flex h-16 flex-col items-center justify-center gap-1 rounded-xl border border-[var(--color-border)] text-xs text-[var(--color-muted-foreground)] transition-colors hover:bg-[var(--color-muted)]'
              }
            >
              <CategoryIcon name={iconTarget?.name ?? ''} iconKey={key} className="size-5" />
              {CATEGORY_ICON_NAME[key]}
            </button>
          ))}
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
    </Card>
  )
}
