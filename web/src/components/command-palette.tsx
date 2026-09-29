import { useEffect, useMemo, useState } from 'react'
import { Command } from 'cmdk'
import { CornerDownLeft, ExternalLink, Search } from 'lucide-react'

import { searchSites } from '../lib/search'
import type { Site } from '../lib/types'
import { avatarGradient, firstLetter, shortHost } from '../lib/utils'
import { useUI } from '../stores/ui'
import { Dialog } from './ui/dialog'

export interface CommandPaletteProps {
  sites: Site[]
  resolveUrl: (site: Site) => string
}

function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || tag === 'select' || target.isContentEditable
}

/** ⌘K 全局搜索：按名称、描述、地址模糊匹配，键盘上下选择 + 回车打开。 */
export function CommandPalette({ sites, resolveUrl }: CommandPaletteProps) {
  const open = useUI((state) => state.paletteOpen)
  const setOpen = useUI((state) => state.setPaletteOpen)
  const [query, setQuery] = useState('')

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const paletteKey = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k'
      const slashKey = event.key === '/' && !isEditable(event.target)
      if (paletteKey || slashKey) {
        event.preventDefault()
        setOpen(!useUI.getState().paletteOpen)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [setOpen])

  useEffect(() => {
    if (!open) setQuery('')
  }, [open])

  const results = useMemo(() => searchSites(sites, query).slice(0, 40), [sites, query])

  const openSite = (site: Site) => {
    const url = resolveUrl(site)
    if (url) {
      window.open(url, '_blank', 'noopener,noreferrer')
    }
    setOpen(false)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen} className="w-[min(94vw,38rem)]" bodyClassName="p-0">
      <Command shouldFilter={false} loop className="flex flex-col">
        {/* 右侧留出 pr-14：Dialog 自带的关闭按钮悬浮在右上角，避免与 ESC 提示重叠 */}
        <div className="flex items-center gap-2.5 border-b border-[var(--color-border)] py-3.5 pl-4 pr-14">
          <Search className="size-4 shrink-0 text-[var(--color-muted-foreground)]" />
          <Command.Input
            autoFocus
            value={query}
            onValueChange={setQuery}
            placeholder="搜索服务名称、描述或地址…"
            className="h-7 w-full bg-transparent text-sm outline-none placeholder:text-[var(--color-muted-foreground)]"
          />
          <kbd className="rounded-md border border-[var(--color-border)] px-1.5 py-0.5 text-[0.7rem] text-[var(--color-muted-foreground)]">
            ESC
          </kbd>
        </div>
        <Command.List className="max-h-[22rem] overflow-y-auto p-2">
          <Command.Empty className="px-3 py-10 text-center text-xs text-[var(--color-muted-foreground)]">
            没有匹配的服务
          </Command.Empty>
          {results.map((site) => {
            const url = resolveUrl(site)
            return (
              <Command.Item
                key={site.id}
                value={String(site.id)}
                onSelect={() => openSite(site)}
                className="flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 outline-none data-[selected=true]:bg-[var(--color-muted)]"
              >
                <span
                  className="flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-[var(--color-muted)] text-xs font-semibold text-white"
                  style={{ backgroundImage: avatarGradient(site.name) }}
                >
                  <img
                    src={site.icon}
                    alt=""
                    className="size-full object-contain p-1"
                    onError={(event) => {
                      const img = event.currentTarget
                      img.style.display = 'none'
                      img.parentElement?.append(firstLetter(site.name))
                    }}
                  />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{site.name}</span>
                  <span className="block truncate text-xs text-[var(--color-muted-foreground)]">
                    {site.description || shortHost(url)}
                  </span>
                </span>
                <span className="hidden shrink-0 items-center gap-1 text-xs text-[var(--color-muted-foreground)] sm:flex">
                  <ExternalLink className="size-3" />
                  {shortHost(url)}
                </span>
                <CornerDownLeft className="size-3.5 shrink-0 text-[var(--color-muted-foreground)]" />
              </Command.Item>
            )
          })}
        </Command.List>
        <div className="flex items-center justify-between border-t border-[var(--color-border)] px-4 py-2.5 text-xs text-[var(--color-muted-foreground)]">
          <span>{results.length} 个结果</span>
          <span>↑ ↓ 选择 · ⏎ 打开 · ⌘K 关闭</span>
        </div>
      </Command>
    </Dialog>
  )
}
