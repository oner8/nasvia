import {
  Activity,
  Book,
  Box,
  Camera,
  Cloud,
  DatabaseBackup,
  Download,
  FileText,
  Film,
  Gamepad2,
  HardDrive,
  House,
  Image,
  Key,
  Layers,
  LayoutGrid,
  Lock,
  Mail,
  MessageSquare,
  Music,
  Server,
  Shield,
  Star,
  Terminal,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

import { CATEGORY_ICON_FALLBACK, categoryIconKey, type CategoryIconKey } from '../lib/category-icon'

/** 图标键 → lucide 图标（与 `CATEGORY_ICON_KEYS` 一一对应）。 */
export const CATEGORY_ICONS: Record<CategoryIconKey, LucideIcon> = {
  'layout-grid': LayoutGrid,
  star: Star,
  film: Film,
  music: Music,
  image: Image,
  camera: Camera,
  download: Download,
  house: House,
  shield: Shield,
  lock: Lock,
  key: Key,
  server: Server,
  'hard-drive': HardDrive,
  cloud: Cloud,
  'database-backup': DatabaseBackup,
  'file-text': FileText,
  activity: Activity,
  terminal: Terminal,
  gamepad: Gamepad2,
  mail: Mail,
  message: MessageSquare,
  book: Book,
  box: Box,
  layers: Layers,
}

export interface CategoryIconProps {
  /** 分类名（自动匹配用）。 */
  name: string
  /** 后台手选的图标键；留空 = 按名称自动匹配。 */
  icon?: string | null
  /** 直接指定图标键（导航的「全部 / 常用」用，优先于自动匹配）。 */
  iconKey?: CategoryIconKey
  className?: string
}

/** 分类图标：手选优先，其次按名称自动匹配，最后回退通用图标。 */
export function CategoryIcon({ name, icon, iconKey, className }: CategoryIconProps) {
  const key = iconKey ?? categoryIconKey(name, icon)
  const Icon = CATEGORY_ICONS[key] ?? CATEGORY_ICONS[CATEGORY_ICON_FALLBACK]
  return <Icon className={className} aria-hidden />
}
