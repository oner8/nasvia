/**
 * 分类导航（后台统一设置：`nav_position` / `nav_style`，见 model.go）。
 *
 * 这里只放纯逻辑与类名常量，便于 `node:test` 直接覆盖。
 *
 * 三条硬约束：
 * 1. 类名必须是**完整字面量**（不做拼装），否则 Tailwind 扫描不到这些类。
 * 2. 侧栏与顶部（已取消的旧形态）共用同一套 `NAV_BAR_CLASS` / `NAV_ITEM_CLASS` / `NAV_INDICATOR_CLASS`
 *    —— 单一来源，只有方向与定位不同。
 * 3. 侧栏与内容**同处一行**（不是固定在窗口边缘），所以不论窗口多宽，侧栏都紧贴内容；
 *    手机与平板（< lg = 1024px）整块隐藏。
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

/**
 * 行容器：侧栏与内容在同一行、整体居中。
 * 宽度 = 侧栏 + 间距 + 内容(60rem) + 内边距(4rem)：仅图标 68.5rem；显示名称 75rem。
 * 这样窗口再宽，侧栏也只紧贴内容，而不会漂到窗口边缘。
 */
export const NAV_ROW_BASE_CLASS = 'mx-auto flex w-full justify-center gap-4 px-4 sm:px-6 lg:px-8'
export const NAV_ROW_MAX_ICON_CLASS = 'max-w-[68.5rem]'
export const NAV_ROW_MAX_TEXT_CLASS = 'max-w-[75rem]'
/** 侧栏在右时把整行反向（DOM 顺序仍是 [侧栏, 内容]）。 */
export const NAV_ROW_RIGHT_CLASS = 'flex-row-reverse'

/**
 * 侧栏所在列：与视口等高并吸顶，内部垂直居中
 * → 侧栏始终停在视口的竖直中线上（滚动时也是）。
 *
 * **手机与平板（< lg = 1024px）整块隐藏**（`hidden … lg:flex`）：小屏上分类导航会挤占本就不多的宽度，
 * 此时内容占满整行；桌面（≥1024px）才显示导航。
 */
export const NAV_RAIL_COLUMN_CLASS = 'sticky top-0 hidden h-dvh shrink-0 items-center lg:flex'

/**
 * 侧栏宽度：
 * - 仅图标：固定 3.5rem（63px）；
 * - 显示名称：**按内容自适应**（`w-max`）——宽度只取最长分类名所需，不再固定 180px；
 *   极长分类名用 `max-w-[11rem]`(198px) 兜底，标签本身会截断成省略号（悬停有完整名称）。
 */
export const NAV_RAIL_WIDTH_ICON_CLASS = 'w-14'
export const NAV_RAIL_WIDTH_TEXT_CLASS = 'w-max max-w-[11rem]'

/** 侧栏标签栏自身：铺满所在列；分类很多时栏内滚动。 */
export const NAV_RAIL_CLASS = 'w-full max-h-[85dvh] overflow-y-auto'

/** 内容列：宽度与「居中内容」一致（60rem），小屏时占满整行。 */
export const NAV_CONTENT_CLASS = 'flex min-h-dvh min-w-0 max-w-[60rem] flex-1 flex-col pt-8'

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

/** 隐藏分类导航时的行宽：内容 60rem + 内边距 4rem（内容仍然居中）。 */
export const NAV_ROW_HIDDEN_CLASS = 'max-w-[64rem]'

/** 行容器类名（侧栏 + 内容一起居中整行；隐藏导航时不预留侧栏宽度）。 */
export function navRowClass(
  position: NavPosition,
  style: NavStyle,
  visible: NavVisible = DEFAULT_NAV_VISIBLE,
): string {
  if (visible === 'hide') return join(NAV_ROW_BASE_CLASS, NAV_ROW_HIDDEN_CLASS)
  return join(
    NAV_ROW_BASE_CLASS,
    style === 'icon' ? NAV_ROW_MAX_ICON_CLASS : NAV_ROW_MAX_TEXT_CLASS,
    position === 'right' && NAV_ROW_RIGHT_CLASS,
  )
}

/** 侧栏所在列类名（宽度由样式决定）。 */
export function navRailColumnClass(style: NavStyle): string {
  return join(NAV_RAIL_COLUMN_CLASS, style === 'icon' ? NAV_RAIL_WIDTH_ICON_CLASS : NAV_RAIL_WIDTH_TEXT_CLASS)
}

/**
 * 侧栏标签栏类名：共享药丸样式 + 竖向（display 由 navBarClass 提供）。
 * 仅图标模式下让图标格在列内居中。
 */
export function navRailClass(style: NavStyle): string {
  return join(NAV_RAIL_CLASS, NAV_BAR_CLASS, 'flex-col gap-1', style === 'icon' && 'items-center')
}
