import { Menu as BaseMenu } from '@base-ui/react/menu'
import { Grid3x3, LayoutGrid, Monitor, Moon, Palette, Sun } from 'lucide-react'
import { ThemeProvider as NextThemesProvider, useTheme } from 'next-themes'
import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'

import { normalizeViewMode, VIEW_MODES, VIEW_NAME, type ViewMode } from '../lib/view-mode'
import { cn } from '../lib/utils'
import { Button } from './ui/button'
import { Tooltip } from './ui/tooltip'

/** 深色 / 浅色 / 跟随系统 三态主题（storageKey 与 index.html 中防闪白脚本保持一致）。 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      storageKey="nasvia-theme"
      disableTransitionOnChange
    >
      {children}
    </NextThemesProvider>
  )
}

/** 菜单选项顺序：跟随系统 → 深色 → 浅色。 */
const MODES = ['system', 'dark', 'light'] as const
type Mode = (typeof MODES)[number]

const LABELS: Record<Mode, string> = {
  system: '跟随系统',
  dark: '深色',
  light: '浅色',
}

const ICONS: Record<Mode, typeof Monitor> = {
  system: Monitor,
  dark: Moon,
  light: Sun,
}

/** 视图选项的图标：磁贴看成一张张卡片，应用图标看成主屏九宫格。 */
const VIEW_ICONS: Record<ViewMode, typeof LayoutGrid> = {
  tile: LayoutGrid,
  app: Grid3x3,
}

/** 两行选项共用同一套外观。 */
const ITEM_CLASS = cn(
  'flex size-9 cursor-pointer items-center justify-center rounded-lg outline-none',
  'text-[var(--color-muted-foreground)] transition-colors',
  'data-[highlighted]:bg-[var(--color-muted)] data-[highlighted]:text-[var(--color-foreground)]',
  'data-[checked]:bg-[var(--color-accent)] data-[checked]:text-[var(--color-primary)]',
)
/**
 * 两行共用同一套列网格（标签列 2rem + 三个选项列 2.25rem），
 * 这样「主题」的 3 个选项与「视图」的 2 个选项严格同列，
 * 且无论哪行有几个选项，两行都等宽、弹出菜单是齐整的矩形。
 */
const ROW_CLASS = 'grid grid-cols-[2rem_2.25rem_2.25rem_2.25rem] items-center gap-1'
const GROUP_LABEL_CLASS = 'pl-1 text-xs text-[var(--color-muted-foreground)]'

export interface ThemeToggleProps {
  className?: string
  /** 当前视图模式（磁贴 / 应用图标），与主题同在一个菜单里切换。 */
  view: ViewMode
  onViewChange: (mode: ViewMode) => void
}

/**
 * 外观菜单：点击调色板图标弹出两行选项——「主题」跟随系统 / 深色 / 浅色，
 * 「视图」磁贴 / 应用图标（原来单独占一个工具栏按钮，现在收进这个菜单）。
 */
export function ThemeToggle({ className, view, onViewChange }: ThemeToggleProps) {
  const { theme, setTheme } = useTheme()
  const [mounted, setMounted] = useState(false)

  useEffect(() => setMounted(true), [])

  const current: Mode =
    mounted && (theme === 'light' || theme === 'dark' || theme === 'system') ? theme : 'system'
  const label = `主题与视图：${LABELS[current]} · ${VIEW_NAME[view]}`

  return (
    <BaseMenu.Root modal={false}>
      <Tooltip content={label} side="bottom">
        <BaseMenu.Trigger
          render={<Button variant="ghost" size="icon" className={className} aria-label={label} />}
        >
          <Palette className="size-4" />
        </BaseMenu.Trigger>
      </Tooltip>
      <BaseMenu.Portal>
        <BaseMenu.Positioner side="bottom" align="center" sideOffset={8} className="z-[70] outline-none">
          <BaseMenu.Popup
            className={cn(
              'relative flex flex-col gap-1 rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] p-1 shadow-soft outline-none',
              'transition data-[starting-style]:scale-95 data-[starting-style]:opacity-0',
              'data-[ending-style]:scale-95 data-[ending-style]:opacity-0',
            )}
          >
            {/* 指向主题按钮的小箭头（居中弹窗时正好落在图标正下方） */}
            <span
              aria-hidden
              className={cn(
                'absolute left-1/2 size-3 -translate-x-1/2 -top-1 rotate-45 rounded-[1px]',
                'border-l border-t border-[var(--color-border)] bg-[var(--color-card)]',
              )}
            />

            <BaseMenu.RadioGroup
              value={current}
              onValueChange={(value) => setTheme(String(value))}
              className={ROW_CLASS}
            >
              <BaseMenu.GroupLabel className={GROUP_LABEL_CLASS}>主题</BaseMenu.GroupLabel>
              {MODES.map((mode) => {
                const Icon = ICONS[mode]
                return (
                  <Tooltip key={mode} content={LABELS[mode]} side="bottom" delay={120}>
                    <BaseMenu.RadioItem
                      value={mode}
                      closeOnClick
                      aria-label={LABELS[mode]}
                      className={ITEM_CLASS}
                    >
                      <Icon className="size-4" />
                      <span className="sr-only">{LABELS[mode]}</span>
                    </BaseMenu.RadioItem>
                  </Tooltip>
                )
              })}
            </BaseMenu.RadioGroup>

            <BaseMenu.RadioGroup
              value={view}
              onValueChange={(value) => onViewChange(normalizeViewMode(String(value)))}
              className={cn(ROW_CLASS, 'border-t border-[var(--color-border)] pt-1')}
            >
              <BaseMenu.GroupLabel className={GROUP_LABEL_CLASS}>视图</BaseMenu.GroupLabel>
              {VIEW_MODES.map((mode) => {
                const Icon = VIEW_ICONS[mode]
                const name = `${VIEW_NAME[mode]}视图`
                return (
                  <Tooltip key={mode} content={name} side="bottom" delay={120}>
                    <BaseMenu.RadioItem
                      value={mode}
                      closeOnClick
                      aria-label={name}
                      className={ITEM_CLASS}
                    >
                      <Icon className="size-4" />
                      <span className="sr-only">{name}</span>
                    </BaseMenu.RadioItem>
                  </Tooltip>
                )
              })}
            </BaseMenu.RadioGroup>
          </BaseMenu.Popup>
        </BaseMenu.Positioner>
      </BaseMenu.Portal>
    </BaseMenu.Root>
  )
}
