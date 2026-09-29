import assert from 'node:assert/strict'
import test from 'node:test'

import { iconPollDelay, mergeIconStatuses } from './icon-poll.ts'
import type { Site, SiteListResponse } from './types.ts'

const site = {
  id: 1,
  name: 'NAS',
  description: '',
  url: 'https://nas.example.com',
  lan_url: '',
  icon: '/old',
  icon_state: 'pending',
  icon_source: 'favicon',
  category_id: null,
  sort: 1,
  pinned: false,
  visibility: 'public',
  effective_visibility: 'public',
  tags: '',
} satisfies Site

test('轮询间隔按等待时间递增，请求失败时退避', () => {
  assert.equal(iconPollDelay(0), 2_000)
  assert.equal(iconPollDelay(30_000), 5_000)
  assert.equal(iconPollDelay(120_000), 10_000)
  assert.equal(iconPollDelay(0, 1), 4_000)
  assert.equal(iconPollDelay(30_000, 2), 10_000)
})

test('轻量图标状态只更新对应站点字段', () => {
  const current: SiteListResponse = { items: [site], total: 1, authenticated: false }
  const next = mergeIconStatuses(current, [
    { id: 1, icon: '/new', icon_state: 'ready', icon_source: 'hdicons' },
  ])
  assert.equal(next?.items[0]?.name, 'NAS')
  assert.equal(next?.items[0]?.icon, '/new')
  assert.equal(next?.items[0]?.icon_state, 'ready')
})
