import test from 'node:test'
import assert from 'node:assert/strict'

import { scoreSite, searchSites, searchVisibleSites } from './search.ts'
import type { CategoryView, Site } from './types'

function site(overrides: Partial<Site> & Pick<Site, 'id' | 'name'>): Site {
  return {
    description: '',
    url: '',
    lan_url: '',
    icon: `/api/sites/${overrides.id}/icon`,
    icon_state: 'ready',
    icon_source: 'hdicons',
    category_id: null,
    sort: overrides.id,
    pinned: false,
    visibility: 'public',
    effective_visibility: 'public',
    tags: '',
    ...overrides,
  }
}

const jellyfin = site({
  id: 1,
  name: 'Jellyfin',
  description: '电影 / 剧集流媒体服务',
  url: 'https://jellyfin.example.com',
  lan_url: 'http://192.168.1.10:8096',
  tags: '影音',
})
const adguard = site({
  id: 2,
  name: 'AdGuard Home',
  description: '全网 DNS 广告过滤',
  url: 'https://adguard.example.com',
  visibility: 'public',
  tags: 'DNS',
})
const vault = site({
  id: 3,
  name: 'Vaultwarden',
  description: '自托管密码库',
  url: 'https://vault.example.com',
  visibility: 'private',
  effective_visibility: 'private',
})

test('名称匹配大小写不敏感', () => {
  assert.ok(scoreSite(jellyfin, 'jelly') > 0)
  assert.ok(scoreSite(jellyfin, 'JELLYFIN') > 0)
})

test('描述、地址、标签均可命中', () => {
  assert.ok(scoreSite(jellyfin, '流媒体') > 0)
  assert.ok(scoreSite(jellyfin, '192.168.1.10') > 0)
  assert.ok(scoreSite(adguard, 'dns') > 0)
  assert.equal(scoreSite(adguard, '没有这个词'), 0)
})

test('名称前缀命中排序靠前', () => {
  const sites = [
    site({ id: 10, name: '我的 Jellyfin 镜像', description: 'jellyfin 备份' }),
    site({ id: 11, name: 'Jellyfin', description: '主服务' }),
  ]
  const result = searchSites(sites, 'jellyfin')
  assert.equal(result[0]?.name, 'Jellyfin')
})

test('多个关键词之间是「与」关系', () => {
  const sites = [jellyfin, adguard]
  assert.equal(searchSites(sites, 'adguard 过滤').length, 1)
  assert.equal(searchSites(sites, 'adguard 电影').length, 0)
})

test('空查询返回全部站点并按置顶/排序排列', () => {
  const sites = [
    site({ id: 20, name: 'B', sort: 2 }),
    site({ id: 21, name: 'A', sort: 1 }),
    site({ id: 22, name: 'C', sort: 3, pinned: true }),
  ]
  assert.deepEqual(
    searchSites(sites, '').map((item) => item.name),
    ['C', 'A', 'B'],
  )
})

test('未登录时私密站点不会出现在搜索结果里', () => {
  const sites = [jellyfin, adguard, vault]
  const categories: CategoryView[] = []

  const anon = searchVisibleSites(sites, categories, false, 'vault')
  assert.equal(anon.length, 0)

  const authed = searchVisibleSites(sites, categories, true, 'vault')
  assert.equal(authed.length, 1)
  assert.equal(authed[0]?.name, 'Vaultwarden')

  const anonAll = searchVisibleSites(sites, categories, false, '')
  assert.deepEqual(anonAll.map((item) => item.name).sort(), ['AdGuard Home', 'Jellyfin'])
})
