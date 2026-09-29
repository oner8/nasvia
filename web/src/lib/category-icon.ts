/**
 * 分类图标：后台可手选（`category.icon`），留空则按分类名自动匹配。
 *
 * 纯字符串逻辑（不含 React / lucide），便于 `node:test` 直接覆盖；
 * 图标键 → 图标的渲染映射在 `components/category-icon.tsx`。
 */

export type CategoryIconKey =
  | 'layout-grid'
  | 'star'
  | 'film'
  | 'music'
  | 'image'
  | 'camera'
  | 'download'
  | 'house'
  | 'shield'
  | 'lock'
  | 'key'
  | 'server'
  | 'hard-drive'
  | 'cloud'
  | 'database-backup'
  | 'file-text'
  | 'activity'
  | 'terminal'
  | 'gamepad'
  | 'mail'
  | 'message'
  | 'book'
  | 'box'
  | 'layers'

/** 可选图标键（后台选择器的网格顺序）。 */
export const CATEGORY_ICON_KEYS: readonly CategoryIconKey[] = [
  'layout-grid',
  'star',
  'film',
  'music',
  'image',
  'camera',
  'download',
  'house',
  'shield',
  'lock',
  'key',
  'server',
  'hard-drive',
  'cloud',
  'database-backup',
  'file-text',
  'activity',
  'terminal',
  'gamepad',
  'mail',
  'message',
  'book',
  'box',
  'layers',
]

/** 未匹配到任何关键词时的兜底图标。 */
export const CATEGORY_ICON_FALLBACK: CategoryIconKey = 'layers'

/** 图标键的中文名（后台选择器与提示用）。 */
export const CATEGORY_ICON_NAME: Record<CategoryIconKey, string> = {
  'layout-grid': '网格',
  star: '星标',
  film: '影音',
  music: '音乐',
  image: '图片',
  camera: '摄像头',
  download: '下载',
  house: '家庭',
  shield: '网络与安全',
  lock: '私密',
  key: '密钥',
  server: '服务器',
  'hard-drive': '存储',
  cloud: '云盘',
  'database-backup': '备份',
  'file-text': '文档',
  activity: '监控',
  terminal: '开发工具',
  gamepad: '游戏',
  mail: '邮件',
  message: '聊天',
  book: '阅读',
  box: '容器',
  layers: '通用',
}

/** 「全部」「常用」两个固定标签的图标（不可自定义）。 */
export const TAB_ICON_KEYS = {
  all: 'layout-grid',
  pinned: 'star',
} as const satisfies Record<string, CategoryIconKey>

/**
 * 关键词表：命中即用，**关键词越长优先级越高**（同长度按表序）。
 * 关键词一律小写、不带空格（匹配前会先把分类名小写化并去掉空格）。
 */
const RULES: ReadonlyArray<{ key: CategoryIconKey; keywords: readonly string[] }> = [
  {
    key: 'film',
    keywords: ['影视','影音','媒体','电影','视频','movie','video','media','film','plex','jellyfin','emby','jellyseerr','tautulli','infuse','kodi'],
  },
  { key: 'music', keywords: ['有声书','播客','音乐','music','audio','podcast','audiobook','navidrome'] },
  { key: 'image', keywords: ['相册','照片','图片','图库','image','photo','gallery','immich','photoprism'] },
  { key: 'camera', keywords: ['摄像头','相机','监控探头','camera','frigate','surveillance'] },
  {
    key: 'download',
    keywords: ['影视库整理','下载','整理','种子','download','torrent','qbittorrent','transmission','deluge','prowlarr','sonarr','radarr','bazarr'],
  },
  { key: 'house', keywords: ['智能家居','家庭','自动化','house','home','smarthome','homeassistant'] },
  {
    key: 'shield',
    keywords: ['网络与安全','网络','路由','安全','防火墙','network','router','security','firewall','vpn','adguard','pihole','wireguard','tailscale','authelia'],
  },
  { key: 'lock', keywords: ['私密','密码','凭据','vault','password','secret','vaultwarden'] },
  { key: 'key', keywords: ['密钥','证书','key','cert'] },
  { key: 'server', keywords: ['服务器','主机','机房','server','host','nas','synology','群晖','unraid'] },
  {
    key: 'hard-drive',
    keywords: ['存储','硬盘','磁盘','文件','网盘','storage','disk','file','filebrowser'],
  },
  { key: 'cloud', keywords: ['云盘','云','cloud','nextcloud','seafile','onedrive'] },
  { key: 'database-backup', keywords: ['备份','归档','backup','archive','duplicati','restic','borg'] },
  { key: 'file-text', keywords: ['文档','笔记','知识库','document','docs','note','wiki','paperless','bookstack'] },
  {
    key: 'activity',
    keywords: ['监控','统计','仪表','性能','metrics','monitor','grafana','netdata','prometheus','uptime'],
  },
  { key: 'terminal', keywords: ['开发','代码','工具','terminal','dev','code','tool','shell','ssh'] },
  { key: 'gamepad', keywords: ['游戏','game','steam','emulator'] },
  { key: 'mail', keywords: ['邮件','邮局','mail','smtp','imap'] },
  { key: 'message', keywords: ['聊天','通讯','社交','chat','message','matrix','telegram'] },
  { key: 'book', keywords: ['阅读','学习','书','read','book','study'] },
  { key: 'box', keywords: ['容器','应用','docker','container','portainer','app'] },
]

/** 判断某个值是不是合法的图标键（后台接口的格式校验在 Go 侧）。 */
export function isCategoryIconKey(value: string | null | undefined): value is CategoryIconKey {
  return typeof value === 'string' && (CATEGORY_ICON_KEYS as readonly string[]).includes(value)
}

/**
 * 取分类图标键：`override`（后台手选）合法就优先用它，否则按名称自动匹配，
 * 都不中时回退 `layers`。
 */
export function categoryIconKey(name: string, override?: string | null): CategoryIconKey {
  if (isCategoryIconKey(override)) return override
  const normalized = (name ?? '').trim().toLowerCase().replace(/\s+/g, '')
  let bestKey: CategoryIconKey | null = null
  let bestLength = 0
  for (const rule of RULES) {
    for (const keyword of rule.keywords) {
      if (keyword.length > bestLength && normalized.includes(keyword)) {
        bestKey = rule.key
        bestLength = keyword.length
      }
    }
  }
  return bestKey ?? CATEGORY_ICON_FALLBACK
}

/** 导航标签用的图标键：全部 / 常用 固定，其余分类按手选或名称自动匹配。 */
export function tabIconKey(
  id: number | 'all' | 'pinned',
  name: string,
  icon?: string | null,
): CategoryIconKey {
  if (id === 'all') return TAB_ICON_KEYS.all
  if (id === 'pinned') return TAB_ICON_KEYS.pinned
  return categoryIconKey(name, icon)
}
