import test from 'node:test'
import assert from 'node:assert/strict'

import { filterSitesByKeyword, moveInList, nextSort, sortSites } from './site-order.ts'
import type { Site } from './types'

function site(overrides: Partial<Site> & Pick<Site, 'id' | 'name'>): Site {
  return {
    description: '',
    url: '',
    lan_url: '',
    icon: `/api/sites/${overrides.id}/icon`,
    icon_state: 'ready',
    icon_source: 'favicon',
    category_id: null,
    sort: overrides.id,
    pinned: false,
    visibility: 'inherit',
    effective_visibility: 'public',
    tags: '',
    ...overrides,
  }
}

const categoryNames = new Map<number, string>([
  [1, '媒体中心'],
  [2, '开发运维'],
])
const categoryNameOf = (id: number | null) => (id === null ? '未分类' : (categoryNames.get(id) ?? ''))

// 输入顺序（= 手动顺序）：群晖、Jellyfin、Gitea
const sites = [
  site({
    id: 3,
    name: '群晖 DSM',
    category_id: 1,
    effective_visibility: 'private',
    url: 'https://dsm.example.com',
  }),
  site({
    id: 1,
    name: 'Jellyfin',
    category_id: 1,
    effective_visibility: 'public',
    url: 'https://jellyfin.example.com',
  }),
  site({ id: 2, name: 'Gitea', category_id: 2, effective_visibility: 'public', url: '', lan_url: 'http://192.168.1.10:3000' }),
]

test('表头三态切换：升序 → 降序 → 手动顺序', () => {
  assert.deepEqual(nextSort(null, 'name'), { key: 'name', dir: 'asc' })
  assert.deepEqual(nextSort({ key: 'name', dir: 'asc' }, 'name'), { key: 'name', dir: 'desc' })
  assert.equal(nextSort({ key: 'name', dir: 'desc' }, 'name'), null)
  // 换列时从升序重新开始
  assert.deepEqual(nextSort({ key: 'name', dir: 'desc' }, 'address'), { key: 'address', dir: 'asc' })
})

test('sort 为 null 时原样返回（手动顺序）', () => {
  assert.deepEqual(
    sortSites(sites, null, categoryNameOf).map((s) => s.id),
    [3, 1, 2],
  )
})

test('按站点名排序：汉字按拼音、中文排在英文前，可升可降', () => {
  assert.deepEqual(
    sortSites(sites, { key: 'name', dir: 'asc' }, categoryNameOf).map((s) => s.name),
    ['群晖 DSM', 'Gitea', 'Jellyfin'],
  )
  assert.deepEqual(
    sortSites(sites, { key: 'name', dir: 'desc' }, categoryNameOf).map((s) => s.name),
    ['Jellyfin', 'Gitea', '群晖 DSM'],
  )
})

test('按分类排序且保持稳定（同分类沿用原顺序）', () => {
  const withNone = [...sites, site({ id: 4, name: '未分类站点' })]
  assert.deepEqual(
    sortSites(withNone, { key: 'category', dir: 'asc' }, categoryNameOf).map((s) => s.name),
    ['Gitea', '群晖 DSM', 'Jellyfin', '未分类站点'],
  )
})

test('按可见性排序用的是生效可见性', () => {
  assert.deepEqual(
    sortSites(sites, { key: 'visibility', dir: 'asc' }, categoryNameOf).map((s) => s.id),
    [1, 2, 3],
  )
})

test('按地址排序：外网为空时回落到内网地址', () => {
  assert.deepEqual(
    sortSites(sites, { key: 'address', dir: 'asc' }, categoryNameOf).map((s) => s.id),
    [2, 3, 1],
  )
})

test('排序不修改入参数组', () => {
  const input = [...sites]
  sortSites(input, { key: 'name', dir: 'asc' }, categoryNameOf)
  assert.deepEqual(
    input.map((s) => s.id),
    [3, 1, 2],
  )
})

test('拖动：插到目标之前 / 之后', () => {
  assert.deepEqual(moveInList([1, 2, 3], 3, 1, false), [3, 1, 2])
  assert.deepEqual(moveInList([1, 2, 3], 1, 3, true), [2, 3, 1])
  assert.deepEqual(moveInList([1, 2, 3, 4], 2, 4, false), [1, 3, 2, 4])
})

test('拖动：拖到自己身上或未知 id 时原样返回', () => {
  assert.deepEqual(moveInList([1, 2, 3], 2, 2, false), [1, 2, 3])
  assert.deepEqual(moveInList([1, 2, 3], 9, 1, false), [1, 2, 3])
  assert.deepEqual(moveInList([1, 2, 3], 1, 9, true), [1, 2, 3])
})

test('关键词筛选：保序、多词为「与」、可命中描述与地址', () => {
  const all = [
    ...sites,
    site({ id: 4, name: 'Moviepilot', description: '电影自动下载', url: 'https://mp.example.com' }),
  ]
  assert.equal(filterSitesByKeyword(all, '').length, all.length)
  assert.deepEqual(
    filterSitesByKeyword(all, 'gitea').map((s) => s.id),
    [2],
  )
  // 多个关键词必须全部命中
  assert.deepEqual(
    filterSitesByKeyword(all, '电影 下载').map((s) => s.id),
    [4],
  )
  assert.deepEqual(filterSitesByKeyword(all, '电影 不存在').map((s) => s.id), [])
  // 命中地址，并保持传入顺序（不重排）
  assert.deepEqual(
    filterSitesByKeyword(all, 'example.com').map((s) => s.id),
    [3, 1, 4],
  )
})
