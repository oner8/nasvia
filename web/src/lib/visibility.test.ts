import test from 'node:test'
import assert from 'node:assert/strict'

import {
  categoryVisibilityMap,
  effectiveVisibility,
  isSiteVisible,
  visibleCategories,
  visibleSites,
} from './visibility.ts'
import type { CategoryView, Site } from './types'

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

function category(overrides: Partial<CategoryView> & Pick<CategoryView, 'id' | 'name'>): CategoryView {
  return {
    sort: overrides.id,
    visibility: 'public',
    site_count: 0,
    icon: '',
    ...overrides,
  }
}

const publicCat = category({ id: 1, name: '媒体中心', visibility: 'public' })
const privateCat = category({ id: 2, name: '私密分类', visibility: 'private' })

test('站点三态可见性：public / private / inherit', () => {
  const map = categoryVisibilityMap([publicCat, privateCat])
  assert.equal(effectiveVisibility({ visibility: 'public', category_id: 2 }, map), 'private')
  assert.equal(effectiveVisibility({ visibility: 'private', category_id: 1 }, map), 'private')
  assert.equal(effectiveVisibility({ visibility: 'inherit', category_id: 1 }, map), 'public')
  assert.equal(effectiveVisibility({ visibility: 'inherit', category_id: 2 }, map), 'private')
})

test('inherit 且无分类时视为公开', () => {
  const map = categoryVisibilityMap([privateCat])
  assert.equal(effectiveVisibility({ visibility: 'inherit', category_id: null }, map), 'public')
  assert.equal(isSiteVisible({ visibility: 'inherit', category_id: null }, map, false), true)
})

test('未登录只看到公开站点，登录后可见全部', () => {
  const sites = [
    site({ id: 1, name: 'Jellyfin', category_id: publicCat.id, visibility: 'inherit' }),
    site({ id: 2, name: 'Vaultwarden', category_id: privateCat.id, visibility: 'private' }),
    site({ id: 3, name: 'File Station', category_id: privateCat.id, visibility: 'inherit' }),
  ]
  const categories = [publicCat, privateCat]

  assert.deepEqual(
    visibleSites(sites, categories, false).map((item) => item.name),
    ['Jellyfin'],
  )
  assert.equal(visibleSites(sites, categories, true).length, 3)
})

test('私密分类连带隐藏其下继承站点', () => {
  const sites = [
    site({ id: 1, name: '群晖 DSM', category_id: privateCat.id, visibility: 'private' }),
    site({ id: 2, name: 'File Station', category_id: privateCat.id, visibility: 'inherit' }),
  ]
  const result = visibleSites(sites, [privateCat], false)
  assert.equal(result.length, 0)
  assert.equal(visibleCategories(sites, [privateCat], false).length, 0)
})

test('分类私密时站点一律不可见，且分类不因公开站点而提升', () => {
  const sites = [
    site({ id: 1, name: '历史公开面板', category_id: privateCat.id, visibility: 'public' }),
    site({ id: 2, name: '内部面板', category_id: privateCat.id, visibility: 'private' }),
    site({ id: 3, name: '继承面板', category_id: privateCat.id, visibility: 'inherit' }),
  ]
  assert.deepEqual(visibleSites(sites, [privateCat], false), [])
  assert.equal(visibleCategories(sites, [privateCat], false).length, 0, '私密分类不应对外可见')

  // 分类改为公开后，public / inherit 重新可见
  const opened = category({ id: privateCat.id, name: '私密分类', visibility: 'public' })
  assert.deepEqual(
    visibleSites(sites, [opened], false).map((item) => item.name),
    ['历史公开面板', '继承面板'],
  )
  assert.equal(visibleCategories(sites, [opened], false)[0]?.site_count, 2)
})

test('分类计数按身份重算', () => {
  const sites = [
    site({ id: 1, name: 'A', category_id: publicCat.id, visibility: 'public' }),
    site({ id: 2, name: 'B', category_id: publicCat.id, visibility: 'private' }),
  ]
  const anon = visibleCategories(sites, [publicCat], false)
  assert.equal(anon[0]?.site_count, 1)

  const authed = visibleCategories(sites, [publicCat], true)
  assert.equal(authed[0]?.site_count, 2)
})
