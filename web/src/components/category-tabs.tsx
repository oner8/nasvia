import { motion } from 'motion/react'

import { tabIconKey } from '../lib/category-icon'
import {
  NAV_INDICATOR_CLASS,
  NAV_INDICATOR_ID,
  navBarClass,
  navItemClass,
  navLabel,
  type NavStyle,
} from '../lib/nav'
import { cn } from '../lib/utils'
import { CategoryIcon } from './category-icon'
import { Tooltip } from './ui/tooltip'

export interface CategoryTab {
  id: number | 'all' | 'pinned'
  name: string
  count: number
  /** 后台手选的图标键；留空 = 按分类名自动匹配。 */
  icon?: string
}

export interface CategoryTabsProps {
  tabs: CategoryTab[]
  active: CategoryTab['id']
  onChange: (id: CategoryTab['id']) => void
  /** 标签样式：text 显示名称（默认）；icon 只显示图标 + 悬停提示。 */
  style?: NavStyle
  /** 侧栏所在侧：决定悬停提示往哪边弹（左栏往右、右栏往左）。 */
  side?: 'left' | 'right'
  /** 追加类名（侧栏把定位与宽度类从这里传进来）。 */
  className?: string
}

/**
 * 分类导航标签（竖向侧栏）：
 * - 药丸样式来自 `lib/nav.ts` 的共享常量（单一来源）；
 * - `style='icon'` 时只显示图标，分类名与数量走悬停提示（并保留 sr-only 文本与 aria-label）。
 */
export function CategoryTabs({
  tabs,
  active,
  onChange,
  style = 'text',
  side = 'left',
  className,
}: CategoryTabsProps) {
  const iconOnly = style === 'icon'
  const tooltipSide = side === 'right' ? 'left' : 'right'

  return (
    <nav
      role="tablist"
      aria-label="服务分类"
      aria-orientation="vertical"
      className={cn(navBarClass(), 'w-full', className)}
    >
      {tabs.map((tab) => {
        const isActive = tab.id === active
        const button = (
          <button
            key={String(tab.id)}
            type="button"
            role="tab"
            aria-selected={isActive}
            aria-label={iconOnly ? `${tab.name}（${tab.count}）` : tab.name}
            onClick={() => onChange(tab.id)}
            className={cn(navItemClass(style, isActive), !iconOnly && 'w-full text-left')}
          >
            {isActive ? (
              <motion.span
                layoutId={NAV_INDICATOR_ID}
                className={NAV_INDICATOR_CLASS}
                transition={{ type: 'spring', stiffness: 500, damping: 40 }}
              />
            ) : null}
            {iconOnly ? (
              <>
                <CategoryIcon
                  name={tab.name}
                  icon={tab.icon}
                  iconKey={tabIconKey(tab.id, tab.name, tab.icon)}
                  className="size-[18px]"
                />
                <span className="sr-only">{tab.name}</span>
              </>
            ) : (
              // 文字模式：最多两个汉字（其余截断），完整名称在 title / aria-label 里
              <span className="relative block truncate" title={tab.name}>
                {navLabel(tab.name)}
              </span>
            )}
          </button>
        )

        return iconOnly ? (
          <Tooltip key={String(tab.id)} content={`${tab.name} · ${tab.count}`} side={tooltipSide}>
            {button}
          </Tooltip>
        ) : (
          button
        )
      })}
    </nav>
  )
}
