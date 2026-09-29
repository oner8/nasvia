import assert from 'node:assert/strict'
import test from 'node:test'

import { clockLabel, clockParts } from './clock-digits.ts'

test('clockParts：个位数补零', () => {
  const date = new Date(2026, 8, 26, 9, 5, 3)
  assert.deepEqual(clockParts(date), { hours: '09', minutes: '05', seconds: '03' })
})

test('clockParts：边界 00:00:00 与 23:59:59', () => {
  assert.deepEqual(clockParts(new Date(2026, 0, 1, 0, 0, 0)), {
    hours: '00',
    minutes: '00',
    seconds: '00',
  })
  assert.deepEqual(clockParts(new Date(2026, 0, 1, 23, 59, 59)), {
    hours: '23',
    minutes: '59',
    seconds: '59',
  })
})

test('clockParts：非法日期回退为 00:00:00', () => {
  assert.deepEqual(clockParts(new Date(Number.NaN)), {
    hours: '00',
    minutes: '00',
    seconds: '00',
  })
  assert.deepEqual(clockParts(undefined as unknown as Date), {
    hours: '00',
    minutes: '00',
    seconds: '00',
  })
})

test('clockLabel：读屏文本不带前导零', () => {
  assert.equal(clockLabel(new Date(2026, 8, 26, 16, 57, 5)), '16点57分5秒')
  assert.equal(clockLabel(new Date(2026, 8, 26, 0, 0, 0)), '0点0分0秒')
})
