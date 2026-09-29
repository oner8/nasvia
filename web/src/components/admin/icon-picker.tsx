import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ImageOff, Loader2, Search, X } from 'lucide-react'

import { api } from '../../lib/api'
import { Button } from '../ui/button'
import { Input, Label } from '../ui/input'

interface IconPickerProps {
  /** 手动指定的 HD-Icons 条目名（空 = 自动匹配）。 */
  value: string
  /** 服务端当前的自动匹配结果，用来提示「不指定会用哪个」。 */
  auto: string
  onChange: (value: string) => void
}

/**
 * 图标选择器：搜 HD-Icons 索引里的条目名（支持中文关键词，如「网盘」会列出 drive 类图标），
 * 选中后写回 icon_name；留空则回到自动匹配。
 */
export function IconPicker({ value, auto, onChange }: IconPickerProps) {
  const [keyword, setKeyword] = useState('')
  const [debounced, setDebounced] = useState('')

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(keyword.trim()), 250)
    return () => clearTimeout(timer)
  }, [keyword])

  const suggestionsQuery = useQuery({
    queryKey: ['admin', 'hdicons', 'search', debounced],
    queryFn: () => api.searchHDIcons(debounced),
    enabled: debounced.length > 0,
    staleTime: 5 * 60 * 1000,
  })

  const suggestions = useMemo(() => suggestionsQuery.data?.items ?? [], [suggestionsQuery.data])
  const searching = suggestionsQuery.isFetching

  const previewURL = value ? api.iconPreviewURL(value) : ''

  const pick = (name: string) => {
    onChange(name)
    setKeyword('')
    setDebounced('')
  }

  return (
    <div className="flex flex-col gap-2">
      <Label>图标</Label>
      <div className="flex items-start gap-2">
        <span className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-[var(--color-border)] bg-[var(--color-muted)]">
          {previewURL ? (
            <img src={previewURL} alt="" className="size-8 rounded-lg object-contain" />
          ) : (
            <ImageOff className="size-4 text-[var(--color-muted-foreground)]" />
          )}
        </span>
        <div className="relative min-w-0 flex-1">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-[var(--color-muted-foreground)]" />
          <Input
            className="pl-8"
            value={keyword}
            placeholder="搜索图标名：baidu、群晖、网盘…"
            onChange={(event) => setKeyword(event.target.value)}
          />
          {debounced ? (
            <div className="absolute z-20 mt-1 max-h-56 w-full overflow-y-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-popover)] p-1 shadow-md">
              {searching && suggestions.length === 0 ? (
                <p className="flex items-center gap-2 px-2 py-2 text-xs text-[var(--color-muted-foreground)]">
                  <Loader2 className="size-3 animate-spin" />
                  搜索中…
                </p>
              ) : null}
              {!searching && suggestions.length === 0 ? (
                <p className="px-2 py-2 text-xs text-[var(--color-muted-foreground)]">
                  没找到，换一个词试试（两个来源一起搜：HD-Icons 是英文名，nasicon 带中文名）
                </p>
              ) : null}
              {suggestions.map((name) => (
                <button
                  key={name}
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs hover:bg-[var(--color-accent)]"
                  onClick={() => pick(name)}
                >
                  <img src={api.iconPreviewURL(name)} alt="" className="size-6 rounded-md object-contain" />
                  <span className="truncate">{name.replace(/^nasicon:/, '')}</span>
                </button>
              ))}
            </div>
          ) : null}
        </div>
        {value ? (
          <Button variant="ghost" size="icon-sm" onClick={() => pick('')} title="清空，改回自动匹配">
            <X className="size-4" />
          </Button>
        ) : null}
      </div>
      <p className="text-xs text-[var(--color-muted-foreground)]">
        {value
          ? `已指定：${value.replace(/^nasicon:/, '')}（保存后按它抓取，需在设置里保持 HD-Icons 或 nasicon 来源开启）`
          : auto
            ? `留空 = 自动匹配（当前匹配到 ${auto.replace(/^nasicon:/, '')}）`
            : '留空 = 自动匹配（当前按名称与域名都没匹配到，可在这里手动指定）'}
      </p>
    </div>
  )
}
