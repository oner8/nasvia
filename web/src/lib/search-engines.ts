/**
 * 搜索引擎配置与地址拼装（纯逻辑，便于单测）。
 *
 * 该搜索框用于检索互联网，不涉及 NASVIA 自身数据；
 * 支持多引擎并记住用户选择（localStorage）。
 */

export interface SearchEngine {
  /** 稳定标识，用于持久化。 */
  id: string
  /** 展示名。 */
  name: string
  /** 查询地址模板，`{q}` 会被替换为 URL 编码后的关键词。 */
  template: string
}

export const SEARCH_ENGINES: readonly SearchEngine[] = [
  { id: 'google', name: 'Google', template: 'https://www.google.com/search?q={q}' },
  { id: 'bing', name: 'Bing', template: 'https://www.bing.com/search?q={q}' },
  { id: 'baidu', name: '百度', template: 'https://www.baidu.com/s?wd={q}' },
] as const

export const DEFAULT_ENGINE_ID = 'google'

/** localStorage 键名（与主题的 nasvia-theme 风格保持一致）。 */
export const ENGINE_STORAGE_KEY = 'nasvia-search-engine'

/** 按 id 取引擎，未知或缺失时回退默认引擎。 */
export function getEngine(id: string | null | undefined): SearchEngine {
  const found = SEARCH_ENGINES.find((engine) => engine.id === id)
  return found ?? SEARCH_ENGINES.find((engine) => engine.id === DEFAULT_ENGINE_ID)!
}

/** 从存储值解析出有效引擎 id（容错：非法值回退默认）。 */
export function normalizeEngineId(id: string | null | undefined): string {
  return SEARCH_ENGINES.some((engine) => engine.id === id) ? (id as string) : DEFAULT_ENGINE_ID
}

/**
 * 从任意 Storage 形状读取已保存的引擎 id。
 * 传入 undefined（例如 SSR / 无 localStorage）时返回默认值。
 */
export function readStoredEngineId(storage: Pick<Storage, 'getItem'> | undefined): string {
  if (!storage) return DEFAULT_ENGINE_ID
  try {
    return normalizeEngineId(storage.getItem(ENGINE_STORAGE_KEY))
  } catch {
    return DEFAULT_ENGINE_ID
  }
}

/**
 * 拼装搜索地址：关键词去首尾空白并做 URL 编码。
 * 关键词为空时返回 null（调用方据此不跳转）。
 */
export function buildSearchUrl(query: string, engineId?: string | null): string | null {
  const keyword = (query ?? '').trim()
  if (!keyword) return null
  const engine = getEngine(normalizeEngineId(engineId))
  return engine.template.replace('{q}', encodeURIComponent(keyword))
}
