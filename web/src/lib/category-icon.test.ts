import assert from 'node:assert/strict'
import test from 'node:test'

import {
  CATEGORY_ICON_FALLBACK,
  CATEGORY_ICON_KEYS,
  CATEGORY_ICON_NAME,
  TAB_ICON_KEYS,
  categoryIconKey,
  isCategoryIconKey,
  tabIconKey,
} from './category-icon.ts'

test('演示数据的 5 个分类名各自命中不同图标', () => {
  const pairs: Array<[string, string]> = [
    ['媒体中心', 'film'],
    ['下载与整理', 'download'],
    ['家庭自动化', 'house'],
    ['网络与安全', 'shield'],
    ['私密分类', 'lock'],
  ]
  for (const [name, expected] of pairs) {
    assert.equal(categoryIconKey(name), expected, `${name} 应匹配 ${expected}`)
  }
  const keys = pairs.map(([name]) => categoryIconKey(name))
  assert.equal(new Set(keys).size, keys.length, `5 个分类图标必须互不相同：${keys.join(', ')}`)
})

test('最长关键词优先：更具体的关键词胜出', () => {
  // 「影视库整理」同时含「影视」(film) 与「影视库整理」(download)，长的赢
  assert.equal(categoryIconKey('影视库整理'), 'download')
  // 「网络与安全」优先命中整词，而不是被别的规则抢走
  assert.equal(categoryIconKey('网络与安全'), 'shield')
  assert.equal(categoryIconKey('网络安全'), 'shield')
})

test('英文名：忽略大小写与空格', () => {
  assert.equal(categoryIconKey('Media Center'), 'film')
  assert.equal(categoryIconKey('  HOME  '), 'house')
  assert.equal(categoryIconKey('Next Cloud'), 'cloud')
})

test('未匹配时回退通用图标', () => {
  assert.equal(categoryIconKey('我的东西'), CATEGORY_ICON_FALLBACK)
  assert.equal(categoryIconKey('随便起的名字'), 'layers')
})

test('空名与纯符号不抛错', () => {
  assert.equal(categoryIconKey(''), CATEGORY_ICON_FALLBACK)
  assert.equal(categoryIconKey('   '), CATEGORY_ICON_FALLBACK)
  assert.equal(categoryIconKey('——…!@#'), CATEGORY_ICON_FALLBACK)
})

test('后台手选的图标优先于自动匹配', () => {
  assert.equal(categoryIconKey('媒体中心', 'cloud'), 'cloud')
  assert.equal(categoryIconKey('随便起的名字', 'star'), 'star')
  // 非法 / 空的 override 一律忽略，回落到自动匹配
  assert.equal(categoryIconKey('媒体中心', ''), 'film')
  assert.equal(categoryIconKey('媒体中心', null), 'film')
  assert.equal(categoryIconKey('媒体中心', 'Not A Key!'), 'film')
  assert.equal(categoryIconKey('随便起的名字', 'Not A Key!'), CATEGORY_ICON_FALLBACK)
})

test('导航标签图标：全部 / 常用固定，分类走自动匹配', () => {
  assert.equal(tabIconKey('all', '全部'), TAB_ICON_KEYS.all)
  assert.equal(tabIconKey('pinned', '常用'), TAB_ICON_KEYS.pinned)
  assert.equal(tabIconKey(3, '网络与安全'), 'shield')
  assert.equal(tabIconKey(4, '媒体中心', 'gamepad'), 'gamepad')
})

test('相册与文件各自命中不同图标（关键词不互相误伤）', () => {
  assert.equal(categoryIconKey('我的相册'), 'image')
  assert.equal(categoryIconKey('文件'), 'hard-drive')
  assert.equal(categoryIconKey('文档'), 'file-text')
})

test('图标键集合完整：无重复、含回退键与两个固定键', () => {
  assert.equal(new Set(CATEGORY_ICON_KEYS).size, CATEGORY_ICON_KEYS.length, '图标键不应重复')
  assert.ok(CATEGORY_ICON_KEYS.includes(CATEGORY_ICON_FALLBACK))
  assert.ok(CATEGORY_ICON_KEYS.includes(TAB_ICON_KEYS.all))
  assert.ok(CATEGORY_ICON_KEYS.includes(TAB_ICON_KEYS.pinned))
})

test('每个图标键都有非空中文名（后台选择器用）', () => {
  for (const key of CATEGORY_ICON_KEYS) {
    assert.ok(CATEGORY_ICON_NAME[key]?.length > 0, `${key} 缺中文名`)
  }
})

test('isCategoryIconKey 只接受已知键', () => {
  assert.equal(isCategoryIconKey('film'), true)
  assert.equal(isCategoryIconKey('layers'), true)
  assert.equal(isCategoryIconKey('nope'), false)
  assert.equal(isCategoryIconKey(''), false)
  assert.equal(isCategoryIconKey(null), false)
  assert.equal(isCategoryIconKey(undefined), false)
})
