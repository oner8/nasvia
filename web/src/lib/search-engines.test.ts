import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_ENGINE_ID,
  ENGINE_STORAGE_KEY,
  SEARCH_ENGINES,
  buildSearchUrl,
  getEngine,
  normalizeEngineId,
  readStoredEngineId,
} from './search-engines.ts'

test('引擎清单：默认引擎存在且模板都含 {q} 占位符', () => {
  assert.ok(SEARCH_ENGINES.some((engine) => engine.id === DEFAULT_ENGINE_ID))
  for (const engine of SEARCH_ENGINES) {
    assert.ok(engine.template.includes('{q}'), `${engine.id} 缺少占位符`)
    assert.match(engine.template, /^https:\/\//)
  }
})

test('buildSearchUrl：默认引擎 + 关键词 URL 编码', () => {
  assert.equal(buildSearchUrl('hello'), 'https://www.google.com/search?q=hello')
  assert.equal(buildSearchUrl('hello world'), 'https://www.google.com/search?q=hello%20world')
  assert.equal(buildSearchUrl('  前后空白  '), 'https://www.google.com/search?q=%E5%89%8D%E5%90%8E%E7%A9%BA%E7%99%BD')
})

test('buildSearchUrl：特殊字符被编码，不会截断查询串', () => {
  assert.equal(buildSearchUrl('a&b=c'), 'https://www.google.com/search?q=a%26b%3Dc')
  assert.equal(buildSearchUrl('a#b'), 'https://www.google.com/search?q=a%23b')
  assert.equal(buildSearchUrl('a+b'), 'https://www.google.com/search?q=a%2Bb')
})

test('buildSearchUrl：空查询返回 null（不跳转）', () => {
  assert.equal(buildSearchUrl(''), null)
  assert.equal(buildSearchUrl('   '), null)
  assert.equal(buildSearchUrl(undefined as unknown as string), null)
})

test('buildSearchUrl：指定引擎生效（百度用 wd，Google / Bing 用 q）', () => {
  assert.equal(buildSearchUrl('nas', 'baidu'), 'https://www.baidu.com/s?wd=nas')
  assert.equal(buildSearchUrl('nas', 'bing'), 'https://www.bing.com/search?q=nas')
  assert.equal(buildSearchUrl('nas', 'google'), 'https://www.google.com/search?q=nas')
})

test('搜索引擎只剩 Google / Bing / 百度（DuckDuckGo 已移除）', () => {
  assert.deepEqual(SEARCH_ENGINES.map((engine) => engine.id), ['google', 'bing', 'baidu'])
  assert.equal(SEARCH_ENGINES.some((engine) => engine.id === 'duckduckgo'), false)
  // 老浏览器可能残留 duckduckgo 选择：一律回落默认引擎
  assert.equal(normalizeEngineId('duckduckgo'), DEFAULT_ENGINE_ID)
  assert.equal(buildSearchUrl('nas', 'duckduckgo'), 'https://www.google.com/search?q=nas')
})

test('未知引擎 id 回退到默认引擎', () => {
  assert.equal(getEngine('nope').id, DEFAULT_ENGINE_ID)
  assert.equal(getEngine(undefined).id, DEFAULT_ENGINE_ID)
  assert.equal(normalizeEngineId('nope'), DEFAULT_ENGINE_ID)
  assert.equal(normalizeEngineId('bing'), 'bing')
  assert.equal(buildSearchUrl('x', 'nope'), 'https://www.google.com/search?q=x')
})

test('readStoredEngineId：正常读取、非法值回退、无存储回退、getItem 抛错也不崩', () => {
  const store = (value: string | null) => ({ getItem: (key: string) => (key === ENGINE_STORAGE_KEY ? value : null) })
  assert.equal(readStoredEngineId(store('baidu')), 'baidu')
  assert.equal(readStoredEngineId(store('not-an-engine')), DEFAULT_ENGINE_ID)
  assert.equal(readStoredEngineId(store(null)), DEFAULT_ENGINE_ID)
  assert.equal(readStoredEngineId(undefined), DEFAULT_ENGINE_ID)
  assert.equal(
    readStoredEngineId({
      getItem() {
        throw new Error('blocked')
      },
    }),
    DEFAULT_ENGINE_ID,
  )
})
