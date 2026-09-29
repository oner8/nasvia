import test from 'node:test'
import assert from 'node:assert/strict'

import { moveActive, splitSuggestion } from './search-suggest.ts'

test('splitSuggestion：以关键词开头时拆出补全部分（不区分大小写，保留原大小写）', () => {
  assert.deepEqual(splitSuggestion('群晖 docker', '群晖'), ['群晖', ' docker'])
  assert.deepEqual(splitSuggestion('Docker compose', 'dock'), ['Dock', 'er compose'])
  assert.deepEqual(splitSuggestion('群晖', '  群晖 '), ['群晖', ''])
})

test('splitSuggestion：不以关键词开头时整条作为补全部分', () => {
  assert.deepEqual(splitSuggestion('synology nas', '群晖'), ['', 'synology nas'])
  assert.deepEqual(splitSuggestion('nas', ''), ['', 'nas'])
})

test('moveActive：上下循环，-1 代表输入框本身', () => {
  assert.equal(moveActive(-1, 1, 3), 0)
  assert.equal(moveActive(2, 1, 3), -1)
  assert.equal(moveActive(-1, -1, 3), 2)
  assert.equal(moveActive(0, -1, 3), -1)
  assert.equal(moveActive(-1, 1, 0), -1)
})
