import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  DEFAULT_NAV_POSITION,
  DEFAULT_NAV_STYLE,
  NAV_BAR_CLASS,
  NAV_CONTENT_CLASS,
  NAV_INDICATOR_ID,
  NAV_ITEM_ACTIVE_CLASS,
  NAV_ITEM_CLASS,
  NAV_ITEM_ICON_CLASS,
  NAV_ITEM_IDLE_CLASS,
  NAV_POSITIONS,
  NAV_POSITION_NAME,
  NAV_ROW_BASE_CLASS,
  NAV_ROW_HIDDEN_CLASS,
  NAV_ROW_MAX_ICON_CLASS,
  NAV_ROW_MAX_TEXT_CLASS,
  NAV_STYLES,
  NAV_STYLE_NAME,
  NAV_VISIBLES,
  NAV_VISIBLE_NAME,
  navLabel,
  navBarClass,
  navItemClass,
  navRailClass,
  navRailColumnClass,
  navRowClass,
  normalizeNavPosition,
  normalizeNavStyle,
  normalizeNavVisible,
} from './nav.ts'

const tokens = (value: string) => String(value ?? '').split(/\s+/).filter(Boolean)
const contains = (haystack: string, needle: string) => tokens(haystack).includes(needle)
const containsAll = (haystack: string, needles: string[]) => needles.every((t) => contains(haystack, t))

test('归一化：只有 right 保留，其余（含历史值 top）一律 left', () => {
  assert.equal(normalizeNavPosition('right'), 'right')
  assert.equal(normalizeNavPosition('left'), 'left')
  // 顶部导航已取消：历史库里的 top、空值、非法值都落到 left
  for (const value of ['top', '', null, undefined, 'RIGHT', 'middle', 'bottom']) {
    assert.equal(normalizeNavPosition(value), 'left')
  }
  assert.equal(DEFAULT_NAV_POSITION, 'left')
  assert.deepEqual(NAV_POSITIONS, ['left', 'right'])
  assert.deepEqual(Object.keys(NAV_POSITION_NAME), ['left', 'right'])
})

test('标签样式归一化与默认值：默认仅图标（icon）', () => {
  assert.equal(DEFAULT_NAV_STYLE, 'icon', '默认仅图标')
  assert.equal(normalizeNavStyle('text'), 'text', '只有显式 text 才用文字')
  assert.equal(normalizeNavStyle('icon'), 'icon')
  for (const bad of ['emoji', '', null, undefined, 'ICON', 'TEXT']) {
    assert.equal(normalizeNavStyle(bad), 'icon', `${String(bad)} 应回落到默认仅图标`)
  }
  assert.deepEqual(NAV_STYLES, ['text', 'icon'])
  assert.equal(NAV_POSITION_NAME.left, '左侧')
  assert.equal(NAV_STYLE_NAME.icon, '仅图标')
})

test('药丸容器：共享样式 + 竖向（真的 display:flex，否则 items-center 不生效）', () => {
  const bar = navBarClass()
  for (const token of tokens(NAV_BAR_CLASS)) {
    assert.ok(contains(bar, token), `容器应含共享类 ${token}`)
  }
  assert.ok(contains(bar, 'flex'), '必须声明 flex')
  assert.ok(contains(bar, 'flex-col') && contains(bar, 'gap-1'))
})

test('侧栏标签栏与共享药丸样式同源', () => {
  const rail = navRailClass('icon')
  for (const token of tokens(NAV_BAR_CLASS)) {
    assert.ok(contains(rail, token), `侧栏应含共享容器类 ${token}`)
  }
  assert.ok(contains(rail, 'w-full'), '侧栏铺满所在列')
  // 组件里的真实组合：navBarClass()（提供 display:flex）+ 侧栏类
  const composed = navBarClass() + ' ' + rail
  assert.ok(contains(composed, 'flex'), '组合后必须是 display:flex，否则 items-center 不生效')
  assert.ok(contains(rail, 'flex-col'))
  assert.ok(contains(rail, 'items-center'), '仅图标时图标格居中')
  assert.ok(contains(rail, 'max-h-[85dvh]') && contains(rail, 'overflow-y-auto'), '分类多时栏内滚动')
  assert.ok(contains(navRailClass('text'), 'flex-col') && !contains(navRailClass('text'), 'items-center'))
})

test('标签样式：文字与仅图标只在图标格 / 选中态上不同', () => {
  const textIdle = navItemClass('text', false)
  const textActive = navItemClass('text', true)
  const iconIdle = navItemClass('icon', false)

  for (const value of [textIdle, textActive, iconIdle]) {
    for (const token of tokens(NAV_ITEM_CLASS)) {
      assert.ok(contains(value, token), `标签应含 ${token}`)
    }
  }
  assert.ok(contains(iconIdle, 'size-9'))
  assert.ok(!contains(textIdle, 'size-9'))
  assert.ok(contains(textActive, 'text-[var(--color-background)]'))
  const activeText: string = NAV_ITEM_ACTIVE_CLASS
  assert.ok(activeText !== NAV_ITEM_IDLE_CLASS)
  assert.ok(NAV_ITEM_ICON_CLASS.includes('justify-center'))
})

test('侧栏与内容同一行：不论窗口多宽都紧贴内容', () => {
  assert.ok(containsAll(NAV_ROW_BASE_CLASS, ['mx-auto', 'flex', 'w-full', 'justify-center', 'gap-4', 'px-4', 'sm:px-6', 'lg:px-8']))

  const left = navRowClass('left', 'icon')
  assert.ok(containsAll(left, tokens(NAV_ROW_BASE_CLASS)), '行容器应含基础类')
  assert.ok(contains(left, 'max-w-[68.5rem]'), '仅图标：宽度 = 侧栏 + 间距 + 内容 + 内边距')
  assert.ok(!contains(left, 'flex-row-reverse'), '左侧不反向')
  assert.ok(!contains(left, 'fixed'), '不能固定在窗口边缘')

  const right = navRowClass('right', 'icon')
  assert.ok(contains(right, 'flex-row-reverse'), '右侧整行反向')
  assert.ok(contains(right, 'max-w-[68.5rem]'))
  assert.equal(tokens(right).length, tokens(left).length + 1)

  const text = navRowClass('left', 'text')
  assert.ok(contains(text, 'max-w-[75rem]'), '显示名称：行更宽')
  assert.notEqual(NAV_ROW_MAX_ICON_CLASS, NAV_ROW_MAX_TEXT_CLASS)
})

test('侧栏所在列：视口等高 + 吸顶 + 垂直居中 + 手机平板隐藏', () => {
  const iconColumn = navRailColumnClass('icon')
  assert.ok(containsAll(iconColumn, ['sticky', 'top-0', 'hidden', 'lg:flex', 'h-dvh', 'shrink-0', 'items-center']))
  assert.ok(!contains(iconColumn, 'flex'), '手机/平板要真的隐藏：不能出现裸 flex')
  assert.ok(contains(iconColumn, 'w-14'), '仅图标固定 3.5rem')
  const textColumn = navRailColumnClass('text')
  assert.ok(contains(textColumn, 'w-max'), '显示名称按内容自适应（不再固定 180px）')
  assert.ok(contains(textColumn, 'max-w-[11rem]'), '极长名称有上限兜底')
  assert.ok(!contains(textColumn, 'w-40'), '不再是固定 10rem')
  assert.equal(tokens(textColumn).filter((t) => t.startsWith('h-')).join(), 'h-dvh')
})

test('分类导航显示开关：只有 hide 生效，其余回落 show', () => {
  assert.equal(normalizeNavVisible('hide'), 'hide')
  assert.equal(normalizeNavVisible('show'), 'show')
  for (const value of ['', null, undefined, 'HIDE', 'off', 'no']) {
    assert.equal(normalizeNavVisible(value), 'show')
  }
  assert.deepEqual(NAV_VISIBLES, ['show', 'hide'])
  assert.equal(NAV_VISIBLE_NAME.hide, '隐藏')
  assert.equal(NAV_VISIBLE_NAME.show, '显示')
})

test('隐藏分类导航时不预留侧栏宽度，内容仍然居中', () => {
  assert.ok(contains(NAV_ROW_HIDDEN_CLASS, 'max-w-[64rem]'), '内容 60rem + 内边距 4rem')
  const hidden = navRowClass('left', 'icon', 'hide')
  assert.ok(containsAll(hidden, tokens(NAV_ROW_BASE_CLASS)), '仍是同一份行容器基础类')
  assert.ok(contains(hidden, 'max-w-[64rem]'))
  assert.ok(!contains(hidden, NAV_ROW_MAX_ICON_CLASS), '不该再预留侧栏宽度')
  assert.ok(!contains(hidden, 'flex-row-reverse'), '没有侧栏也就不需要反向')
  assert.equal(navRowClass('right', 'icon', 'hide'), hidden, '左右两侧在隐藏时完全一致')
  // 显示时保持原样
  assert.ok(contains(navRowClass('left', 'icon', 'show'), NAV_ROW_MAX_ICON_CLASS))
  assert.equal(navRowClass('left', 'icon'), navRowClass('left', 'icon', 'show'), '默认显示')
})

test('文字标签最多两个汉字（超出截断，完整名留给悬停提示）', () => {
  assert.equal(navLabel('媒体中心'), '媒体')
  assert.equal(navLabel('下载与整理'), '下载')
  assert.equal(navLabel('家庭自动化'), '家庭')
  assert.equal(navLabel('网络与安全'), '网络')
  assert.equal(navLabel('私密分类'), '私密')
  // 已经够短的保持不变
  assert.equal(navLabel('全部'), '全部')
  assert.equal(navLabel('常用'), '常用')
  // 非汉字按「1 个 = 半个汉字」算（最多 4 个）
  assert.equal(navLabel('Jellyfin'), 'Jell')
  // 代理对（emoji）不会被截断成半个字符
  assert.equal(navLabel('🎬媒体'), '🎬媒')
  // 首尾空白忽略；空名安全
  assert.equal(navLabel('  我的相册  '), '我的')
  assert.equal(navLabel(''), '')
  assert.equal(navLabel('   '), '')
  assert.equal(Array.from(navLabel('我的相册')).length, 2)
})

test('内容列：与居中内容同宽（60rem），小屏占满整行', () => {
  assert.ok(containsAll(NAV_CONTENT_CLASS, ['flex', 'min-w-0', 'flex-1', 'min-h-dvh', 'max-w-[60rem]']))
})

test('指示器 layoutId 固定（界面上只有一份竖向导航）', () => {
  assert.equal(NAV_INDICATOR_ID, 'category-tab-indicator')
})

test('不引入新配色：只用现有 --color-* 语义色', () => {
  const all = [
    NAV_BAR_CLASS,
    NAV_ITEM_CLASS,
    NAV_ITEM_ACTIVE_CLASS,
    NAV_ITEM_IDLE_CLASS,
    navBarClass(),
    navRailClass('icon'),
    navRailClass('text'),
    navRowClass('left', 'icon'),
    navRailColumnClass('icon'),
  ].join(' ')
  assert.ok(!/(bg|text|border)-(red|green|blue|yellow|purple)-/.test(all), `不应出现 Tailwind 调色板：${all}`)
  assert.ok(!all.includes('#'), '不应出现硬编码十六进制颜色')
})
