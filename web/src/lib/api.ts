import axios from 'axios'

import type { NavPosition, NavStyle, NavVisible } from './nav'
import type {
  AdminSettings,
  ApiErrorBody,
  AppConfig,
  CategoryListResponse,
  CategoryView,
  FaviconSources,
  Site,
  SiteListResponse,
} from './types'

/** 统一的 API 客户端（同源部署，Cookie 会话）。 */
export const http = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 30000,
})

export function apiErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data as ApiErrorBody | undefined
    if (body?.message) return body.message
    if (error.code === 'ECONNABORTED') return '请求超时，请稍后重试'
    if (!error.response) return '无法连接到 NASVIA 服务'
    return `请求失败（HTTP ${error.response.status}）`
  }
  if (error instanceof Error) return error.message
  return '未知错误'
}

export function apiErrorCode(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data as ApiErrorBody | undefined
    return body?.code ?? ''
  }
  return ''
}

export interface SitePayload {
  name?: string
  description?: string
  url?: string
  lan_url?: string
  category_id?: number | null
  clear_category?: boolean
  visibility?: Site['visibility']
  tags?: string
  pinned?: boolean
  /** 手动指定的 HD-Icons 条目名（留空则用 clear_icon_name 回到自动匹配）。 */
  icon_name?: string
  clear_icon_name?: boolean
}

export interface CategoryPayload {
  name?: string
  visibility?: CategoryView['visibility']
  /** 导航图标键（空串 = 清除，改回按分类名自动匹配）。 */
  icon?: string
}

export interface SettingsPayload {
  auth_mode?: 'public' | 'private'
  lan_cidrs?: string
  /** 家庭公网出口（auto / IP / CIDR / DDNS 域名）。 */
  home_egress?: string
  site_title?: string
  favicon_sources?: FaviconSources
  /** HD-Icons 镜像前缀（每行一个）。 */
  hdicons_mirrors?: string
  /** 分类导航位置：top / left / right（空串 = 回到默认 top）。 */
  nav_position?: NavPosition
  /** 分类导航标签样式：text / icon（空串 = 回到默认 text）。 */
  nav_style?: NavStyle
  /** 分类导航是否显示：show / hide（空串 = 回到默认 show）。 */
  nav_visible?: NavVisible
}

/** 单个镜像前缀的连通性测试结果。 */
export interface HDIconsMirrorResult {
  mirror: string
  ok: boolean
  /** 耗时（毫秒）。 */
  ms: number
  /** 索引里的图标数量（失败时为 0）。 */
  count: number
  error?: string
}

export interface HDIconsTestResult {
  mirrors: HDIconsMirrorResult[]
  mirrors_raw: string
  cache?: {
    count: number
    mirror: string
    fetched_at: string
    fresh: boolean
  }
}

/** 图标搜索响应（后台图标选择器用）。 */
export interface HDIconsSearchResult {
  items: string[]
  count: number
  /** 本地是否已有索引缓存（false 表示还没拉过索引）。 */
  cached: boolean
  total?: number
}

export type BatchAction = 'delete' | 'visibility' | 'pinned' | 'category'

export const api = {
  async config(): Promise<AppConfig> {
    return (await http.get<AppConfig>('/config')).data
  },
  async session(): Promise<{ authenticated: boolean }> {
    return (await http.get<{ authenticated: boolean }>('/auth/session')).data
  },
  async login(password: string): Promise<{ authenticated: boolean }> {
    return (await http.post<{ authenticated: boolean }>('/auth/login', { password })).data
  },
  async logout(): Promise<void> {
    await http.post('/auth/logout')
  },
  async sites(): Promise<SiteListResponse> {
    return (await http.get<SiteListResponse>('/sites')).data
  },
  async categories(): Promise<CategoryListResponse> {
    return (await http.get<CategoryListResponse>('/categories')).data
  },
  async adminSites(): Promise<SiteListResponse> {
    return (await http.get<SiteListResponse>('/admin/sites')).data
  },
  async createSite(payload: SitePayload): Promise<Site> {
    const res = await http.post<{ site: Site }>('/admin/sites', payload)
    return res.data.site
  },
  async updateSite(id: number, payload: SitePayload): Promise<Site> {
    const res = await http.put<{ site: Site }>(`/admin/sites/${id}`, payload)
    return res.data.site
  },
  async deleteSite(id: number): Promise<void> {
    await http.delete(`/admin/sites/${id}`)
  },
  async batchSites(payload: {
    ids: number[]
    action: BatchAction
    visibility?: Site['visibility']
    pinned?: boolean
    category_id?: number | null
    clear_category?: boolean
  }): Promise<{ affected: number }> {
    return (await http.post<{ affected: number }>('/admin/sites/batch', payload)).data
  },
  async purgeSites(): Promise<{ deleted: number }> {
    return (await http.post<{ deleted: number }>('/admin/sites/purge')).data
  },
  async purgeCategories(): Promise<{ deleted: number }> {
    return (await http.post<{ deleted: number }>('/admin/categories/purge')).data
  },
  async purgeAll(): Promise<void> {
    await http.post('/admin/purge')
  },
  async moveSite(id: number, direction: 'up' | 'down'): Promise<void> {
    await http.post(`/admin/sites/${id}/move`, { direction })
  },
  /** 拖动排序：按给定顺序整体重排全部站点（ids 必须不重不漏）。 */
  async reorderSites(ids: number[]): Promise<void> {
    await http.post('/admin/sites/reorder', { ids })
  },
  async refetchIcon(id: number): Promise<void> {
    await http.post(`/admin/sites/${id}/refetch-icon`)
  },
  async adminCategories(): Promise<CategoryListResponse> {
    return (await http.get<CategoryListResponse>('/admin/categories')).data
  },
  async createCategory(payload: CategoryPayload): Promise<CategoryView> {
    const res = await http.post<{ category: CategoryView }>('/admin/categories', payload)
    return res.data.category
  },
  async updateCategory(id: number, payload: CategoryPayload): Promise<CategoryView> {
    const res = await http.put<{ category: CategoryView }>(`/admin/categories/${id}`, payload)
    return res.data.category
  },
  async deleteCategory(id: number): Promise<void> {
    await http.delete(`/admin/categories/${id}`)
  },
  async moveCategory(id: number, direction: 'up' | 'down'): Promise<void> {
    await http.post(`/admin/categories/${id}/move`, { direction })
  },
  async settings(): Promise<AdminSettings> {
    return (await http.get<AdminSettings>('/admin/settings')).data
  },
  async updateSettings(payload: SettingsPayload): Promise<AdminSettings> {
    return (await http.put<AdminSettings>('/admin/settings', payload)).data
  },
  /** 逐个测试 HD-Icons 镜像前缀的连通性（后台「测试连通性」按钮）。 */
  async testHDIcons(): Promise<HDIconsTestResult> {
    return (await http.post<HDIconsTestResult>('/admin/hdicons/test')).data
  },
  /** 在本地索引缓存里搜图标条目名（支持中文关键词，如「网盘」），后台图标选择器用。 */
  async searchHDIcons(query: string): Promise<HDIconsSearchResult> {
    return (await http.get<HDIconsSearchResult>('/admin/hdicons/search', { params: { q: query, limit: 20 } })).data
  },
  /** 图标预览地址：走服务端下载并校验，前端不直接接触镜像地址。 */
  iconPreviewURL(name: string): string {
    return `/api/admin/hdicons/icon?name=${encodeURIComponent(name)}`
  },
  /** 联网搜索联想词（服务端代取；当前引擎取不到时会用其它引擎的结果）。 */
  async suggest(engine: string, q: string, signal?: AbortSignal): Promise<string[]> {
    const { data } = await http.get<{ items: string[] }>('/suggest', { params: { engine, q }, signal })
    return data.items ?? []
  },
  async changePassword(password: string): Promise<void> {
    await http.post('/admin/password', { password })
  },
}
