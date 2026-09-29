import test from 'node:test'
import assert from 'node:assert/strict'

import {
  DEFAULT_LAN_RULES,
  isLoopbackHost,
  isPrivateHost,
  matchesRule,
  networkMode,
  normalizeHost,
  normalizeRules,
  pickByMode,
  pickSiteUrl,
  resolveNetwork,
} from './network.ts'

test('normalizeRules 支持逗号/分号/换行/空格分隔', () => {
  assert.deepEqual(normalizeRules('192.168.1.0/24, 10.0.\n172.16.0.0/12;nas.lan'), [
    '192.168.1.0/24',
    '10.0.',
    '172.16.0.0/12',
    'nas.lan',
  ])
  assert.deepEqual(normalizeRules([]), [])
  assert.deepEqual(normalizeRules(null), [])
})

test('normalizeHost 去掉端口与方括号', () => {
  assert.equal(normalizeHost('192.168.1.10:8096'), '192.168.1.10')
  assert.equal(normalizeHost('NAS.lan:3720'), 'nas.lan')
  assert.equal(normalizeHost('[fd00::1]'), 'fd00::1')
})

test('CIDR 规则命中内网网段', () => {
  assert.equal(isPrivateHost('192.168.1.10', ['192.168.1.0/24']), true)
  assert.equal(isPrivateHost('192.168.1.254', ['192.168.1.0/24']), true)
  assert.equal(isPrivateHost('192.168.2.10', ['192.168.1.0/24']), false)
  assert.equal(isPrivateHost('nas.lan', ['192.168.1.0/24']), false)
})

test('IP 前缀规则命中', () => {
  assert.equal(isPrivateHost('10.0.5.7', ['10.0.']), true)
  assert.equal(isPrivateHost('10.1.5.7', ['10.0.']), false)
  assert.equal(matchesRule('172.16.3.4', '172.16.'), true)
})

test('精确 IP 规则命中', () => {
  assert.equal(isPrivateHost('192.168.1.10', ['192.168.1.10']), true)
  assert.equal(isPrivateHost('192.168.1.11', ['192.168.1.10']), false)
})

test('多条规则组合命中任意一条即为内网', () => {
  const rules = '192.168.1.0/24,10.0.,nas.lan'
  assert.equal(isPrivateHost('192.168.1.5', rules), true)
  assert.equal(isPrivateHost('10.0.9.9', rules), true)
  assert.equal(isPrivateHost('nas.lan', rules), true)
  assert.equal(isPrivateHost('media.nas.lan', rules), true)
  assert.equal(isPrivateHost('example.com', rules), false)
})

test('主机名后缀与通配规则', () => {
  assert.equal(isPrivateHost('a.home.arpa', ['.home.arpa']), true)
  assert.equal(isPrivateHost('home.arpa', ['.home.arpa']), false)
  assert.equal(isPrivateHost('home.arpa', ['*.home.arpa']), true)
  assert.equal(isPrivateHost('b.home.arpa', ['*.home.arpa']), true)
})

test('IPv6 前缀规则', () => {
  assert.equal(isPrivateHost('fd00::1', ['fd00:']), true)
  assert.equal(isPrivateHost('2001:db8::1', ['fd00:']), false)
})

test('pickSiteUrl 命中内网时使用内网地址', () => {
  const site = { url: 'https://jellyfin.example.com', lan_url: 'http://192.168.1.10:8096' }
  assert.equal(pickSiteUrl(site, '192.168.1.10', '192.168.1.0/24'), 'http://192.168.1.10:8096')
  assert.equal(pickSiteUrl(site, 'jellyfin.example.com', '192.168.1.0/24'), 'https://jellyfin.example.com')
})

test('pickSiteUrl 缺少内网地址时回落外网，反之亦然', () => {
  const onlyWan = { url: 'https://one.one.one.one', lan_url: '' }
  assert.equal(pickSiteUrl(onlyWan, '192.168.1.10', '192.168.1.0/24'), 'https://one.one.one.one')

  const onlyLan = { url: '', lan_url: 'http://192.168.1.10:5001' }
  assert.equal(pickSiteUrl(onlyLan, 'nas.example.com', '192.168.1.0/24'), 'http://192.168.1.10:5001')
})

test('未配置内网网段时始终使用外网地址', () => {
  const site = { url: 'https://a.example.com', lan_url: 'http://192.168.1.10' }
  assert.equal(pickSiteUrl(site, '192.168.1.10', []), 'https://a.example.com')
  assert.equal(networkMode('192.168.1.10', []), 'wan')
  assert.equal(networkMode('192.168.1.10', '192.168.1.0/24'), 'lan')
})

test('resolveNetwork：地址栏命中规则优先', () => {
  assert.deepEqual(resolveNetwork('192.168.1.10', ['192.168.1.0/24'], { mode: 'wan', reason: '' }), {
    mode: 'lan',
    reason: 'host',
  })
})

test('resolveNetwork：经反代访问同一域名时采用服务端按访客 IP 的判定', () => {
  const rules = ['192.168.1.0/24']
  assert.deepEqual(resolveNetwork('nav.example.com', rules, { mode: 'lan', reason: 'home_egress' }), {
    mode: 'lan',
    reason: 'home_egress',
  })
  assert.deepEqual(resolveNetwork('nav.example.com', rules, { mode: 'wan', reason: '' }), { mode: 'wan', reason: null })
  // 旧后端没有 network 字段：回落到只按地址栏判定
  assert.deepEqual(resolveNetwork('nav.example.com', rules, undefined), { mode: 'wan', reason: null })
})

test('pickByMode 内网优先内网地址，缺一个时回落另一个', () => {
  assert.equal(pickByMode({ url: 'https://a.example.com', lan_url: 'http://192.168.1.2:8096' }, 'lan'), 'http://192.168.1.2:8096')
  assert.equal(pickByMode({ url: 'https://a.example.com', lan_url: 'http://192.168.1.2:8096' }, 'wan'), 'https://a.example.com')
  assert.equal(pickByMode({ url: '', lan_url: 'http://192.168.1.2:8096' }, 'wan'), 'http://192.168.1.2:8096')
  assert.equal(pickByMode({ url: 'https://a.example.com', lan_url: '' }, 'lan'), 'https://a.example.com')
})

test('本机回环地址不依赖任何规则，固定判为内网', () => {
  for (const host of ['localhost', 'localhost:3720', 'app.localhost', '127.0.0.1', '127.8.8.8:3720', '[::1]']) {
    assert.equal(isLoopbackHost(host), true, host)
    assert.equal(isPrivateHost(host, []), true, host)
  }
  for (const host of ['nav.example.com', '192.168.1.2', 'localhost.example.com']) {
    assert.equal(isLoopbackHost(host), false, host)
  }
})

test('常用内网规则覆盖家用网段，但不含 Docker 网桥的 172.16/12', () => {
  for (const host of ['192.168.1.10', '10.0.0.2', 'nas.lan', 'nas.local', 'home.arpa']) {
    assert.equal(isPrivateHost(host, DEFAULT_LAN_RULES), true, host)
  }
  for (const host of ['172.17.0.1', 'nav.example.com', '8.8.8.8']) {
    assert.equal(isPrivateHost(host, DEFAULT_LAN_RULES), false, host)
  }
})
