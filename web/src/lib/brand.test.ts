import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

/**
 * 品牌一致性守卫：浏览器标签页图标（web/public/icon.svg）与页面徽章（BrandMark）
 * 必须是同一造型、同一配色 —— 这两处曾经各画一版，导致 tab 图标与页面 logo 看起来不一样。
 * 这里直接读源码比对：几何参数逐项相等；颜色则由 styles.css 的主题令牌反推期望值。
 */
const icon = readFileSync(new URL('../../public/icon.svg', import.meta.url), 'utf8')
const mark = readFileSync(new URL('../components/brand-logo.tsx', import.meta.url), 'utf8')
const html = readFileSync(new URL('../../index.html', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

const viewBox = (src: string) => src.match(/viewBox="([^"]+)"/)?.[1]
const rectRx = (src: string) => src.match(/<rect[^>]*\brx="([\d.]+)"/)?.[1]
const pathD = (src: string) => src.match(/\bd="([^"]+)"/)?.[1]
const circleR = (src: string) => src.match(/<circle[^>]*\br="([\d.]+)"/)?.[1]
const strokeWidths = (src: string) =>
  [...src.matchAll(/stroke(?:Width|-width)="([\d.]+)"/g)].map((match) => match[1])

/** 取 styles.css 中某个块（:root / .dark）里的 oklch 中性色令牌。 */
function tokenLightness(blockSelector: string, variable: string): number {
  const block = new RegExp(`${blockSelector}\\s*\\{([\\s\\S]*?)\\n\\}`).exec(styles)?.[1]
  assert.ok(block, `styles.css 找不到 ${blockSelector} 块`)
  const value = new RegExp(`${variable}:\\s*oklch\\(([\\d.]+)\\s+0\\s+0\\)`).exec(block)?.[1]
  assert.ok(value, `${blockSelector} 里找不到 ${variable} 的 oklch(L 0 0) 值`)
  return Number(value)
}

/**
 * oklch(L 0 0) → sRGB 十六进制。
 * 中性色（C=0）在 Oklab 里恰好是 l=m=s=L，转线性 sRGB 后三个通道都等于 L³，再做 sRGB 传递函数即可。
 * 用实测值校验：L=0.145 → #0a0a0a；L=0.985 → #fafafa。
 */
function neutralHex(lightness: number): string {
  const linear = lightness ** 3
  const encoded = linear <= 0.0031308 ? 12.92 * linear : 1.055 * linear ** (1 / 2.4) - 0.055
  const channel = Math.round(Math.min(1, Math.max(0, encoded)) * 255)
  return '#' + channel.toString(16).padStart(2, '0').repeat(3)
}

const lightForeground = neutralHex(tokenLightness(':root', '--foreground'))
const lightBackground = neutralHex(tokenLightness(':root', '--background'))
const darkForeground = neutralHex(tokenLightness('\\.dark', '--foreground'))
const darkBackground = neutralHex(tokenLightness('\\.dark', '--background'))

test('tab 图标与页面徽章几何参数一致', () => {
  assert.deepEqual(
    viewBox(icon)?.split(' ').map(Number),
    [0, 0, 40, 40],
    '图标 viewBox 应为 0 0 40 40',
  )
  assert.equal(viewBox(icon), viewBox(mark), 'viewBox 不一致')
  assert.equal(rectRx(icon), rectRx(mark), '徽章圆角不一致')
  assert.equal(pathD(icon), pathD(mark), '「N」路径不一致')
  assert.deepEqual(strokeWidths(icon), strokeWidths(mark), '笔画宽度不一致（顺序：N → 中点节点）')
  assert.equal(circleR(icon), circleR(mark), '中点节点半径不一致')
})

test('tab 图标颜色 = 主题令牌的等效 sRGB（浅色 + 深色两套）', () => {
  // 换算自检：确保 oklch→sRGB 的实现没跑偏
  assert.equal(neutralHex(0.145), '#0a0a0a')
  assert.equal(neutralHex(0.985), '#fafafa')
  assert.equal(neutralHex(1), '#ffffff')

  // 浅色：底 = --foreground、字形 = --background
  assert.match(icon, new RegExp(`fill="${lightForeground}"`), `浅色底应为 ${lightForeground}`)
  assert.match(icon, new RegExp(`stroke="${lightBackground}"`), `浅色字形应为 ${lightBackground}`)

  // 深色覆盖：底 = 深色 --foreground、字形 = 深色 --background
  assert.match(icon, /@media\s*\(prefers-color-scheme:\s*dark\)/)
  assert.match(icon, new RegExp(`fill:\\s*${darkForeground}`), `深色底应为 ${darkForeground}`)
  assert.match(icon, new RegExp(`stroke:\\s*${darkBackground}`), `深色字形应为 ${darkBackground}`)

  // 页面徽章用主题变量，深浅色自动跟随（无需写死颜色）
  assert.match(mark, /fill="var\(--color-foreground\)"/)
  assert.match(mark, /stroke="var\(--color-background\)"/)
})

test('图标 SVG 必须是合法 XML（注释内容里不能出现连续双连字符）', () => {
  assert.ok(icon.startsWith('<svg'), '应以内联 SVG 根元素开头')
  assert.ok(icon.trimEnd().endsWith('</svg>'), '应以 </svg> 结尾')

  // XML 注释的「内容」里不允许出现 `--`（定界符 <!-- --> 自身不算），
  // 否则整份 SVG 解析失败 —— 浏览器会直接把 favicon 渲染成报错页。
  const bodies = [...icon.matchAll(/<!--([\s\S]*?)-->/g)].map((match) => match[1])
  assert.ok(bodies.length > 0, '图标应带说明注释')
  for (const body of bodies) {
    assert.ok(!body.includes('--'), 'XML 注释内容里不能出现「--」（会让整个图标解析失败）')
  }

  // 注释之外也不应出现裸露的 `--`（例如误写成 var(--x)）
  const outside = icon.replace(/<!--[\s\S]*?-->/g, '')
  assert.ok(!outside.includes('--'), '图标正文里不应出现「--」')
})

test('index.html 引用图标文件而不是另画一版内联图标', () => {
  assert.match(html, /<link[^>]*rel="icon"[^>]*href="\/icon\.svg"/)
  assert.ok(!html.includes('data:image/svg+xml'), '不应再内联 data URI 图标（会与组件各画一版）')
})
