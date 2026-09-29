/** 前后端共享的纯类型定义（无运行时代码，可直接被 node --test 加载）。 */

import type { NavPosition, NavStyle, NavVisible } from './nav'

export type Visibility = 'public' | 'private' | 'inherit'
export type EffectiveVisibility = 'public' | 'private'
export type AuthMode = 'public' | 'private'

export interface Site {
  id: number
  name: string
  description: string
  url: string
  lan_url: string
  icon: string
  icon_state: 'pending' | 'ready' | 'failed' | string
  /** 图标来源三档：hdicons / placeholder 自带底，favicon 需要前端套一层灰底。 */
  /** 图标来源档：hdicons / nasicon / placeholder 自带底不套框，favicon 需要前端套一层灰底。 */
  icon_source: 'hdicons' | 'nasicon' | 'favicon' | 'placeholder' | string
  /** 手动指定的 HD-Icons 条目名（仅后台接口返回，空 = 自动匹配）。 */
  icon_name?: string
  /** 服务端当前的自动匹配结果（仅后台接口返回，供后台提示用）。 */
  icon_auto?: string
  category_id: number | null
  sort: number
  pinned: boolean
  visibility: Visibility
  effective_visibility: EffectiveVisibility
  tags: string
}

export interface CategoryView {
  id: number
  name: string
  sort: number
  visibility: 'public' | 'private'
  site_count: number
  /** 后台手选的导航图标键；留空 = 按分类名自动匹配（见 lib/category-icon.ts）。 */
  icon: string
}

export interface FaviconSources {
  /** HD-Icons 圆角图标（按名称匹配，优先级最高）。 */
  hdicons: boolean
  /** nasicon.top 图标（中文名/域名匹配，HD-Icons 之后兜底）。 */
  nasicon: boolean
  site: boolean
  duckduckgo: boolean
  google: boolean
}

export interface AppConfig {
  site_title: string
  auth_mode: AuthMode
  lan_cidrs: string[]
  lan_cidrs_raw: string
  favicon_sources: FaviconSources
  authenticated: boolean
  password_configured: boolean
  version: string
  /** 分类导航位置（后台统一设置）：top / left / right。 */
  nav_position: NavPosition
  /** 分类导航标签样式：text（显示名称）/ icon（仅图标）。 */
  nav_style: NavStyle
  /** 分类导航是否显示：show / hide（后台统一设置）。 */
  nav_visible: NavVisible
  /** 服务端按访客真实 IP 的内外网判定（经反代访问同一域名时只能靠它区分在家 / 在外）。 */
  network?: ServerNetwork
}

/** 服务端内外网判定：reason 为 lan_rule（命中内网网段）/ home_egress（来自家庭公网出口）/ 空（外网）。 */
export interface ServerNetwork {
  mode: 'lan' | 'wan'
  client_ip: string
  reason: 'lan_rule' | 'home_egress' | ''
}

export interface AdminSettings {
  auth_mode: AuthMode
  lan_cidrs_raw: string
  /** 家庭公网出口原文（auto / IP / CIDR / DDNS 域名，每行一条）。 */
  home_egress_raw: string
  /** 其中 DDNS 域名与 auto 当前解析到的地址。 */
  home_egress_status: { resolved: string[]; updated_at: string; error: string }
  site_title: string
  /** HD-Icons 图标源（按站点名匹配圆角图标，优先于其它来源）。 */
  favicon_sources: FaviconSources
  /** 生效的 HD-Icons 镜像前缀（多行文本，按顺序尝试）。 */
  hdicons_mirrors_raw: string
  /** 生效的 nasicon.top 站点地址（默认 https://nasicon.top，可用 NASVIA_NASICON_BASE 覆盖）。 */
  nasicon_base_raw: string
  password_configured: boolean
  /** 分类导航位置（top / left / right），前台所有访客统一。 */
  nav_position: NavPosition
  /** 分类导航标签样式（text 显示名称 / icon 仅图标）。 */
  nav_style: NavStyle
  /** 分类导航是否显示（show / hide）。 */
  nav_visible: NavVisible
}

export interface SiteListResponse {
  items: Site[]
  total: number
  authenticated: boolean
}

export interface CategoryListResponse {
  items: CategoryView[]
  total: number
  authenticated: boolean
}

export interface ApiErrorBody {
  code: string
  message: string
}
