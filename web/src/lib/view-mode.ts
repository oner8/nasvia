/**
 * 首页视图模式：磁贴卡片（tile）/ 应用图标网格（app）。
 *
 * 纯前端个人偏好：**默认应用图标**（未设置过的访客首屏就是应用图标视图），
 * 选择存在浏览器 localStorage，已显式选过磁贴的浏览器保持自己的选择；
 * 非法值一律回退默认。这里只放不依赖 React 的纯逻辑，便于 node:test 直接覆盖。
 */

export type ViewMode = 'tile' | 'app'

/**
 * 菜单里的顺序即此数组顺序：应用图标排在磁贴前面
 * （默认选中项也是第一个，与「默认应用图标」保持一致）。
 */
export const VIEW_MODES: readonly ViewMode[] = ['app', 'tile']

export const DEFAULT_VIEW_MODE: ViewMode = 'app'

/** localStorage 键名（与 nasvia-theme / nasvia-search-engine 风格保持一致）。 */
export const VIEW_STORAGE_KEY = 'nasvia-view-mode'

/** 视图展示名。 */
export const VIEW_NAME: Record<ViewMode, string> = {
  tile: '磁贴',
  app: '应用图标',
}

/**
 * 网格容器类名。必须是完整字符串字面量（不做拼接），否则 Tailwind 扫描不到这些类。
 * 两个视图的列数都在这里：磁贴 手机 2 / 平板 3 / 桌面 4；应用图标 手机 4 / 平板 5、6 / 桌面 10。
 */
export const GRID_CLASS: Record<ViewMode, string> = {
  tile: 'grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4',
  app: 'grid grid-cols-4 gap-x-2 gap-y-5 sm:grid-cols-5 md:grid-cols-6 lg:grid-cols-10',
}

/** 把任意存储值规整为合法视图模式。 */
export function normalizeViewMode(value: string | null | undefined): ViewMode {
  return value === 'tile' || value === 'app' ? value : DEFAULT_VIEW_MODE
}

/**
 * 从任意 Storage 形状读取已保存的视图模式。
 * 传入 undefined（无 localStorage / SSR）或读取抛错时返回默认值。
 */
export function readStoredViewMode(storage: Pick<Storage, 'getItem'> | undefined): ViewMode {
  if (!storage) return DEFAULT_VIEW_MODE
  try {
    return normalizeViewMode(storage.getItem(VIEW_STORAGE_KEY))
  } catch {
    return DEFAULT_VIEW_MODE
  }
}

/** 写入视图模式；存储不可用（隐私模式、配额满）时静默忽略。 */
export function writeStoredViewMode(
  storage: Pick<Storage, 'setItem'> | undefined,
  mode: ViewMode,
): void {
  if (!storage) return
  try {
    storage.setItem(VIEW_STORAGE_KEY, normalizeViewMode(mode))
  } catch {
    /* 忽略写入失败：偏好丢失不影响页面可用 */
  }
}

/** 在两种视图之间切换。 */
export function nextViewMode(mode: ViewMode): ViewMode {
  return mode === 'app' ? 'tile' : 'app'
}
