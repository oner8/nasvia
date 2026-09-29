import { Menu as BaseMenu } from '@base-ui/react/menu'
import { Check, ChevronDown, Search, X } from 'lucide-react'
import type { FormEvent, KeyboardEvent } from 'react'
import { useEffect, useId, useRef, useState } from 'react'

import { api } from '../lib/api'
import {
  DEFAULT_ENGINE_ID,
  ENGINE_STORAGE_KEY,
  SEARCH_ENGINES,
  buildSearchUrl,
  getEngine,
  normalizeEngineId,
  readStoredEngineId,
} from '../lib/search-engines'
import { moveActive, splitSuggestion } from '../lib/search-suggest'
import { cn } from '../lib/utils'
import { EngineLogo } from './engine-logo'
import { Tooltip } from './ui/tooltip'

export interface WebSearchProps {
  className?: string
  /** 打开搜索结果页后的回调（弹窗模式下用于关闭弹窗）。 */
  onSubmitted?: () => void
}

/**
 * Google 风格的联网搜索框：左侧下拉选择搜索引擎（选项为各家 logo），
 * 输入时下方弹出联想词（上下键选择、回车搜索、Esc 收起），
 * 回车或点右侧按钮即在新标签页打开对应引擎的结果页。
 * 仅检索互联网，不涉及 NASVIA 的站点数据；引擎选择保存在 localStorage。
 */
export function WebSearch({ className, onSubmitted }: WebSearchProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const listId = useId()
  const [query, setQuery] = useState('')
  const [engineId, setEngineId] = useState<string>(DEFAULT_ENGINE_ID)
  // 联想词：items 为当前关键词的联想结果；active 为键盘选中的下标（-1 表示输入框里自己打的字）
  const [items, setItems] = useState<string[]>([])
  const [active, setActive] = useState(-1)
  const [open, setOpen] = useState(false)

  // 挂载后取上次选择的引擎，避免首屏水合不一致
  useEffect(() => {
    setEngineId(readStoredEngineId(typeof window === 'undefined' ? undefined : window.localStorage))
  }, [])

  const engine = getEngine(engineId)

  // 输入停顿 150ms 再取联想词；继续输入时取消上一次请求，避免旧结果覆盖新结果
  useEffect(() => {
    const keyword = query.trim()
    if (!keyword) {
      setItems([])
      setActive(-1)
      return
    }
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      api
        .suggest(engineId, keyword, controller.signal)
        .then((list) => {
          setItems(list)
          setActive(-1)
        })
        .catch(() => {
          /* 联想词取不到不影响搜索本身 */
        })
    }, 150)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [query, engineId])

  const showList = open && items.length > 0
  // 用键盘选中联想词时，输入框同步显示那一条（与 Google 一致）；鼠标悬停不改输入框
  const display = active >= 0 && items[active] ? items[active] : query

  const pickEngine = (id: string) => {
    const next = normalizeEngineId(id)
    setEngineId(next)
    try {
      window.localStorage.setItem(ENGINE_STORAGE_KEY, next)
    } catch {
      /* 隐私模式下写入失败可忽略，本次会话内仍然生效 */
    }
    inputRef.current?.focus()
  }

  const search = (keyword: string) => {
    const url = buildSearchUrl(keyword, engineId)
    if (!url) {
      inputRef.current?.focus()
      return
    }
    window.open(url, '_blank', 'noopener,noreferrer')
    setOpen(false)
    setActive(-1)
    onSubmitted?.()
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    search(display)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing) return // 中文输入法选词中，方向键和回车交给输入法
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      if (items.length === 0) return
      event.preventDefault()
      if (!open) {
        setOpen(true)
        return
      }
      setActive((current) => moveActive(current, event.key === 'ArrowDown' ? 1 : -1, items.length))
    } else if (event.key === 'Escape' && showList) {
      event.preventDefault()
      setOpen(false)
      setActive(-1)
    }
  }

  return (
    <div className={cn('relative mx-auto w-full max-w-2xl', className)}>
      <form
        role="search"
        aria-label="联网搜索"
        onSubmit={onSubmit}
        className={cn(
          'flex w-full items-center gap-1.5 rounded-full border border-[var(--color-border)]',
          'bg-[var(--color-card)] py-1.5 pl-1.5 pr-1.5 shadow-soft',
        )}
      >
        {/* 左侧：搜索引擎下拉（选项为对应 logo），类型固定为 button，避免误提交表单 */}
        <BaseMenu.Root modal={false}>
          <Tooltip content={`当前搜索引擎：${engine.name}，点击切换`} side="bottom">
            <BaseMenu.Trigger
              render={
                <button
                  type="button"
                  aria-label={`当前搜索引擎：${engine.name}，点击切换`}
                  className="flex h-9 shrink-0 cursor-pointer items-center gap-1 rounded-full px-2.5 text-[var(--color-muted-foreground)] outline-none transition-colors hover:bg-[var(--color-muted)] hover:text-[var(--color-foreground)]"
                />
              }
            >
              <EngineLogo id={engine.id} />
              <ChevronDown className="size-3.5" />
            </BaseMenu.Trigger>
          </Tooltip>
          <BaseMenu.Portal>
            <BaseMenu.Positioner side="bottom" align="start" sideOffset={8} className="z-[70] outline-none">
              <BaseMenu.Popup
                className={cn(
                  'min-w-[11rem] rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] p-1 shadow-soft outline-none',
                  'transition data-[starting-style]:scale-95 data-[starting-style]:opacity-0',
                  'data-[ending-style]:scale-95 data-[ending-style]:opacity-0',
                )}
              >
                <BaseMenu.RadioGroup value={engine.id} onValueChange={(value) => pickEngine(String(value))}>
                  {SEARCH_ENGINES.map((item) => (
                    <BaseMenu.RadioItem
                      key={item.id}
                      value={item.id}
                      closeOnClick
                      aria-label={`用 ${item.name} 搜索`}
                      className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-[var(--color-muted)]"
                    >
                      <EngineLogo id={item.id} />
                      <span className="flex-1 whitespace-nowrap">{item.name}</span>
                      <BaseMenu.RadioItemIndicator className="text-[var(--color-primary)]">
                        <Check className="size-3.5" />
                      </BaseMenu.RadioItemIndicator>
                    </BaseMenu.RadioItem>
                  ))}
                </BaseMenu.RadioGroup>
              </BaseMenu.Popup>
            </BaseMenu.Positioner>
          </BaseMenu.Portal>
        </BaseMenu.Root>

        <input
          ref={inputRef}
          type="search"
          role="combobox"
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList && active >= 0 ? `${listId}-${active}` : undefined}
          value={display}
          onChange={(event) => {
            setQuery(event.target.value)
            setActive(-1)
            setOpen(true)
          }}
          onKeyDown={onKeyDown}
          onFocus={() => setOpen(true)}
          onBlur={() => {
            setOpen(false)
            setActive(-1)
          }}
          placeholder={`用 ${engine.name} 搜索网页…`}
          aria-label={`用 ${engine.name} 搜索网页`}
          enterKeyHint="search"
          autoComplete="off"
          className={cn(
            'h-9 min-w-0 flex-1 bg-transparent text-sm text-[var(--color-card-foreground)] outline-none',
            'placeholder:text-[var(--color-muted-foreground)]',
            '[&::-webkit-search-cancel-button]:appearance-none',
          )}
        />

        {query ? (
          <Tooltip content="清空" side="bottom">
            <button
              type="button"
              aria-label="清空"
              onClick={() => {
                setQuery('')
                setItems([])
                setActive(-1)
                inputRef.current?.focus()
              }}
              className="flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-full text-[var(--color-muted-foreground)] outline-none transition-colors hover:bg-[var(--color-muted)] hover:text-[var(--color-foreground)]"
            >
              <X className="size-4" />
            </button>
          </Tooltip>
        ) : null}

        <Tooltip content="搜索（新标签页打开）" side="bottom">
          <button
            type="submit"
            aria-label="搜索"
            className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-[var(--color-primary)] text-[var(--color-primary-foreground)] outline-none transition hover:opacity-90"
          >
            <Search className="size-4" />
          </button>
        </Tooltip>
      </form>

      {showList ? (
        <ul
          id={listId}
          role="listbox"
          aria-label="搜索联想"
          className="absolute inset-x-0 top-full z-40 mt-2 overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-card)] py-1.5 shadow-soft"
        >
          {items.map((item, index) => {
            const [typed, completion] = splitSuggestion(item, query)
            return (
              <li
                key={item}
                id={`${listId}-${index}`}
                role="option"
                aria-selected={index === active}
                // 按下时阻止输入框失焦，否则列表会先因失焦收起，点击落空
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => search(item)}
                className={cn(
                  'flex cursor-pointer items-center gap-3 px-4 py-1.5 text-sm text-[var(--color-card-foreground)] hover:bg-[var(--color-muted)]',
                  index === active && 'bg-[var(--color-muted)]',
                )}
              >
                <Search className="size-3.5 shrink-0 text-[var(--color-muted-foreground)]" />
                <span className="truncate">
                  {typed}
                  <span className="font-semibold">{completion}</span>
                </span>
              </li>
            )
          })}
        </ul>
      ) : null}
    </div>
  )
}
