import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DEFAULT_VIEW_MODE,
  GRID_CLASS,
  VIEW_MODES,
  VIEW_NAME,
  VIEW_STORAGE_KEY,
  nextViewMode,
  normalizeViewMode,
  readStoredViewMode,
  writeStoredViewMode,
} from './view-mode.ts'

/** 内存版 Storage（只实现用到的两个方法）。 */
function memoryStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    dump: () => Object.fromEntries(data),
  }
}

test('默认视图是应用图标，非法值一律回退', () => {
  assert.equal(DEFAULT_VIEW_MODE, 'app')
  assert.equal(normalizeViewMode('app'), 'app')
  assert.equal(normalizeViewMode('tile'), 'tile')
  assert.equal(normalizeViewMode('APP'), 'app')
  assert.equal(normalizeViewMode('grid'), 'app')
  assert.equal(normalizeViewMode(''), 'app')
  assert.equal(normalizeViewMode(null), 'app')
  assert.equal(normalizeViewMode(undefined), 'app')
})

test('读取存储：缺省/非法/不可用时返回默认，合法值原样返回', () => {
  assert.equal(readStoredViewMode(undefined), 'app')
  assert.equal(readStoredViewMode(memoryStorage()), 'app')
  assert.equal(readStoredViewMode(memoryStorage({ [VIEW_STORAGE_KEY]: 'app' })), 'app')
  // 已显式选过磁贴的浏览器保持自己的选择
  assert.equal(readStoredViewMode(memoryStorage({ [VIEW_STORAGE_KEY]: 'tile' })), 'tile')
  assert.equal(readStoredViewMode(memoryStorage({ [VIEW_STORAGE_KEY]: 'nope' })), 'app')

  const throwing = {
    getItem: () => {
      throw new Error('storage disabled')
    },
  }
  assert.equal(readStoredViewMode(throwing), 'app')
})

test('写入存储：写入规整后的值，存储不可用不抛错', () => {
  const storage = memoryStorage()
  writeStoredViewMode(storage, 'app')
  assert.deepEqual(storage.dump(), { [VIEW_STORAGE_KEY]: 'app' })
  writeStoredViewMode(storage, 'tile')
  assert.deepEqual(storage.dump(), { [VIEW_STORAGE_KEY]: 'tile' })
  writeStoredViewMode(undefined, 'app')

  const throwing = {
    setItem: () => {
      throw new Error('quota exceeded')
    },
  }
  assert.doesNotThrow(() => writeStoredViewMode(throwing, 'app'))
})

test('切换：磁贴 ⇄ 应用 双向、可逆', () => {
  assert.equal(nextViewMode('tile'), 'app')
  assert.equal(nextViewMode('app'), 'tile')
  assert.equal(nextViewMode(nextViewMode('tile')), 'tile')
})

test('两种视图的网格类名齐备且互不相同（含合法 Tailwind 字面量）', () => {
  for (const mode of VIEW_MODES) {
    const value = GRID_CLASS[mode]
    assert.ok(value.trim().length > 0, `${mode} 网格类名不应为空`)
    assert.ok(value.startsWith('grid '), `${mode} 应以 grid 开头`)
  }
  assert.notEqual(GRID_CLASS.tile, GRID_CLASS.app)
  assert.ok(GRID_CLASS.tile.includes('grid-cols-2') && GRID_CLASS.tile.includes('lg:grid-cols-4'))
  assert.ok(GRID_CLASS.app.includes('grid-cols-4') && GRID_CLASS.app.includes('lg:grid-cols-10'))
})

test('展示名两种视图都非空，且应用图标排在菜单前面', () => {
  assert.deepEqual([...VIEW_MODES], ['app', 'tile'], '菜单顺序：应用图标在前')
  assert.equal(VIEW_MODES[0], DEFAULT_VIEW_MODE, '默认视图就是菜单第一项')
  for (const mode of VIEW_MODES) {
    assert.ok(VIEW_NAME[mode].length > 0)
  }
})
