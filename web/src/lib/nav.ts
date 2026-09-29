/**
 * 分类导航（后台统一设置：`nav_position` / `nav_style`，见 model.go）。
 *
 * 这里只放纯逻辑与类名常量，便于 `node:test` 直接覆盖。
 *
 * 三条硬约束：
 * 1. 类名必须是**完整字面量**（不做拼装），否则 Tailwind 扫描不到这些类。
 * 2. 侧栏与顶部（已取消的旧形态）共用同一套 `NAV_BAR_CLASS` / `NAV_ITEM_CLASS` / `NAV_INDICATOR_CLASS`
 *    —— 单一来源，只有方向与定位不同。
 * 3. 主内容固定在对称三列网格的中间列，侧栏使用两侧留白，不参与主内容宽度计算；
 *    小于 xl（1280px）时整块隐藏。
 */

export type NavPosition = 'left' | 'right'
export type NavStyle = 'text' | 'icon'
export type NavVisible = 'show' | 'hide'

/** 位置只剩左右两侧（顶部导航已取消）。 */
export const NAV_POSITIONS: readonly NavPosition[] = ['left', 'right']
export const NAV_STYLES: readonly NavStyle[] = ['text', 'icon']

export const DEFAULT_NAV_POSITION: NavPosition = 'left'
export const DEFAULT_NAV_STYLE: NavStyle = 'icon'

export const NAV_POSITION_NAME: Record<NavPosition, string> = {
  left: '左侧',
  right: '右侧',
}

export const NAV_STYLE_NAME: Record<NavStyle, string> = {
  text: '显示名称',
  icon: '仅图标',
}

/**
 * 归一化导航位置：只有 `right` 保留，其余一律 `left`。
 * 历史库里的 `top`（已取消的顶部导航）与空值、非法值都会落到 `left` —— 不改写数据，只在读取时归一。
 */
export function normalizeNavPosition(value: string | null | undefined): NavPosition {
  return value === 'right' ? 'right' : DEFAULT_NAV_POSITION
}

/**
 * 归一化标签样式：默认是 **icon（仅图标）**，只有显式 `text` 才用文字。
 * （空值 / 非法值 / 历史数据都按默认走。）
 */
export function normalizeNavStyle(value: string | null | undefined): NavStyle {
  return value === 'text' ? 'text' : DEFAULT_NAV_STYLE
}

/**
 * 文字模式下的导航标签：**最多两个汉字**（其它字符按「1 个 = 半个汉字」计，最多 4 个），
 * 超出的部分截掉；完整名称走悬停提示与无障碍名，所以信息不会丢。
 *
 * 例：媒体中心 → 媒体；下载与整理 → 下载；网络与安全 → 网络；Jellyfin → Jell。
 */
const WIDE_CHAR = /[\u1100-\u115f\u2e80-\u303e\u3041-\u33ff\u3400-\u4dbf\u4e00-\u9fff\ua000-\ua4cf\uac00-\ud7a3\uf900-\ufaff\ufe30-\ufe6f\uff00-\uff60\uffe0-\uffe6]/

export function navLabel(name: string, maxUnits = 4): string {
  let used = 0
  let label = ''
  for (const char of Array.from((name ?? '').trim())) {
    const weight = WIDE_CHAR.test(char) ? 2 : 1
    if (used + weight > maxUnits) break
    label += char
    used += weight
  }
  return label
}

/** 分类导航是否显示（后台统一设置，所有访客一致）。 */
export const NAV_VISIBLES: readonly NavVisible[] = ['show', 'hide']
export const DEFAULT_NAV_VISIBLE: NavVisible = 'show'
export const NAV_VISIBLE_NAME: Record<NavVisible, string> = {
  show: '显示',
  hide: '隐藏',
}

/** 归一化「是否显示分类导航」：只有 hide 保留，其余（含空值、非法值）回落到 show。 */
export function normalizeNavVisible(value: string | null | undefined): NavVisible {
  return value === 'hide' ? 'hide' : 'show'
}

/* ---------------------------------------------------------------- 共享样式 */

/** 药丸容器：单一来源（同一份圆角、边框、底色与内边距）。 */
export const NAV_BAR_CLASS = 'rounded-2xl border border-[var(--color-border)] bg-[var(--color-muted)]/40 p-1'

/** 单个标签：竖向导航用。 */
export const NAV_ITEM_CLASS =
  'relative z-10 cursor-pointer rounded-lg px-3 py-1.5 text-xs font-medium outline-none transition-colors duration-200'

/** 选中态：反色药丸（浅色=近黑，深色=近白）。 */
export const NAV_ITEM_ACTIVE_CLASS = 'text-[var(--color-background)]'

/** 未选中态：次级文字 + 悬停反馈。 */
export const NAV_ITEM_IDLE_CLASS =
  'text-[var(--color-muted-foreground)] hover:bg-[var(--color-accent)]/60 hover:text-[var(--color-foreground)]'

/** 仅图标模式：把药丸变成方形图标格。 */
export const NAV_ITEM_ICON_CLASS = 'flex size-9 items-center justify-center p-0'

/** 滑动指示器（motion layoutId）。 */
export const NAV_INDICATOR_CLASS = 'absolute inset-0 -z-10 rounded-lg bg-[var(--color-foreground)]'

/** 指示器 layoutId（界面上只有一份竖向导航，用固定值即可）。 */
export const NAV_INDICATOR_ID = 'category-tab-indicator'

/* ---------------------------------------------------------------- 侧栏与内容 */

/** 对称三列：两侧均分留白，中间主内容最大 60rem，侧栏不会改变中间列的位置或宽度。 */
export const NAV_GRID_CLASS =
  'grid min-h-dvh w-full grid-cols-[minmax(0,1fr)_min(60rem,100%)_minmax(0,1fr)] px-4 sm:px-6 lg:px-8'

/**
 * 侧栏所在列：与视口等高并吸顶，内部垂直居中
 * → 侧栏始终停在视口的竖直中线上（滚动时也是）。
 *
 * **小于 xl = 1280px 时整块隐藏**（`hidden … xl:flex`），避免侧栏覆盖主内容。
 */
export const NAV_RAIL_COLUMN_CLASS = 'sticky top-0 row-start-1 hidden h-dvh min-w-0 self-start items-center xl:flex'
export const NAV_RAIL_LEFT_CLASS = 'col-start-1 justify-end pr-4'
export const NAV_RAIL_RIGHT_CLASS = 'col-start-3 justify-start pl-4'

/**
 * 侧栏宽度：
 * - 仅图标：固定 3.5rem（63px）；
 * - 显示名称：**按内容自适应**（`w-max`）——宽度只取最长分类名所需，不再固定 180px；
 *   极长分类名用 `max-w-[11rem]`(198px) 兜底，标签本身会截断成省略号（悬停有完整名称）。
 */
export const NAV_RAIL_WIDTH_ICON_CLASS = 'w-14'
export const NAV_RAIL_WIDTH_TEXT_CLASS = 'w-max max-w-[11rem]'

/** 侧栏标签栏自身：分类很多时栏内滚动。 */
export const NAV_RAIL_CLASS = 'max-h-[85dvh] overflow-y-auto'

/** 内容固定在中间列；左右侧栏切换或隐藏时，位置和宽度不变。 */
export const NAV_CONTENT_CLASS = 'col-start-2 row-start-1 flex min-h-dvh min-w-0 w-full flex-col pt-8'

function join(...parts: Array<string | false | undefined>): string {
  return parts.filter((part) => Boolean(part)).join(' ')
}

/**
 * 药丸容器类名：共享药丸样式 + 竖向。
 * 竖向必须真的 `display:flex`，否则 `items-center` / `flex-col` 都不生效 ——
 * 图标会贴着左侧、而不是在栏内水平居中（已踩过：偏左 5.7px）。
 */
export function navBarClass(): string {
  return join(NAV_BAR_CLASS, 'flex flex-col gap-1')
}

/** 单个标签类名（仅图标模式换成方形图标格）。 */
export function navItemClass(style: NavStyle, active: boolean): string {
  return join(
    NAV_ITEM_CLASS,
    style === 'icon' && NAV_ITEM_ICON_CLASS,
    active ? NAV_ITEM_ACTIVE_CLASS : NAV_ITEM_IDLE_CLASS,
  )
}

/** 侧栏放入主内容左侧或右侧的留白，并从留白中留出 1rem 间距。 */
export function navRailColumnClass(position: NavPosition): string {
  return join(NAV_RAIL_COLUMN_CLASS, position === 'right' ? NAV_RAIL_RIGHT_CLASS : NAV_RAIL_LEFT_CLASS)
}

/**
 * 侧栏标签栏类名：共享药丸样式 + 竖向（display 由 navBarClass 提供）。
 * 仅图标模式下让图标格在列内居中。
 */
export function navRailClass(style: NavStyle): string {
  return join(
    NAV_RAIL_CLASS,
    NAV_BAR_CLASS,
    'flex-col gap-1',
    style === 'icon' ? join(NAV_RAIL_WIDTH_ICON_CLASS, 'items-center') : NAV_RAIL_WIDTH_TEXT_CLASS,
  )
}
