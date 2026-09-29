import { cn } from '../lib/utils'

/**
 * 搜索引擎品牌图标：项目内置的 iTab 风格 SVG（`web/public/engines/*.svg`）。
 *
 * 图标文件随前端一起打包进容器，使用时只请求本机静态资源 ——
 * 不访问任何第三方图床，离线也能正常显示。
 *
 * 新增引擎时：把 SVG 放进 `web/public/engines/`，并在下面登记文件名即可；
 * 未登记（或图标缺失）时回退 Google 图标。
 */
const LOGO_FILES: Record<string, string> = {
  google: 'google.svg',
  bing: 'bing.svg',
  baidu: 'baidu.svg',
}

const FALLBACK_FILE = 'google.svg'

export interface EngineLogoProps {
  /** 引擎 id（未知时回退 Google 图标）。 */
  id: string
  className?: string
}

/** 渲染某个搜索引擎的品牌图标（本地静态 SVG）。 */
export function EngineLogo({ id, className }: EngineLogoProps) {
  return (
    <img
      src={`/engines/${LOGO_FILES[id] ?? FALLBACK_FILE}`}
      alt=""
      aria-hidden="true"
      draggable={false}
      className={cn('size-4 shrink-0 select-none', className)}
    />
  )
}
