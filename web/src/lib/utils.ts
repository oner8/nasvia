import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Tailwind 类名合并。 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** 从 URL 中取出主机名（去掉端口）。 */
export function hostOf(raw: string): string {
  const value = (raw ?? '').trim()
  if (!value) return ''
  try {
    const url = new URL(value.includes('://') ? value : `https://${value}`)
    return url.hostname.toLowerCase()
  } catch {
    return ''
  }
}

/** 供卡片副标题展示的简短主机名。 */
export function shortHost(raw: string): string {
  const host = hostOf(raw)
  if (!host) return raw
  return host.replace(/^www\./, '')
}

/** 由名称生成确定性的头像配色（与后端占位图保持同样的视觉语言）。 */
export function avatarGradient(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i += 1) {
    hash = (hash * 31 + name.charCodeAt(i)) % 360
  }
  const hue = hash
  return `linear-gradient(135deg, hsl(${hue} 72% 55%), hsl(${(hue + 42) % 360} 68% 42%))`
}

export function firstLetter(name: string): string {
  const trimmed = (name ?? '').trim()
  return trimmed ? trimmed.slice(0, 1).toUpperCase() : '?'
}

/** 拆分逗号/空格分隔的标签。 */
export function splitTags(tags: string): string[] {
  return (tags ?? '')
    .split(/[\s,，;；]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}
